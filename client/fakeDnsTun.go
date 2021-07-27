package client

import (
	"fmt"
	"github.com/dosgo/xsocks/client/tun"
	"github.com/dosgo/xsocks/client/tun2socks"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/comm/dot"
	"github.com/dosgo/xsocks/comm/socks"
	"github.com/dosgo/xsocks/comm/winDivert"
	"github.com/dosgo/xsocks/param"
	"github.com/miekg/dns"
	"github.com/vishalkuo/bimap"
	"golang.org/x/sync/singleflight"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"io"
	"log"
	"golang.org/x/time/rate"
	"net"
	"net/url"
	"sync"
	"runtime"
	"strconv"
	"strings"
	"errors"
	"time"
)

var fakeDnsCache *comm.DnsCache


func init(){
	fakeDnsCache = &comm.DnsCache{Cache: make(map[string]string, 128)}
}

type SafeDns interface {
	Resolve(remoteHost string) (string,error)
}

type FakeDnsTun struct {
	tunType int;// 3 or 5
	localSocks string;
	udpLimit sync.Map;
	run bool;
	tunDns *TunDns
	tunDev io.ReadWriteCloser
	safeDns SafeDns
}

type TunDns struct {
	dnsClient *dns.Client
	dnsClientConn *dns.Conn
	udpServer  *dns.Server
	tcpServer  *dns.Server
	serverHost string
	dnsAddr string;
	dnsAddrV6 string;
	dnsPort string;
	ip2Domain *bimap.BiMap
	singleflight   *singleflight.Group
}


var tunAddr ="10.0.0.2";
var tunGW ="10.0.0.1";
var tunMask="255.255.0.0"
var fakeUdpNat sync.Map


func (fakeDns *FakeDnsTun)Start(tunType int,tunDevice string,_tunAddr string,_tunMask string,_tunGW string,tunDNS string) {
	fakeDns.tunType=tunType;
	//start local dns
	fakeDns.tunDns =&TunDns{};
	fakeDns.tunDns.dnsPort="53";
	fakeDns.tunDns.dnsAddr="127.0.0.1"
	fakeDns.tunDns.dnsAddrV6="0:0:0:0:0:0:0:1"
	if fakeDns.tunType==3 {
		//remote dns
		fakeDns.safeDns = &RemoteDns{}
		fakeDns.localSocks=param.Args.Sock5Addr;
	}
	if fakeDns.tunType==5 {
		fakeDns.localSocks=	param.Args.ServerAddr[9:];
		_safeDns:=&dot.DoT{};
		_safeDns.ServerName="dns.google";
		_safeDns.Addr="8.8.8.8:853";
		_safeDns.LSocks=fakeDns.localSocks;
		fakeDns.safeDns = _safeDns
	}


	fakeDns.tunDns.ip2Domain= bimap.NewBiMap()
	fakeDns.tunDns.singleflight  = &singleflight.Group{}
	//fmt.Printf("dnsServers:%v\r\n",dnsServers)
	urlInfo, _ := url.Parse(param.Args.ServerAddr)
	fakeDns.tunDns.serverHost=urlInfo.Hostname()

	//生成本地udp端口避免过滤的时候变动了
	clientPort,_:=comm.GetFreeUdpPort();
	fakeDns.tunDns._startSmartDns(clientPort)

	//edit DNS
	if runtime.GOOS!="windows" {
		comm.SetNetConf(fakeDns.tunDns.dnsAddr, fakeDns.tunDns.dnsAddrV6);
	}
	fakeDns._startTun(tunDevice,_tunAddr,_tunMask,_tunGW,tunDNS);

	if runtime.GOOS=="windows" {
		go winDivert.RedirectDNS(fakeDns.tunDns.dnsAddr,fakeDns.tunDns.dnsPort,clientPort);
	}
	//udp limit auto remove
	fakeDns.run=true;
	go fakeDns.autoFree();
}

