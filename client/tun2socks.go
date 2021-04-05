package client

import (
	"fmt"
	"github.com/songgao/water"
	"gvisor.dev/gvisor/pkg/tcpip"
	"net/url"
	"os"
	"time"
	"github.com/dosgo/xSocks/client/tun2socks"
	"github.com/dosgo/xSocks/comm"
	//"github.com/google/netstack/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"io"
	"log"
	"os/exec"
	"net"
	"runtime"
	"strings"
	"github.com/dosgo/xSocks/param"
)



/*tunType==1*/
func StartTunDevice(tunDevice string,tunAddr string,tunMask string,tunGW string,tunDNS string) {
	if len(tunDevice)==0 {
		tunDevice="tun0";
	}
	if len(tunAddr)==0 {
		tunAddr="10.0.0.2";
	}
	if len(tunMask)==0 {
		tunMask="255.255.255.0";
	}
	if len(tunGW)==0 {
		tunGW="10.0.0.1";
	}
	if len(tunDNS)==0 {
		tunDNS="114.114.114.114";
	}
	//
	var oldGw=comm.GetGateway();
	dnsServers := strings.Split(tunDNS, ",")
	var dev io.ReadWriteCloser;
	var remoteAddr string;
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
		if runtime.GOOS=="windows" {
			urlInfo, _ := url.Parse(param.Args.ServerAddr)
			addr, err := net.ResolveIPAddr("ip",urlInfo.Hostname())
			if err == nil {
				remoteAddr = addr.String()
			}
			fmt.Printf("remoteAddr:%s\r\n", remoteAddr)
		}

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

	//windows
	if runtime.GOOS=="windows" {
		oldDns:=comm.GetDnsServer();
		if oldDns!=nil&&len(oldDns)>0 {
			dnsServers = append(dnsServers, oldDns...)
		}
		routeEdit(tunGW,remoteAddr,dnsServers,oldGw);
	}
	tun2socks.ForwardTransportFromIo(dev,param.Args.Mtu,rawTcpForwarder,rawUdpForwarder);
}


func rawTcpForwarder(conn *gonet.TCPConn)error{
	var remoteAddr=conn.LocalAddr().String()
	//dns ,use 8.8.8.8
	if strings.HasSuffix(remoteAddr,":53") {
		dnsReqTcp(conn);
		return  nil;
	}
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

func rawUdpForwarder(conn *gonet.UDPConn, ep tcpip.Endpoint)error{
	defer ep.Close();
	defer conn.Close();
	//dns port
	if strings.HasSuffix(conn.LocalAddr().String(),":53") {
		dnsReqUdp(conn);
	}else{
		dstAddr,_:=net.ResolveUDPAddr("udp",conn.LocalAddr().String())
		tun2socks.SocksUdpGate(conn,dstAddr);
	}
	return nil;
}
func dnsReqUdp(conn *gonet.UDPConn) error{
	dnsConn, err := net.DialTimeout("udp", "127.0.0.1:"+param.Args.DnsPort,time.Second*15);
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	comm.UdpPipe(conn,dnsConn,time.Minute*5)
	return nil;
}
/*to dns*/
func dnsReqTcp(conn *gonet.TCPConn) error{
	dnsConn, err := net.DialTimeout("tcp", "127.0.0.1:"+param.Args.DnsPort,time.Second*15);
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	comm.TcpPipe(conn,dnsConn,time.Minute*2)
	fmt.Printf("dnsReq Tcp\r\n");
	return nil;
}




