package main

import (
	"flag"

	"github.com/dosgo/xsocks/param"
	"github.com/dosgo/xsocks/server"
	//"github.com/dosgo/xsocks/server"
)

/*服务功能
  2.启动本地的sock5服务  (通过quic服务转发过来)
  3.启动quic服务,(包括dns解析功能)
*/

func main() {
	paramParam := param.Args
	flag.StringVar(&paramParam.QuicPort, "quicPort", "5002", "quic port")
	flag.StringVar(&paramParam.WebPort, "webPort", "5003", "websocket port")
	flag.StringVar(&paramParam.Password, "password", "password", "password")
	flag.StringVar(&paramParam.KeyFile, "keyFile", "", "keyFile")
	flag.StringVar(&paramParam.CertFile, "certFile", "", "certFile")
	flag.IntVar(&paramParam.Mtu, "mtu", 4500, "mtu")
	flag.Parse()

	server.Start()
	select {}
}
