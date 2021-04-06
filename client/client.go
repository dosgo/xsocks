package client

import (
	"context"
	"github.com/miekg/dns"
	"io"
	"net"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
	"os"
	"fmt"
)


type Client struct {
	lSocks5  *LocalSocks5
	dnsUdp  *dns.Server
	dnsTcp  *dns.Server
	tunDev io.ReadWriteCloser
	fakeDns *FakeDns
}
func (c *Client) Shutdown(){
	if c.lSocks5!=nil {
		c.lSocks5.Shutdown();
	}
	if c.dnsTcp!=nil {
		c.dnsTcp.Shutdown();
	}
	if c.dnsUdp!=nil {
		c.dnsUdp.Shutdown();
	}
	if c.tunDev!=nil {
		c.tunDev.Close();
	}
	if c.fakeDns!=nil {
		c.fakeDns.Shutdown();
	}
}

func (c *Client) Start( ){
	//随机端口
	if param.Args.DnsPort=="" {
		param.Args.DnsPort,_= comm.GetFreePort();
	}
	if param.Args.Sock5UdpPort=="" {
		param.Args.Sock5UdpPort,_= comm.GetFreeUdpPort();
	}
	fmt.Printf("verison:%s\r\n",param.Args.Version)
	fmt.Printf("server addr:%s\r\n",param.Args.ServerAddr)
	fmt.Printf("socks5 addr :%s\r\n",param.Args.Sock5Addr)
	fmt.Printf("Sock5UdpPort:%s\r\n",param.Args.Sock5UdpPort)

	var tunAddr=""
	var tunGw=""
	//no android
	if os.Getenv("ANDROID_DATA")=="" {
		tunAddr, tunGw = comm.GetUnusedTunAddr();
	}
	//1==tun2sock
	if param.Args.TunType==1 {
		 c.tunDev=StartTunDevice("",tunAddr,"",tunGw,"");
	}
	//2==tun2remote tun
	if param.Args.TunType==2 {
		c.tunDev,_=StartTun("","","","","");
	}
	if param.Args.TunType==3 {
		c.fakeDns=&FakeDns{}
		c.fakeDns.Start("",tunAddr,"",tunGw,"");
	}
	c.dnsUdp,c.dnsTcp,_=StartDns();
	c.lSocks5=&LocalSocks5{}
	go c.lSocks5.Start(param.Args.Sock5Addr);
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