func (fakeDns *FakeDnsTun)Shutdown(){
	if fakeDns.tunDev!=nil {
		fakeDns.tunDev.Close();
	}
	if fakeDns.tunDns!=nil {
		comm.ResetNetConf(fakeDns.tunDns.dnsAddr);
		fakeDns.tunDns.Shutdown();
	}
	fakeDns.run=false;
	winDivert.CloseWinDivert();
}



func (fakeDns *FakeDnsTun) _startTun(tunDevice string,_tunAddr string,_tunMask string,_tunGW string,tunDNS string) (error){
	if len(_tunAddr)>0 {
		tunAddr =_tunAddr;
	}
	if len(_tunMask)>0 {
		tunMask = _tunMask;
	}
	if len(_tunGW)>0 {
		tunGW=_tunGW
	}
	var err error
	//dnsServers := strings.Split(tunDNS, ",")
	if len(param.Args.UnixSockTun)>0 {
		fakeDns.tunDev,err=tun.UsocketToTun(param.Args.UnixSockTun)
		if err!=nil {
			return err;
		}
	}else{
		fmt.Printf("tunGW:%s tunMask:%s\r\n",tunGW,tunMask)
		fakeDns.tunDev, err = tun.RegTunDev(tunDevice,tunAddr,tunMask,tunGW,tunDNS)
		if err != nil {
			return err;
		}
	}
	go func() {
		time.Sleep(time.Second*1)
		comm.AddRoute(tunAddr, tunGW,tunMask)
	}()
	go tun2socks.ForwardTransportFromIo(fakeDns.tunDev,param.Args.Mtu,fakeDns.tcpForwarder,fakeDns.udpForwarder);
	return nil;
}
func (fakeDns *FakeDnsTun) autoFree(){
	for fakeDns.run{
		fakeDns.udpLimit.Range(func(k, v interface{}) bool {
			_v:=v.(*comm.UdpLimit);
			if _v.Expired<time.Now().Unix() {
				fakeDns.udpLimit.Delete(k)
			}
			return true
		})
		time.Sleep(time.Second*30);
	}
}

func (fakeDns *FakeDnsTun) tcpForwarder(conn *gonet.TCPConn)error{
	var srcAddr=conn.LocalAddr().String();
	var remoteAddr="";
	var addrType uint8;
	addrType =0x01;
	remoteAddr = fakeDns.dnsToAddr(srcAddr)
	if remoteAddr==""{
		log.Printf("remoteAddr:%v\r\n", remoteAddr)
		conn.Close();
		return nil;
	}
	log.Printf("remoteAddr:%v\r\n", remoteAddr)
	socksConn,err1:= net.DialTimeout("tcp",fakeDns.localSocks,time.Second*15)
	if err1 != nil {
		log.Printf("err:%v",err1)
		return nil
	}
	defer socksConn.Close();
	if socks.SocksCmd(socksConn,1,addrType,remoteAddr)==nil {
		comm.TcpPipe(conn,socksConn,time.Minute*5)
	}
	return nil
}

func (fakeDns *FakeDnsTun) udpForwarder(conn *gonet.UDPConn, ep tcpip.Endpoint)error{
	var srcAddr=conn.LocalAddr().String();
    var remoteAddr=fakeDns.dnsToAddr(srcAddr)
	if remoteAddr==""{
		conn.Close();
		return nil;
	}
	if fakeDns.tunType==3 {
		defer ep.Close();
		dstAddr,_:=net.ResolveUDPAddr("udp",remoteAddr)
		fmt.Printf("udp-remoteAddr:%s\r\n",remoteAddr)
		socks.SocksUdpGate(conn,dstAddr);
	}
	//tuntype 直连
	if fakeDns.tunType==5 {
		var limit *comm.UdpLimit;
		_limit,ok:=fakeDns.udpLimit.Load(remoteAddr)
		if !ok{
			limit=&comm.UdpLimit{Limit: rate.NewLimiter(rate.Every(1 * time.Second), 50),Expired: time.Now().Unix()+5}
		}else{
			limit=_limit.(*comm.UdpLimit);
		}
		//限流
		if limit.Limit.Allow(){
			limit.Expired=time.Now().Unix()+5;
			//本地直连交换
			comm.TunNatSawp(&fakeUdpNat, conn,ep, remoteAddr, 65*time.Second);
			fakeDns.udpLimit.Store(remoteAddr,limit);
		}
	}
	return nil;
}


