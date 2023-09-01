package client

import (
	"errors"
	"log"
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

	c.isStart = false
}

func (c *Client) Start() error {
	if c.isStart {
		log.Printf("clien runing...\r\n")
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
	log.Printf("verison:%s\r\n", param.Args.Version)
	log.Printf("server addr:%s\r\n", param.Args.ServerAddr)
	log.Printf("socks5 addr :%s\r\n", param.Args.Sock5Addr)
	log.Printf("Sock5UdpPort:%s\r\n", param.Args.Sock5UdpPort)

	var tunAddr = ""
	var tunGw = ""
	//no android
	if param.Args.TunFd == 0 {
		tunAddr, tunGw = comm.GetUnusedTunAddr()
	}
	//1==tun2sock  (android)
	//windows use IP_UNICAST_IF   Unrealized
	//linux use  SO_BINDTODEVICE or SO_MARK +rule   Unrealized
	//mac use  SO_BINDTODEVICE  Unrealized
	if param.Args.TunType == 1 {
		if runtime.GOOS == "windows" {
			log.Printf("Windows does not support the TUNTYPE 1 parameter, use TUNTYPE 3\r\n")
		} else {
			c.tun2Socks = &Tun2Socks{}
			err := c.tun2Socks.Start("", tunAddr, "", tunGw, "")
			if err != nil {
				log.Printf("tun2Socks start err:%+v\r\n", err)
			}
		}
	}

	//windows + linux +mac
	if param.Args.TunType == 3 {
		c.fakeDns = &FakeDnsTun{}
		c.fakeDns.Start(3, param.Args.UdpProxy == 1, "", tunAddr, "", tunGw, "")
	}

	//to local safe socks5(udp support) windows + linux +mac
	if param.Args.TunType == 5 {
		if !strings.HasPrefix(param.Args.ServerAddr, "socks5") {
			log.Printf("-serverAddr socks5://127.0.0.1:1080 \r\n")
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
	} else {
		return
	}
	//init tunnel
	tunnelcomm.SetTunnel(tunnelDialer, tunnelUrl, param.Args.Password, isSocks)
}
