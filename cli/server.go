package main

import (
	"flag"
	"fmt"
	"qproxy/client"
	"qproxy/param"
	"qproxy/server"
)

/*服务功能
  2.启动本地的sock5服务  (通过quic服务转发过来)
  3.启动quic服务,(包括dns解析功能)
*/

func main() {


	flag.StringVar(&param.Sock5Port, "sock5Port", "", "local socks5 port")
	flag.StringVar(&param.QuicPort, "quicPort", "5002", "quic port")
	flag.StringVar(&param.WebPort, "webPort", "5003", "quic port")
	flag.StringVar(&param.Password, "password", "password", "password")
	flag.StringVar(&param.KeyFile, "keyFile", "", "keyFile")
	flag.StringVar(&param.CertFile, "certFile", "", "certFile")


	flag.Parse()

	//随机端口
	if(param.Sock5Port==""){
		_sock5Port,_:= client.GetFreePort();
		param.Sock5Port= fmt.Sprintf("%d", _sock5Port)
	}

	fmt.Printf("verison:%s\r\n",param.Version)
	fmt.Printf("socks5 server Port:%s\r\n",param.Sock5Port)
	fmt.Printf("Quic Port:%s\r\n",param.QuicPort)
	fmt.Printf("webSocket Port:%s\r\n",param.WebPort)
	fmt.Printf("passWord:%s\r\n",param.Password)

	var publicIp="0.0.0.0";
	_ip,err:= client.GetPublicIP();
	if(err==nil){
		publicIp=_ip;
	}

	fmt.Printf("client run: ./client   -serverAddr \"quic://"+publicIp+":"+param.QuicPort+"\" \r\n")
	fmt.Printf("client run: ./client   -serverAddr \"wss://"+publicIp+":"+param.WebPort+"\" -caFile xx_ca.pem\r\n ")


	go server.StartRemoteSocks51("127.0.0.1:"+param.Sock5Port);
	go server.StartWebSocket(publicIp+":"+param.WebPort);
	server.StartQuic(publicIp+":"+param.QuicPort);

}



