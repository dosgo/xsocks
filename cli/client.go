package main

import (
	"flag"
	"xSocks/client"
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
	flag.IntVar(&param.SmartDns, "smartDns", 1, "use smart dns")
	flag.IntVar(&param.Mtu, "mtu", 4500, "mtu")
	flag.BoolVar(&param.TunSmartProxy,"tunSmartProxy",false,"tun Smart Proxy ")

	flag.Parse()
	client.Start()
	select {

	}
}
