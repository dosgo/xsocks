package main

import (
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"net"
	"os"
	"runtime"
	"xSocks/comm"
	"xSocks/param"
	"xSocks/server"

	//"xSocks/server"
)

/*服务功能
  2.启动本地的sock5服务  (通过quic服务转发过来)
  3.启动quic服务,(包括dns解析功能)
*/

func main() {

	flag.StringVar(&param.Sock5Port, "sock5Port", "", "local socks5 port")
	flag.StringVar(&param.QuicPort, "quicPort", "5002", "quic port")
	flag.StringVar(&param.WebPort, "webPort", "5003", "quic port")
	flag.StringVar(&param.KcpPort, "kcpPort", "5005", "kcp port")
	flag.StringVar(&param.SudpPort, "sudpPort", "5006", "sudp port")
	flag.StringVar(&param.Password, "password", "password", "password")
	flag.StringVar(&param.KeyFile, "keyFile", "", "keyFile")
	flag.StringVar(&param.CertFile, "certFile", "", "certFile")
	flag.IntVar(&param.Mtu, "mtu", 4500, "mtu")
	flag.Parse()

	if param.UdpGatePort=="" {
		param.UdpGatePort,_= comm.GetFreeUdpPort();
	}
	//随机端口
	if param.Sock5Port=="" {
		param.Sock5Port,_= comm.GetFreePort();
	}
	//生成临时目录
	param.LocalTunSock=os.TempDir()+"/"+comm.UniqueId(8)


	fmt.Printf("verison:%s\r\n",param.Version)
	fmt.Printf("socks5 server Port:%s\r\n",param.Sock5Port)
	fmt.Printf("Quic Port:%s\r\n",param.QuicPort)
	fmt.Printf("webSocket Port:%s\r\n",param.WebPort)
	fmt.Printf("passWord:%s\r\n",param.Password)

	var publicIp="0.0.0.0";
	_ip,err:= server.GetPublicIP();
	if err==nil {
		publicIp=_ip;
	}
	if publicIp!="0.0.0.0"&&comm.IsPublicIP(net.ParseIP(publicIp))&&!comm.IsChinaMainlandIP(publicIp) {
		param.SafeDns="8.8.4.4"
	}
	if(runtime.GOOS=="linux"){
		config, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if(err==nil&&len(config.Servers)>0&&config.Servers[0]!="") {
			param.SafeDns = config.Servers[0]
		}
	}


	fmt.Println("client run: ./client   -serverAddr \"quic://"+publicIp+":"+param.QuicPort+"\"")
	fmt.Println("client run: ./client   -serverAddr \"wss://"+publicIp+":"+param.WebPort+"\" -caFile xx_ca.pem")
	fmt.Println("client run: ./client   -serverAddr \"kcp://"+publicIp+":"+param.KcpPort+"\"")
	fmt.Println("client run: ./client   -tunType 2   -serverAddr \"sudp://"+publicIp+":"+param.SudpPort+"\"")


	go server.StartRemoteSocks51("127.0.0.1:"+param.Sock5Port);
	go server.StartWebSocket(publicIp+":"+param.WebPort);
	go server.StartKcp(publicIp+":"+param.KcpPort);
	go server.StartSudp(publicIp+":"+param.SudpPort)
	server.StartQuic(publicIp+":"+param.QuicPort);
}


