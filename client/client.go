package client

import (
	"context"
	"net"
	"xSocks/comm"
	"xSocks/param"
	"os"
	"fmt"
)

func Start(){
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

	var tunAddr=""
	var tunGw=""
	//no android
	if os.Getenv("ANDROID_DATA")=="" {
		tunAddr, tunGw = comm.GetUnusedTunAddr();
	}
	//1==tun2sock
	if param.TunType==1 {
		go StartTunDevice("",tunAddr,"",tunGw,"");
	}
	//2==tun2remote tun
	if param.TunType==2 {
		go StartTun("","","","","");
	}
	if param.TunType==3 {
		go StartTunDns("",tunAddr,"",tunGw,"");
	}
	go StartDns();
	go StartLocalSocks5(param.Sock5Addr);
}

func init(){
	//android
	if os.Getenv("ANDROID_DATA")!="" {
		fmt.Printf("setDefaultDNS\r\n ")
		setDefaultDNS("114.114.114.114:53");
	}
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