/*dns addr swap*/
func (fakeDns *FakeDnsTun) dnsToAddr(remoteAddr string) string{
	remoteAddr=fakeDns.dnsToDomain(remoteAddr)
	if remoteAddr=="" {
		return "";
	}
	remoteAddrs:=strings.Split(remoteAddr,":")
	domain:=remoteAddrs[0]
	ip, err := fakeDns.safeDns.Resolve(domain[0 : len(domain)-1])
	if err!=nil{
		return "";
	}
	return ip+":"+remoteAddrs[1]
}

/*dns addr swap*/
func (fakeDns *FakeDnsTun) dnsToDomain(remoteAddr string) string{
	if fakeDns.tunDns==nil {
		return "";
	}
	remoteAddrs:=strings.Split(remoteAddr,":")
	_domain,ok:= fakeDns.tunDns.ip2Domain.Get(remoteAddrs[0])
	if !ok{
		return "";
	}
	return _domain.(string)+":"+remoteAddrs[1]
}



func (tunDns *TunDns)_startSmartDns(clientPort string)  {
	tunDns.udpServer = &dns.Server{
		Net:          "udp",
		Addr:         ":"+tunDns.dnsPort,
		Handler:      dns.HandlerFunc(tunDns.ServeDNS),
		UDPSize:      4096,
		ReadTimeout:  time.Duration(10) * time.Second,
		WriteTimeout: time.Duration(10) * time.Second,
	}
	tunDns.tcpServer= &dns.Server{
		Net:          "tcp",
		Addr:         ":"+tunDns.dnsPort,
		Handler:      dns.HandlerFunc(tunDns.ServeDNS),
		UDPSize:      4096,
		ReadTimeout:  time.Duration(10) * time.Second,
		WriteTimeout: time.Duration(10) * time.Second,
	}

	localPort, _ := strconv.Atoi(clientPort)
	netAddr := &net.UDPAddr{Port:localPort}
	_Dialer := &net.Dialer{Timeout:3*time.Second,LocalAddr: netAddr}
	tunDns.dnsClient = &dns.Client{
		Net:          "udp",
		UDPSize:      4096,
		Dialer: _Dialer,
		SingleInflight:true,
		ReadTimeout:  time.Duration(3) * time.Second,
		WriteTimeout: time.Duration(2) * time.Second,
	}
	tunDns.dnsClientConn,_=tunDns.dnsClient.Dial( comm.GetOldDns(tunDns.dnsAddr,tunGW,"")+":53");
	go tunDns.udpServer.ListenAndServe();
	go tunDns.tcpServer.ListenAndServe();
}

func (tunDns *TunDns)Shutdown(){
	if tunDns.tcpServer!=nil {
		tunDns.tcpServer.Shutdown();
	}
	if tunDns.udpServer!=nil {
		tunDns.udpServer.Shutdown();
	}
}



func (tunDns *TunDns) doIPv4Query(r *dns.Msg) (*dns.Msg, error) {
	m := &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = false
	domain := r.Question[0].Name
	v, _, _ := tunDns.singleflight.Do(domain, func() (interface{}, error) {
		return tunDns.ipv4Res(domain,r);
	})
	m.Answer =v.( []dns.RR )
	// final
	return m, nil
}

func  (tunDns *TunDns)ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	var msg *dns.Msg
	var err error
	switch r.Question[0].Qtype {
	case  dns.TypeA:
		msg, err = tunDns.doIPv4Query(r)
		break;
	case  dns.TypeAAAA:
		//ipv6
		msg, err = tunDns.resolve(r)
		break;
	default:
		msg,_,err = tunDns.dnsClient.ExchangeWithConn(r,tunDns.dnsClientConn)
		break;
	}
	if err != nil {
		dns.HandleFailed(w, r)
	} else {
		w.WriteMsg(msg)
	}
}





