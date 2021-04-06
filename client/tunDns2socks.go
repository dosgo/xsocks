package client

import (
	"fmt"
	"github.com/miekg/dns"
	"github.com/songgao/water"
	"github.com/vishalkuo/bimap"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"github.com/dosgo/xsocks/client/tun2socks"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
)



type TunDns struct {
	remoteDns RemoteDns
	dnsClient *dns.Client
	oldDns string
	serverHost string
}
var tunDns TunDns;
var ip2Domain = bimap.NewBiMap()

var tunAddr="10.0.0.2"
var tunGW="10.0.0.1";
var tunMask="255.255.0.0"

//var tunMask="255.255.255.0"

func StartTunDns(tunDevice string,_tunAddr string,_tunMask string,_tunGW string,tunDNS string) {
	gwIp:=comm.GetGateway()
	oldDns,_,_:=comm.GetDnsServerByGateWay(gwIp);
	if oldDns[0]=="127.0.0.1"||oldDns[0]==tunGW || oldDns[0]==_tunGW  {
		oldDns[0]="114.114.114.114"
	}
	fmt.Printf("oldDns:%v\r\n",oldDns)
	urlInfo, _ := url.Parse(param.Args.ServerAddr)
	tunDns.serverHost=urlInfo.Hostname()
	_startSmartDns("53",oldDns[0])
	go comm.WatchNotifyIpChange();
	//fmt.Printf("_tunAddr:%s _tunGW:%s\r\n",_tunAddr,_tunGW)
	_startTun(tunDevice,_tunAddr,_tunMask,_tunGW,tunDNS);
}



func _startTun(tunDevice string,_tunAddr string,_tunMask string,_tunGW string,tunDNS string){
	if len(tunDevice)==0 {
		tunDevice="tun0";
	}
	if len(_tunAddr)>0 {
		tunAddr =_tunAddr;
	}
	if len(_tunMask)>0 {
		tunMask = _tunMask;
	}
	if len(_tunGW)>0 {
		tunGW=_tunGW
	}
	if len(tunDNS)==0 {
		tunDNS="114.114.114.114";
	}

	//dnsServers := strings.Split(tunDNS, ",")
	var dev io.ReadWriteCloser;
	if len(param.Args.UnixSockTun)>0 {
		os.Remove(param.Args.UnixSockTun)
		addr, err := net.ResolveUnixAddr("unixpacket", param.Args.UnixSockTun)
		if err != nil {
			return ;
		}
		lis, err := net.ListenUnix("unixpacket", addr)
		if err != nil {                      //如果监听失败，一般是文件已存在，需要删除它
			log.Println("UNIX Domain Socket 创 建失败，正在尝试重新创建 -> ", err)
			os.Remove(param.Args.UnixSockTun)
			return ;
		}
		defer lis.Close() //虽然本次操作不会执行， 不过还是加上比较好
		conn, err := lis.Accept() //开始接 受数据
		if err != nil {                      //如果监听失败，一般是文件已存在，需要删除它
			return ;
		}
		dev=conn;
		defer conn.Close()
	}else{
		fmt.Printf("tunGW:%s tunMask:%s\r\n",tunGW,tunMask)
		/*
		f, err:= tun.OpenTunDevice(tunDevice, tunAddr, tunGW, tunMask, dnsServers)
		if err != nil {
			fmt.Println("Error listening:", err)
			return ;
		}
		*/

		config := comm.GetWaterConf(tunAddr,tunMask);
		ifce, err := water.New(config)
		if err != nil {
			fmt.Println("start tun err:", err)
			return ;
		}
		if runtime.GOOS=="windows" {
			//time.Sleep(time.Second*1)
			//netsh interface ip set address name="Ehternet 2" source=static addr=10.1.0.10 mask=255.255.255.0 gateway=none
			exec.Command("netsh", "interface","ip","set","address","name="+ifce.Name(),"source=static","addr="+tunAddr,"mask="+tunMask,"gateway=none").Run();
		}else if runtime.GOOS=="linux"{
			//sudo ip addr add 10.1.0.10/24 dev O_O
			masks:=net.ParseIP(tunMask).To4();
			maskAddr:=net.IPNet{IP: net.ParseIP(tunAddr), Mask: net.IPv4Mask(masks[0], masks[1], masks[2], masks[3] )}
			exec.Command("ip", "addr","add",maskAddr.String(),"dev",ifce.Name()).Run();
			exec.Command("ip", "link","set","dev",ifce.Name(),"up").Run();
		}else if runtime.GOOS=="darwin"{
			//ifconfig utun2 10.1.0.10 10.1.0.20 up
			masks:=net.ParseIP(tunMask).To4();
			maskAddr:=net.IPNet{IP: net.ParseIP(tunAddr), Mask: net.IPv4Mask(masks[0], masks[1], masks[2], masks[3] )}
			ipMin,ipMax:=comm.GetCidrIpRange(maskAddr.String());
			exec.Command("ifconfig", "utun2",ipMin,ipMax,"up").Run();
		}
		dev=ifce;
	}
	go func() {
		time.Sleep(time.Second*1)
		comm.AddRoute(tunAddr, tunGW,tunMask)
	}()
	tun2socks.ForwardTransportFromIo(dev,param.Args.Mtu,dnsTcpForwarder,dnsUdpForwarder);
}


