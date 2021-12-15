package client

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/dosgo/xsocks/client/tunnel"
	"github.com/dosgo/xsocks/client/tunnelcomm"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
)

type Client struct {
	isStart   bool
	lSocks5   *LocalSocks
	lDns      *LocalDns
	tun2Socks *Tun2Socks  //tuntype1
	remoteTun *RemoteTun  //tuntype2
	fakeDns   *FakeDnsTun //tuntype3
}

func (c *Client) Shutdown() {
	if c.lSocks5 != nil {
		c.lSocks5.Shutdown()
	}
	if c.lDns != nil {
		c.lDns.Shutdown()
	}
	if c.fakeDns != nil {
		c.fakeDns.Shutdown()
	}
	if c.tun2Socks != nil {
		c.tun2Socks.Shutdown()
	}
	if c.remoteTun != nil {
		c.remoteTun.Shutdown()
	}
	//关闭代理
	if param.Args.TunType == 4 && runtime.GOOS == "windows" {
		comm.CloseSystenProxy()
	}
	c.isStart = false
}

func (c *Client) Start() error {
	if c.isStart {
		fmt.Printf("clien runing...\r\n")
		return errors.New("clien runing...\n")
	}
	c.isStart = true
	//init tunnel
	initTunnel()

	//随机端口
	if param.Args.DnsPort == "" {
		param.Args.DnsPort, _ = comm.GetFreePort()
	}
	if param.Args.Sock5UdpPort == "" {
		param.Args.Sock5UdpPort, _ = comm.GetFreeUdpPort()
	}
	fmt.Printf("verison:%s\r\n", param.Args.Version)
	fmt.Printf("server addr:%s\r\n", param.Args.ServerAddr)
	fmt.Printf("socks5 addr :%s\r\n", param.Args.Sock5Addr)
	fmt.Printf("Sock5UdpPort:%s\r\n", param.Args.Sock5UdpPort)

	var tunAddr = ""
	var tunGw = ""
	//no android
	if os.Getenv("ANDROID_DATA") == "" {
		tunAddr, tunGw = comm.GetUnusedTunAddr()
	}
	//1==tun2sock  (android+linux(use iptable))
	if param.Args.TunType == 1 {
		if runtime.GOOS == "windows" {
			fmt.Printf("Windows does not support the TUNTYPE 1 parameter, use TUNTYPE 3\r\n")
		} else {
			c.tun2Socks = &Tun2Socks{}
			c.tun2Socks.Start("", tunAddr, "", tunGw, "")
		}
	}
	//2==tun2remote tun (android)
	if param.Args.TunType == 2 {
		if runtime.GOOS == "windows" {
			fmt.Printf("Windows does not support the TUNTYPE 2 parameter, use TUNTYPE 3\r\n")
		} else {
			c.remoteTun = &RemoteTun{}
			c.remoteTun.Start("", "", "", "", "")
		}
	}
	//windows + linux +mac
	if param.Args.TunType == 3 {
		c.fakeDns = &FakeDnsTun{}
		c.fakeDns.Start(3, param.Args.UdpProxy == 1, "", tunAddr, "", tunGw, "")
	}

	//only windows  (system proxy)
	if param.Args.TunType == 4 {
		if runtime.GOOS != "windows" {
			fmt.Printf("TunType 4 supports Windows only\r\n")
		} else {
			comm.SetSystenProxy("socks://"+param.Args.Sock5Addr, "", true)
		}
	}
	//to local safe socks5(udp support)
	if param.Args.TunType == 5 {
		if !strings.HasPrefix(param.Args.ServerAddr, "socks5") {
			fmt.Printf("-serverAddr socks5://127.0.0.1:1080 \r\n")
			return errors.New("-tuntype 5 -serverAddr socks5://127.0.0.1:1080")
		}
		c.fakeDns = &FakeDnsTun{}
		c.fakeDns.Start(5, param.Args.UdpProxy == 1, "", tunAddr, "", tunGw, "")
	}

	if param.Args.TunType != 5 {
		c.lDns = &LocalDns{}
		c.lDns.StartDns()
		c.lSocks5 = &LocalSocks{}
		return c.lSocks5.Start(param.Args.Sock5Addr)
	}
	return nil
}
func init() {
	if runtime.GOOS != "android" {
		comm.Init()
	}
}

func initTunnel() {
	var isSocks = true
	//初始化连接通道
	var tunnelDialer tunnelcomm.CallerTunnel
	var tunnelUrl string
	if strings.HasPrefix(param.Args.ServerAddr, "wss") {
		tunnelDialer = tunnel.NewWsYamuxDialer()
		tunnelUrl = param.Args.ServerAddr
	} else if strings.HasPrefix(param.Args.ServerAddr, "http2") {
		tunnelDialer = tunnel.NewHttp2Dialer()
		tunnelUrl = "https" + param.Args.ServerAddr[5:]
	} else if strings.HasPrefix(param.Args.ServerAddr, "http") {
		tunnelDialer = tunnel.NewHttpDialer()
		tunnelUrl = "https" + param.Args.ServerAddr[4:]
	} else if strings.HasPrefix(param.Args.ServerAddr, "quic") {
		tunnelDialer = tunnel.NewQuicDialer()
		tunnelUrl = param.Args.ServerAddr[7:]
	} else if strings.HasPrefix(param.Args.ServerAddr, "kcp") {
		tunnelDialer = tunnel.NewKcpDialer()
		tunnelUrl = param.Args.ServerAddr[6:]
	} else {
		return
	}
	//init tunnel
	tunnelcomm.SetTunnel(tunnelDialer, tunnelUrl, param.Args.Password, isSocks)
}