/*ipv4智能响应*/
func (tunDns *TunDns)ipv4Res(domain string,r *dns.Msg) ([]dns.RR,error)  {
	var ip ="";
	var _ip  net.IP
	var ipTtl uint32=60;
	var dnsErr=false;
	ipLog,ok :=tunDns.ip2Domain.GetInverse(domain)
	if ok && strings.Index(domain, tunDns.serverHost) == -1 && strings.HasPrefix(ipLog.(string), tunAddr[0:4]) {
		ip=ipLog.(string);
		ipTtl=1;
	}else {
		if _ip==nil && r!=nil  {
			//为空的话智能dns的话先解析一遍
			m1,_,err := tunDns.localResolve(r)
			if err == nil {
				_ip=m1;
			}else{
				fmt.Printf("local dns error:%v\r\n",err)
				oldDns:=comm.GetOldDns(tunDns.dnsAddr,tunGW,"");
				//检测网关DNS是否改变
				if strings.Index(tunDns.dnsClientConn.RemoteAddr().String(),oldDns)==-1 {
					tunDns.dnsClientConn.Close();
					dnsClientConn,err:=tunDns.dnsClient.Dial(oldDns+":53");
					if err==nil {
						tunDns.dnsClientConn=dnsClientConn;
					}
				}
				//解析错误说明无网络,否则就算不存在也会回复的
				dnsErr=true;//标记为错误
			 }
		}

		//不为空判断是不是中国ip
		if   strings.Index(domain, tunDns.serverHost) != -1|| (_ip!=nil && (comm.IsChinaMainlandIP(_ip.String()) || !comm.IsPublicIP(_ip))) {
			//中国Ip直接回复
			if _ip!=nil {
				ip = _ip.String();
			}
		} else if strings.Index(domain, tunDns.serverHost) == -1 &&!dnsErr {
			//外国随机分配一个代理ip
			for i := 0; i <= 2; i++ {
				ip = comm.GetCidrRandIpByNet(tunAddr, tunMask)
				_, ok := tunDns.ip2Domain.Get(ip)
				if !ok && ip!=tunAddr {
					ipTtl=1;
					tunDns.ip2Domain.Insert(ip, domain)
					break;
				} else {
					fmt.Println("ip used up")
					ip = "";
				}
			}
		}
	}
	fmt.Printf("domain:%s ip:%s\r\n",domain,ip)
	return []dns.RR{&dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ipTtl},
		A:   net.ParseIP(ip),
	}},nil;
}

func  (tunDns *TunDns)resolve(r *dns.Msg) (*dns.Msg, error) {
	m :=  &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = false
	domain := r.Question[0].Name
	//ipv6
	m1,_,err :=	tunDns.dnsClient.ExchangeWithConn(r,tunDns.dnsClientConn)
	fmt.Printf("ipv6:%s err:%+v\r\n",domain,err)
	if err == nil {
		return m1,nil;
	}
	return m, err;
}

/*本地dns解析有缓存*/
func  (tunDns *TunDns)localResolve(r *dns.Msg) (net.IP,uint32, error) {
	domain := r.Question[0].Name
	cache,ttl:= fakeDnsCache.ReadDnsCache(domain)
	if cache!="" {
		return net.ParseIP(cache), ttl,nil;
	}

	m1,_,err := tunDns.dnsClient.ExchangeWithConn(r,tunDns.dnsClientConn)
	if err == nil {
		for _, v := range m1.Answer {
			record, isType := v.(*dns.A)
			if isType {
				//有些dns会返回127.0.0.1
				if record.A.String() != "127.0.0.1" {
					fakeDnsCache.WriteDnsCache(domain,record.Hdr.Ttl,record.A.String())
					return  record.A, record.Hdr.Ttl,nil;
				}
			}
		}
	}
	return nil,0,errors.New("error")
}
