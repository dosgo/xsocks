package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"xSocks/comm"
	"xSocks/param"
	"xSocks/server"
)

/*服务功能
  2.启动本地的sock5服务  (通过quic服务转发过来)
  3.启动quic服务,(包括dns解析功能)
*/

func main() {

	flag.StringVar(&param.Sock5Port, "sock5Port", "", "local socks5 port")
	flag.StringVar(&param.TunPort, "TunPort", "", "local tun port")
	flag.StringVar(&param.QuicPort, "quicPort", "5002", "quic port")
	flag.StringVar(&param.WebPort, "webPort", "5003", "quic port")
	flag.StringVar(&param.SctpPort, "sctpPort", "5004", "sctp port")
	flag.StringVar(&param.KcpPort, "kcpPort", "5005", "kcp port")
	flag.StringVar(&param.Password, "password", "password", "password")
	flag.StringVar(&param.KeyFile, "keyFile", "", "keyFile")
	flag.StringVar(&param.CertFile, "certFile", "", "certFile")
	flag.IntVar(&param.Mtu, "mtu", 4500, "mtu")
	flag.Parse()

	//随机端口
	if(param.Sock5Port==""){
		param.Sock5Port,_= comm.GetFreePort();
	}
	if(param.TunPort==""){
		param.TunPort,_= comm.GetFreePort();
	}
	//生成临时目录
	param.LocalTunSock=os.TempDir()+"/"+comm.UniqueId(8)


	fmt.Printf("verison:%s\r\n",param.Version)
	fmt.Printf("socks5 server Port:%s\r\n",param.Sock5Port)
	fmt.Printf("tun  Port:%s\r\n",param.TunPort)
	fmt.Printf("Quic Port:%s\r\n",param.QuicPort)
	fmt.Printf("webSocket Port:%s\r\n",param.WebPort)
	fmt.Printf("passWord:%s\r\n",param.Password)

	var publicIp="0.0.0.0";
	_ip,err:= server.GetPublicIP();
	if(err==nil){
		publicIp=_ip;
	}
	if(publicIp!="0.0.0.0"&&comm.IsPublicIP(net.ParseIP(publicIp))&&!comm.IsChinaMainlandIP(publicIp)){
		param.SafeDns="8.8.4.4"
	}
	fmt.Printf("client run: ./client   -serverAddr \"quic://"+publicIp+":"+param.QuicPort+"\" \r\n")
	fmt.Printf("client run: ./client   -serverAddr \"wss://"+publicIp+":"+param.WebPort+"\" -caFile xx_ca.pem\r\n ")
	fmt.Printf("client run: ./client   -serverAddr \"sctp://"+publicIp+":"+param.SctpPort+"\" \r\n ")
	fmt.Printf("client run: ./client   -serverAddr \"kcp://"+publicIp+":"+param.KcpPort+"\" \r\n ")

	go server.StartRemoteSocks51("127.0.0.1:"+param.Sock5Port);
	go server.StartWebSocket(publicIp+":"+param.WebPort);
	go server.StartSctp(publicIp+":"+param.SctpPort)
	go server.StartKcp(publicIp+":"+param.KcpPort);
	go server.StartTunTcp(); //local tun server
	server.StartQuic(publicIp+":"+param.QuicPort);
}