func dnsTcpForwarder(conn *gonet.TCPConn)error{

	//local dns
	if conn.LocalAddr().String()==(tunGW+":53"){
		log.Printf("local dns\r\n")
		conn2, err := net.DialTimeout("tcp","127.0.0.1:53",time.Second*15);
		if err != nil {
			return err;
		}
		comm.TcpPipe(conn,conn2,time.Second*30)
		return nil;
	}

	remoteAddr:=dnsToAddr(conn.LocalAddr().String())
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

func dnsUdpForwarder(conn *gonet.UDPConn, ep tcpip.Endpoint)error{
	//log.Printf("udpAddr:%s\r\n",conn.LocalAddr().String())
	defer ep.Close();
	defer conn.Close();

	//local dns
	if conn.LocalAddr().String()==(tunGW+":53"){
		log.Printf("local dns\r\n")
		conn2, err := net.DialTimeout("udp","127.0.0.1:53",time.Second*15);
		if err != nil {
			log.Printf("local dns2\r\n")
			return err;
		}
		comm.UdpPipe(conn,conn2,time.Second*30)
		return nil;
	}


	remoteAddr:=dnsToAddr(conn.LocalAddr().String())
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
func dnsToAddr(remoteAddr string) string{
	remoteAddrs:=strings.Split(remoteAddr,":")
	_domain,ok:= ip2Domain.Get(remoteAddrs[0])
	if !ok{
		return "";
	}
	domain:=_domain.(string)
	ip, err := tunDns.remoteDns.Resolve(domain[0 : len(domain)-1])
	if err!=nil{
		return "";
	}
	return ip+":"+remoteAddrs[1]
}




func _startSmartDns(dnsPort string,oldDns string) error {
	udpServer := &dns.Server{
		Net:          "udp",
		Addr:         ":"+dnsPort,
		Handler:      dns.HandlerFunc(tunDns.ServeDNS),
		UDPSize:      4096,
		ReadTimeout:  time.Duration(10) * time.Second,
		WriteTimeout: time.Duration(10) * time.Second,
	}
	tcpServer:= &dns.Server{
		Net:          "tcp",
		Addr:         ":"+dnsPort,
		Handler:      dns.HandlerFunc(tunDns.ServeDNS),
		UDPSize:      4096,
		ReadTimeout:  time.Duration(10) * time.Second,
		WriteTimeout: time.Duration(10) * time.Second,
	}
	tunDns.remoteDns = RemoteDns{}
	tunDns.oldDns=oldDns
	tunDns.dnsClient = &dns.Client{
		Net:          "udp",
		UDPSize:      4096,
		ReadTimeout:  time.Duration(1) * time.Second,
		WriteTimeout: time.Duration(1) * time.Second,
	}
	go udpServer.ListenAndServe();
	go tcpServer.ListenAndServe();
	return nil;
}





func (tunDns *TunDns) doIPv4Query(r *dns.Msg) (*dns.Msg, error) {
	m := &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = false
	domain := r.Question[0].Name
	m.Answer =ipv4Res(domain,nil,r);
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
func ipv4Res(domain string,_ip  net.IP,r *dns.Msg) []dns.RR {
	var ip ="";
	ipLog,ok :=ip2Domain.GetInverse(domain)
	if ok && strings.Index(domain, tunDns.serverHost) == -1{
		ip=ipLog.(string);
	}else {
		if _ip==nil && r!=nil  {
			//为空的话智能dns的话先解析一遍
			if param.Args.SmartDns==1  {
				m1,_,err := localdns.dnsClient.Exchange(r,tunDns.oldDns+":53")
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
				_, ok := ip2Domain.Get(ip)
				if !ok && ip!=tunAddr {
					ip2Domain.Insert(ip, domain)
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


	//先ipv4
	/*
	var ipv4Addr net.IP;
	m2 :=  &dns.Msg{}
	m2.SetQuestion(domain, dns.TypeA)
	m2.Authoritative = false
	r1, _, err := localdns.dnsClient.Exchange(m2,"114.114.114.114:53")
	if err == nil {
		for _, v := range r1.Answer {
			record, _isType := v.(*dns.A)
			if _isType {
				ipv4Addr=record.A;
				break;
			}
		}
	}
*/
	fmt.Printf("ipv6:%s\r\n",domain)
	//ipv6
	m1,_,err := localdns.dnsClient.Exchange(r,"114.114.114.114:53")
	if err == nil {
		return m1,nil;
	}


	/*
		m.Answer = append(r.Answer, &dns.AAAA{
			Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
			AAAA:   net.ParseIP("fd3e:4f5a:5b81::1"),
		})*/
	return m, nil
}