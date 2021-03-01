package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"xSocks/client"
	"context"
	"xSocks/comm"
	"xSocks/param"
)



func main() {

	flag.StringVar(&param.Sock5Addr, "sock5Addr", "127.0.0.1:6000", "remote socks5 addr ")
	//"quic://127.0.0.1:5002" or "wss://127.0.0.1:5003"
	flag.StringVar(&param.ServerAddr, "serverAddr", "wss://127.0.0.1:5003", "remote  addr")
	flag.StringVar(&param.Password, "password", "password", "password")
	flag.StringVar(&param.CaFile, "caFile", "", "RootCAs file")
	flag.BoolVar(&param.SkipVerify, "skipVerify", false, "SkipVerify")
	flag.IntVar(&param.TunType, "tunType", 0, "tun type 1.tun2sock 2.tun2Remote")
	flag.StringVar(&param.UnixSockTun, "unixSockTun", "", "unix socket tun")
	flag.IntVar(&param.MuxNum, "muxNum", 4, "multiplexer Num")
	flag.IntVar(&param.LocalDns, "localDns", 0, "use local dns")
	flag.IntVar(&param.Mtu, "mtu", 4500, "mtu")
	flag.BoolVar(&param.TunSmartProxy,"tunSmartProxy",false,"tun Smart Proxy ")

	flag.Parse()

	//随机端口
	if param.DnsPort=="" {
		param.DnsPort,_= comm.GetFreePort();
	}

	if param.Sock5UdpPort=="" {
		param.Sock5UdpPort,_= comm.GetFreeUdpPort();
	}
	fmt.Printf("verison:%s\r\n",param.Version)
	fmt.Printf("server addr:%s\r\n",param.ServerAddr)
	fmt.Printf("socks5 addr :%s\r\n",param.Sock5Addr)
	fmt.Printf("Sock5UdpPort:%s\r\n",param.Sock5UdpPort)
	//1==tun2sock
	if param.TunType==1 {
		go client.StartTunDevice("","","","","");
	}
	//2==tun2remote tun
	if param.TunType==2 {
		go client.StartTun("","","","","");
	}
	go client.StartDns();
	client.StartLocalSocks5(param.Sock5Addr);


}
func init(){
	//android
	if os.Getenv("ANDROID_DATA")!="" {
		fmt.Printf("setDefaultDNS\r\n ")
		setDefaultDNS("114.114.114.114:53");
	}
	setDefaultDNS("114.114.114.114:53");
	comm.Init()
}
func setDefaultDNS(addrs string) {
	net.DefaultResolver=&net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp",addrs)
		},
	}
}