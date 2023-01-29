package xsocks

import (
	"time"

	"github.com/dosgo/xsocks/client"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
)

var c *client.Client

func Start(sock5Addr string, serverAddr string, password string, caFile string, skipVerify bool, tunType int, unixSockTun string, muxNum int, localDns int, smartDns int, udpProxy int, mtu int, tunSmartProxy bool, ipFile string) {
	paramParam := param.Args
	if sock5Addr != "" {
		paramParam.Sock5Addr = sock5Addr
	} else {
		paramParam.Sock5Addr = "127.0.0.1:6000"
	}

	if serverAddr != "" {
		paramParam.ServerAddr = serverAddr
	} else {
		paramParam.ServerAddr = "wss://127.0.0.1:5003"
	}

	if password != "" {
		paramParam.Password = password
	} else {
		paramParam.Password = "password"
	}

	if caFile != "" {
		paramParam.CaFile = caFile
	} else {
		paramParam.CaFile = ""
	}

	paramParam.SkipVerify = skipVerify
	paramParam.TunType = tunType
	paramParam.UnixSockTun = unixSockTun

	if muxNum != 0 {
		paramParam.MuxNum = muxNum
	} else {
		paramParam.MuxNum = 4
	}

	paramParam.LocalDns = localDns
	paramParam.SmartDns = smartDns
	paramParam.UdpProxy = udpProxy
	paramParam.TunFd = tunFd

	if mtu != 0 {
		paramParam.Mtu = mtu
	} else {
		paramParam.Mtu = 4500
	}
	if ipFile != "" {
		param.Args.IpFile = ipFile
	} else {
		param.Args.IpFile = ""
	}

	paramParam.TunSmartProxy = tunSmartProxy
	time.Sleep(time.Second * 1)
	c = &client.Client{}
	c.Start()
}

func Shutdown() {
	if c != nil {
		c.Shutdown()
		time.Sleep(time.Second * 1)
	}
}

func Init() {
	comm.Init()
}
