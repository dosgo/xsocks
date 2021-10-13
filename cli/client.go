package main

import (
	"flag"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/dosgo/xsocks/client"
	"github.com/dosgo/xsocks/param"
)

func main() {
	paramParam := param.Args
	flag.StringVar(&paramParam.Sock5Addr, "sock5Addr", "127.0.0.1:6000", "remote socks5 addr ")
	//"quic://127.0.0.1:5002" or "wss://127.0.0.1:5003"
	flag.StringVar(&paramParam.ServerAddr, "serverAddr", "wss://127.0.0.1:5003", "remote  addr")
	flag.StringVar(&paramParam.Password, "password", "password", "password")
	flag.StringVar(&paramParam.CaFile, "caFile", "", "RootCAs file")
	flag.BoolVar(&paramParam.SkipVerify, "skipVerify", false, "SkipVerify")
	flag.IntVar(&paramParam.TunType, "tunType", 0, "tun type 1.tun2sock 2.tun2Remote")
	flag.StringVar(&paramParam.UnixSockTun, "unixSockTun", "", "unix socket tun")
	flag.IntVar(&paramParam.MuxNum, "muxNum", 4, "multiplexer Num")
	flag.IntVar(&paramParam.LocalDns, "localDns", 0, "use local dns")
	flag.IntVar(&paramParam.SmartDns, "smartDns", 1, "use smart dns")
	flag.IntVar(&paramParam.UdpProxy, "udpProxy", 1, "use udpProxy ")
	flag.IntVar(&paramParam.Mtu, "mtu", 4500, "mtu")
	flag.BoolVar(&paramParam.TunSmartProxy, "tunSmartProxy", false, "tun Smart Proxy ")

	flag.Parse()
	c := client.Client{}
	c.Start()
	defer c.Shutdown()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGABRT, syscall.SIGSEGV, syscall.SIGQUIT)
	_ = <-ch
	c.Shutdown()
}
