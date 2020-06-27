package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"qproxy/client"
	"qproxy/ipcheck"
	"qproxy/param"
	"qproxy/server"
)



func main() {

	flag.StringVar(&param.Sock5Addr, "sock5Addr", "127.0.0.1:6000", "remote socks5 addr ")
	//"quic://127.0.0.1:5002" or "wss://127.0.0.1:5003"
	flag.StringVar(&param.ServerAddr, "serverAddr", "wss://127.0.0.1:5003", "remote  addr")
	flag.StringVar(&param.Password, "password", "password", "password")
	flag.StringVar(&param.CaFile, "caFile", "", "RootCAs file")
	flag.BoolVar(&param.SkipVerify, "skipVerify", false, "SkipVerify")
	flag.BoolVar(&param.Tun2Socks, "tun2Socks", false, "start Tun2Socks server")
	flag.StringVar(&param.UnixSockTun, "unixSockTun", "", "unix socket tun")
	flag.IntVar(&param.Mux, "mux", 1, "use Multiplexer")
	flag.IntVar(&param.LocalDns, "localDns", 1, "use local dns")
	flag.IntVar(&param.Mtu, "mtu", 4500, "mtu")



	flag.Parse()

	//随机端口
	if(param.DnsPort==""){
		_dnsPort,_:= client.GetFreePort();
		param.DnsPort= fmt.Sprintf("%d", _dnsPort)
	}

	fmt.Printf("verison:%s\r\n",param.Version)
	fmt.Printf("socks5 addr :%s\r\n",param.Sock5Addr)
	if(param.Tun2Socks){
		go server.StartTunDevice("","","","","");
	}
	go server.StartDns();
	server.StartLocalSocks5(param.Sock5Addr);
}
func init(){
	//android
	if(os.Getenv("ANDROID_DATA")!=""){
		fmt.Printf("setDefaultDNS\r\n ")
		setDefaultDNS("114.114.114.114:53");
	}
	ipcheck.Init()
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