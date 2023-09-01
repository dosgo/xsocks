package server

import (
	"log"
	"net"
	"runtime"

	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
	"github.com/miekg/dns"
)

func Start() {
	paramParam := param.Args
	log.Printf("verison:%s\r\n", paramParam.Version)
	log.Printf("socks5 server Port:%s\r\n", paramParam.Sock5Port)
	log.Printf("Quic Port:%s\r\n", paramParam.QuicPort)
	log.Printf("webSocket Port:%s\r\n", paramParam.WebPort)
	log.Printf("passWord:%s\r\n", paramParam.Password)

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

	log.Println("client run: ./client   -serverAddr \"quic://" + publicIp + ":" + paramParam.QuicPort + "\"")
	log.Println("client run: ./client   -serverAddr \"wss://" + publicIp + ":" + paramParam.WebPort + "\" -caFile xx_ca.pem")
	log.Println("client run: ./client   -serverAddr \"http2://" + publicIp + ":" + paramParam.WebPort + "\" -caFile xx_ca.pem")

	go StartRemoteSocks51("127.0.0.1:" + paramParam.Sock5Port)
	go StartWeb(publicIp + ":" + paramParam.WebPort)
	go StartQuic(publicIp + ":" + paramParam.QuicPort)
}
