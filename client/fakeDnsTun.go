package client

import (
	"fmt"
	"github.com/dosgo/xsocks/client/tun"
	"github.com/dosgo/xsocks/client/tun2socks"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
	"github.com/miekg/dns"
	"github.com/vishalkuo/bimap"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"time"
)

type FakeDnsTun struct {
	tunDns *TunDns
	tunDev io.ReadWriteCloser
	remoteDns *RemoteDns
}

type TunDns struct {
	dnsClient *dns.Client
	oldDns string
	udpServer  *dns.Server
	tcpServer  *dns.Server
	serverHost string
	dnsAddr string;
	dnsAddrV6 string;
	dnsPort string;
	ip2Domain *bimap.BiMap
}


var tunAddr ="10.0.0.2";
var tunGW ="10.0.0.1";
var tunMask="255.255.0.0"


func (fakeDns *FakeDnsTun)Start(tunDevice string,_tunAddr string,_tunMask string,_tunGW string,tunDNS string) {
	//remote dns
	fakeDns.remoteDns = &RemoteDns{}
	//start local dns
	fakeDns.tunDns =&TunDns{};
	fakeDns.tunDns.dnsPort="53";
	fakeDns.tunDns.dnsAddr="127.0.0.1"
	fakeDns.tunDns.dnsAddrV6="0:0:0:0:0:0:0:1"
	fakeDns.tunDns.ip2Domain= bimap.NewBiMap()
	gwIp:=comm.GetGateway()
	oldDns,_,_:=comm.GetDnsServerByGateWay(gwIp);
	if oldDns[0]==fakeDns.tunDns.dnsAddr||oldDns[0]==tunGW || oldDns[0]==_tunGW  {
		oldDns[0]="114.114.114.114"
	}
	fmt.Printf("oldDns:%v\r\n",oldDns)
	urlInfo, _ := url.Parse(param.Args.ServerAddr)
	fakeDns.tunDns.serverHost=urlInfo.Hostname()
	fakeDns.tunDns._startSmartDns(oldDns[0])

	//edit DNS
	comm.SetDNSServer(fakeDns.tunDns.dnsAddr,fakeDns.tunDns.dnsAddrV6,gwIp);
	fakeDns._startTun(tunDevice,_tunAddr,_tunMask,_tunGW,tunDNS);
}

func (fakeDns *FakeDnsTun)Shutdown(){
	if fakeDns.tunDev!=nil {
		fakeDns.tunDev.Close();
	}
	if fakeDns.tunDns!=nil {
		comm.ResetDns(fakeDns.tunDns.dnsAddr);
		fakeDns.tunDns.Shutdown();
	}
}



func (fakeDns *FakeDnsTun) _startTun(tunDevice string,_tunAddr string,_tunMask string,_tunGW string,tunDNS string)error{
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
	go tun2socks.ForwardTransportFromIo(fakeDns.tunDev,param.Args.Mtu,fakeDns.dnsTcpForwarder,fakeDns.dnsUdpForwarder);
	return nil;
}


func (fakeDns *FakeDnsTun) dnsTcpForwarder(conn *gonet.TCPConn)error{

	//local dns
	if conn.LocalAddr().String()==(tunGW+":53") && fakeDns.tunDns!=nil{
		log.Printf("local dns\r\n")
		conn2, err := net.DialTimeout("tcp",fakeDns.tunDns.dnsAddr+":"+fakeDns.tunDns.dnsPort,time.Second*15);
		if err != nil {
			return err;
		}
		comm.TcpPipe(conn,conn2,time.Second*30)
		return nil;
	}

	remoteAddr:=fakeDns.dnsToAddr(conn.LocalAddr().String())
	if remoteAddr==""{
		log.Printf("remoteAddr:%v\r\n",remoteAddr)
		conn.Close();
		return nil;
	}
	log.Printf("remoteAddr:%v\r\n",remoteAddr)
	socksConn,err1:= net.DialTimeout("tcp",param.Args.Sock5Addr,time.Second*15)
	if err1 != nil {
		log.Printf("err:%v",err1)
		return nil
	}
	defer socksConn.Close();
	if tun2socks.SocksCmd(socksConn,1,remoteAddr)==nil {
		comm.TcpPipe(conn,socksConn,time.Minute*5)
	}
	return nil
}

