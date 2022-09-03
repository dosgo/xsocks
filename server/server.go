package server

import (
	"fmt"
	"net"
	"runtime"

	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
	"github.com/miekg/dns"
)

func Start() {
	paramParam := param.Args
	fmt.Printf("verison:%s\r\n", paramParam.Version)
	fmt.Printf("socks5 server Port:%s\r\n", paramParam.Sock5Port)
	fmt.Printf("Quic Port:%s\r\n", paramParam.QuicPort)
	fmt.Printf("webSocket Port:%s\r\n", paramParam.WebPort)
	fmt.Printf("passWord:%s\r\n", paramParam.Password)

	var publicIp = "0.0.0.0"
	_ip, err := GetPublicIP()
	if err == nil {
		publicIp = _ip
	}
	if publicIp != "0.0.0.0" && comm.IsPublicIP(net.ParseIP(publicIp)) && !comm.IsChinaMainlandIP(publicIp) {
		paramParam.SafeDns = "8.8.4.4"
	}
	if runtime.GOOS == "linux" {
		config, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err == nil && len(config.Servers) > 0 && config.Servers[0] != "" {
			paramParam.SafeDns = config.Servers[0]
		}
	}

	fmt.Println("client run: ./client   -serverAddr \"quic://" + publicIp + ":" + paramParam.QuicPort + "\"")
	fmt.Println("client run: ./client   -serverAddr \"wss://" + publicIp + ":" + paramParam.WebPort + "\" -caFile xx_ca.pem")
	fmt.Println("client run: ./client   -serverAddr \"http2://" + publicIp + ":" + paramParam.WebPort + "\" -caFile xx_ca.pem")
	fmt.Println("client run: ./client   -serverAddr \"kcp://" + publicIp + ":" + paramParam.KcpPort + "\"")
	fmt.Println("client run: ./client   -tunType 2   -serverAddr \"sudp://" + publicIp + ":" + paramParam.SudpPort + "\"")

	go StartRemoteSocks51("127.0.0.1:" + paramParam.Sock5Port)
	go StartWeb(publicIp + ":" + paramParam.WebPort)
	go StartKcp(publicIp + ":" + paramParam.KcpPort)
	go StartQuic(publicIp + ":" + paramParam.QuicPort)
}
