package client

import (
	"github.com/miekg/dns"
	"github.com/yinghuocho/gotun2socks/tun"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"runtime"
	"fmt"
	"time"
	"xSocks/client/tun2socks"
    "github.com/StackExchange/wmi"
	"xSocks/comm"
	"xSocks/param"
)



type TunDns struct {
	remoteDns RemoteDns
}
var tunDns TunDns;
var dnsAddr sync.Map

var tunAddr="10.0.0.2"
var tunMask="255.255.255.0"


func StartTunDns(tunDevice string,_tunAddr string,_tunMask string,tunGW string,tunDNS string) {
	dnsPort,_:= comm.GetFreePort();
	_startSmartDns(dnsPort)
	_startTun(tunDevice,_tunAddr,_tunMask,tunGW,tunDNS);

}



func _startTun(tunDevice string,_tunAddr string,_tunMask string,tunGW string,tunDNS string){
	if len(tunDevice)==0 {
		tunDevice="tun0";
	}
	if len(tunAddr)==0 {
		tunAddr =_tunAddr;
	}
	if len(tunMask)==0 {
		tunMask = _tunMask;
	}
	if len(tunGW)==0 {
		tunGW="10.0.0.1";
	}
	if len(tunDNS)==0 {
		tunDNS="114.114.114.114";
	}


	strings.Split(param.ServerAddr,":");
	dnsServers := strings.Split(tunDNS, ",")
	fmt.Printf("dnsServers:%v\r\n",dnsServers)
	var dev io.ReadWriteCloser;
	var remoteAddr string;
	if len(param.UnixSockTun)>0 {
		os.Remove(param.UnixSockTun)
		addr, err := net.ResolveUnixAddr("unixpacket", param.UnixSockTun)
		if err != nil {
			return ;
		}
		lis, err := net.ListenUnix("unixpacket", addr)
		if err != nil {                      //如果监听失败，一般是文件已存在，需要删除它
			log.Println("UNIX Domain Socket 创 建失败，正在尝试重新创建 -> ", err)
			os.Remove(param.UnixSockTun)
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
		if runtime.GOOS=="windows" {
			urlInfo, _ := url.Parse(param.ServerAddr)
			addr, err := net.ResolveIPAddr("ip",urlInfo.Hostname())
			if err == nil {
				remoteAddr = addr.String()
			}
			fmt.Printf("remoteAddr:%s\r\n", remoteAddr)
		}

		f, err:= tun.OpenTunDevice(tunDevice, tunAddr, tunGW, tunMask, dnsServers)
		if err != nil {
			fmt.Println("Error listening:", err)
			return ;
		}
		dev=f;
	}
	tunDns.remoteDns = RemoteDns{}
	tun2socks.ForwardTransportFromIo(dev,param.Mtu,dnsTcpForwarder,dnsUdpForwarder);
}


func dnsTcpForwarder(conn *gonet.TCPConn)error{
	remoteAddr:=dnsToAddr(conn.LocalAddr().String())
	if remoteAddr==""{
		conn.Close();
		return nil;
	}

	socksConn,err1:= net.DialTimeout("tcp",param.Sock5Addr,time.Second*15)
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
	defer ep.Close();
	defer conn.Close();
	remoteAddr:=dnsToAddr(conn.LocalAddr().String())
	if remoteAddr==""{
		conn.Close();
		return nil;
	}
	dstAddr,_:=net.ResolveUDPAddr("udp",remoteAddr)
	tun2socks.SocksUdpGate(conn,dstAddr);
	return nil;
}
/*dns addr swap*/
func dnsToAddr(remoteAddr string) string{
	remoteAddrs:=strings.Split(remoteAddr,":")
	_domain,ok:= dnsAddr.Load(remoteAddrs[0])
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




func _startSmartDns(dnsPort string) error {
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
	go udpServer.ListenAndServe();
	tcpServer.ListenAndServe();
	return nil;
}





func (tunDns *TunDns) doIPv4Query(r *dns.Msg) (*dns.Msg, error) {
	m := &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = false
	domain := r.Question[0].Name
	var ip string;
	if param.SmartDns==1 {
		ipAddr, err := net.ResolveIPAddr("ip", domain[0 : len(domain)-1])
		if err == nil {
			//中国Ip直接回复
			if !comm.IsPublicIP(ipAddr.IP) || comm.IsChinaMainlandIP(ipAddr.String()) {
				m.Answer = append(r.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
					A:   net.ParseIP(ipAddr.String()),
				})
				return m,nil;
			}
		}
	}

	masks:=net.ParseIP(tunMask)
	maskAddr:=net.IPNet{net.ParseIP(tunAddr),net.IPv4Mask(masks[0], masks[1], masks[2], masks[3] )}
	ip=comm.GetCidrRandIp(maskAddr.String())
	for i := 0; i <= 2; i++ {
		ip=comm.GetCidrRandIp(maskAddr.String())
		_,ok := dnsAddr.Load(ip)
		if !ok {
			dnsAddr.Store(ip,domain)
			break;
		}else{
			ip="";
		}
	}

	m.Answer = append(r.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   net.ParseIP(ip),
	})
	// final
	return m, nil
}
func  (tunDns *TunDns)ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	isIPv4 := isIPv4Query(r.Question[0])
	var msg *dns.Msg
	var err error
	if isIPv4 {
		msg, err = tunDns.doIPv4Query(r)
	} else {
		msg, err = resolve(r)
	}
	if err != nil {
		dns.HandleFailed(w, r)
	} else {
		w.WriteMsg(msg)
	}
}