func (fakeDns *FakeDnsTun) dnsUdpForwarder(conn *gonet.UDPConn, ep tcpip.Endpoint)error{
	//log.Printf("udpAddr:%s\r\n",conn.LocalAddr().String())
	defer ep.Close();
	defer conn.Close();

	//local dns
	if conn.LocalAddr().String()==(tunGW+":53") && fakeDns.tunDns!=nil{
		log.Printf("local dns\r\n")
		conn2, err := net.DialTimeout("udp",fakeDns.tunDns.dnsAddr+":"+fakeDns.tunDns.dnsPort,time.Second*15);
		if err != nil {
			log.Printf("local dns2\r\n")
			return err;
		}
		comm.UdpPipe(conn,conn2,time.Second*30)
		return nil;
	}

	remoteAddr:=fakeDns.dnsToAddr(conn.LocalAddr().String())
	if remoteAddr==""{
		conn.Close();
		return nil;
	}
	dstAddr,_:=net.ResolveUDPAddr("udp",remoteAddr)
	fmt.Printf("udp-remoteAddr:%s\r\n",remoteAddr)
	tun2socks.SocksUdpGate(conn,dstAddr);
	return nil;
}
/*dns addr swap*/
func (fakeDns *FakeDnsTun) dnsToAddr(remoteAddr string) string{
	if fakeDns.tunDns==nil {
		return "";
	}
	remoteAddrs:=strings.Split(remoteAddr,":")
	_domain,ok:= fakeDns.tunDns.ip2Domain.Get(remoteAddrs[0])
	if !ok{
		return "";
	}
	domain:=_domain.(string)
	ip, err := fakeDns.remoteDns.Resolve(domain[0 : len(domain)-1])
	if err!=nil{
		return "";
	}
	return ip+":"+remoteAddrs[1]
}




func (tunDns *TunDns)_startSmartDns(oldDns string)  {
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
	tunDns.oldDns=oldDns
	tunDns.dnsClient = &dns.Client{
		Net:          "udp",
		UDPSize:      4096,
		ReadTimeout:  time.Duration(1) * time.Second,
		WriteTimeout: time.Duration(1) * time.Second,
	}
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
	m.Answer =tunDns.ipv4Res(domain,nil,r);
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
		msg,_,err = tunDns.dnsClient.Exchange(r,tunDns.oldDns+":53")
		break;
	}
	if err != nil {
		dns.HandleFailed(w, r)
	} else {
		w.WriteMsg(msg)
	}
}
/*ipv4智能响应*/
func (tunDns *TunDns)ipv4Res(domain string,_ip  net.IP,r *dns.Msg) []dns.RR {
	var ip ="";
	ipLog,ok :=tunDns.ip2Domain.GetInverse(domain)
	if ok && strings.Index(domain, tunDns.serverHost) == -1{
		ip=ipLog.(string);
	}else {
		if _ip==nil && r!=nil  {
			//为空的话智能dns的话先解析一遍
			if param.Args.SmartDns==1  {
				m1,_,err := tunDns.dnsClient.Exchange(r,tunDns.oldDns+":53")
				if err == nil {
					for _, v := range m1.Answer {
						record, isType := v.(*dns.A)
						if isType {
							_ip=record.A;
							break;
						}
					}
				}
			}
		}

		//不为空判断是不是中国ip
		if   strings.Index(domain, tunDns.serverHost) != -1|| (_ip!=nil && (comm.IsChinaMainlandIP(_ip.String()) || !comm.IsPublicIP(_ip))) {
			//中国Ip直接回复
			if _ip!=nil {
				ip = _ip.String();
			}
		} else {
			//外国随机分配一个代理ip
			for i := 0; i <= 2; i++ {
				ip = comm.GetCidrRandIpByNet(tunAddr, tunMask)
				_, ok := tunDns.ip2Domain.Get(ip)
				if !ok && ip!=tunAddr {
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
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   net.ParseIP(ip),
	}}
}

func  (tunDns *TunDns)resolve(r *dns.Msg) (*dns.Msg, error) {
	m :=  &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = false
	domain := r.Question[0].Name
	fmt.Printf("ipv6:%s\r\n",domain)
	//ipv6
	m1,_,err := tunDns.dnsClient.Exchange(r,"114.114.114.114:53")
	if err == nil {
		return m1,nil;
	}
	return m, nil
}