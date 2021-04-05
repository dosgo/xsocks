package main

import (
	"flag"
	"os"
	"github.com/dosgo/xSocks/comm"
	"github.com/dosgo/xSocks/param"
	"github.com/dosgo/xSocks/server"

	//"github.com/dosgo/xSocks/server"
)

/*服务功能
  2.启动本地的sock5服务  (通过quic服务转发过来)
  3.启动quic服务,(包括dns解析功能)
*/

func main() {
	paramParam:=param.Args;
	flag.StringVar(&paramParam.Sock5Port, "sock5Port", "", "local socks5 port")
	flag.StringVar(&paramParam.QuicPort, "quicPort", "5002", "quic port")
	flag.StringVar(&paramParam.WebPort, "webPort", "5003", "websocket port")
	flag.StringVar(&paramParam.KcpPort, "kcpPort", "5005", "kcp port")
	flag.StringVar(&paramParam.SudpPort, "sudpPort", "5006", "sudp port")
	flag.StringVar(&paramParam.Password, "password", "password", "password")
	flag.StringVar(&paramParam.KeyFile, "keyFile", "", "keyFile")
	flag.StringVar(&paramParam.CertFile, "certFile", "", "certFile")
	flag.IntVar(&paramParam.Mtu, "mtu", 4500, "mtu")
	flag.Parse()

	if paramParam.UdpGatePort=="" {
		paramParam.UdpGatePort,_= comm.GetFreeUdpPort();
	}
	//随机端口
	if paramParam.Sock5Port=="" {
		paramParam.Sock5Port,_= comm.GetFreePort();
	}
	//生成临时目录
	paramParam.LocalTunSock=os.TempDir()+"/"+comm.UniqueId(8)

	server.Start()
	select {

	}
}


