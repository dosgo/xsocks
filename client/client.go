package client

import (
	"context"
	"fmt"
	"github.com/dosgo/xsocks/client/tunnel"
	"github.com/dosgo/xsocks/client/tunnelcomm"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
	"errors"
	"net"
	"os"
	"runtime"
	"strings"
)


type Client struct {
	isStart bool;
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
	//关闭代理
	if param.Args.TunType==4 && runtime.GOOS=="windows" {
		comm.CloseSystenProxy();
	}
	c.isStart=false;
}

func (c *Client) Start() error{
	if c.isStart {
		fmt.Printf("clien runing...\r\n");
		return errors.New("clien runing...\n");
	}
	c.isStart=true;
	//init tunnel
	initTunnel();

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
	//1==tun2sock  (android+linux(use iptable))
	if param.Args.TunType==1 {
		if runtime.GOOS=="windows"{
			fmt.Printf("Windows does not support the TUNTYPE 1 parameter, use TUNTYPE 3\r\n")
		}else {
			c.tun2Socks = &Tun2Socks{}
			c.tun2Socks.Start("", tunAddr, "", tunGw, "")
		}
	}
	//2==tun2remote tun (android)
	if param.Args.TunType==2 {
		c.remoteTun=&RemoteTun{}
		c.remoteTun.Start("","","","","");
	}
	//windows + linux +mac
	if param.Args.TunType==3 {
		c.fakeDns=&FakeDnsTun{}
		c.fakeDns.Start("",tunAddr,"",tunGw,"");
	}

	//only windows  (system proxy)
	if param.Args.TunType==4 {
		if runtime.GOOS!="windows"{
			fmt.Printf("TunType 4 supports Windows only\r\n")
		}else{
			comm.SetSystenProxy("socks://"+param.Args.Sock5Addr,"",true);
		}
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

func initTunnel(){
	//初始化连接通道
	var tunnelDialer tunnelcomm.CallerTunnel;
	var tunnelUrl string;
	if strings.HasPrefix(param.Args.ServerAddr,"wss") {
		tunnelDialer=tunnel.NewWsYamuxDialer()
		tunnelUrl=param.Args.ServerAddr
	}else if strings.HasPrefix(param.Args.ServerAddr,"http2") {
		tunnelDialer= tunnel.NewHttp2Dialer()
		tunnelUrl="https"+param.Args.ServerAddr[5:]
	}else if strings.HasPrefix(param.Args.ServerAddr,"http") {
		tunnelDialer= tunnel.NewHttpDialer()
		tunnelUrl="https"+param.Args.ServerAddr[4:]
	}else if strings.HasPrefix(param.Args.ServerAddr,"quic") {
		tunnelDialer= tunnel.NewQuicDialer()
		tunnelUrl=param.Args.ServerAddr[7:]
	}else if strings.HasPrefix(param.Args.ServerAddr,"kcp") {
		tunnelDialer= tunnel.NewKcpDialer()
		tunnelUrl=param.Args.ServerAddr[6:]
	}
	//init tunnel
	tunnelcomm.SetTunnel(tunnelDialer,tunnelUrl,param.Args.Password)
}