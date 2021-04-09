package client

import (
	"context"
	"fmt"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
	"net"
	"os"
)


type Client struct {
	lSocks5   net.Listener
	lDns  *LocalDns
	tun2Socks *Tun2Socks //tuntype1
	remoteTun *RemoteTun  //tuntype2
	fakeDns *FakeDnsTun  //tuntype3
}
func (c *Client) Shutdown(){
	if c.lSocks5!=nil {
		c.lSocks5.Close();
	}
	if c.lDns!=nil {
		c.lDns.Shutdown();
	}
	if c.fakeDns!=nil {
		c.fakeDns.Shutdown();
	}
	if c.tun2Socks!=nil {
		c.tun2Socks.Shutdown();
	}
	if c.remoteTun!=nil {
		c.remoteTun.Shutdown();
	}
}

func (c *Client) Start() error{
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
		c.tun2Socks=&Tun2Socks{}
		c.tun2Socks.Start("",tunAddr,"",tunGw,"")
	}
	//2==tun2remote tun
	if param.Args.TunType==2 {
		c.remoteTun=&RemoteTun{}
		c.remoteTun.Start("","","","","");
	}
	if param.Args.TunType==3 {
		c.fakeDns=&FakeDnsTun{}
		c.fakeDns.Start("",tunAddr,"",tunGw,"");
	}
	c.lDns=&LocalDns{}
	c.lDns.StartDns();
	var err error;
	c.lSocks5,err=StartLocalSocks5(param.Args.Sock5Addr);
	return err;
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