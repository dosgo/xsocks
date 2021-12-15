package client

import (
	"log"
	"sync"

	"github.com/dosgo/xsocks/comm"
)

var poolDnsBuf = &sync.Pool{
	New: func() interface{} {
		log.Println("new 1")
		return make([]byte, 4096)
	},
}

func regRoute(tunAddr string, remoteAddr string, dnsServers []string, oldGw string) {
	//delete old
	comm.CmdHide("route", "delete", "0.0.0.0").Output()
	// add socks5 add
	comm.CmdHide("route", "add", remoteAddr, oldGw, "metric", "6").Output()
	for _, v := range dnsServers {
		// add dns add
		comm.CmdHide("route", "add", v, oldGw, "metric", "6").Output()
	}
	//route add 0.0.0.0 mask 0.0.0.0 192.168.8.1 metric 6
	comm.CmdHide("route", "add", "0.0.0.0", "mask", "0.0.0.0", tunAddr, "metric", "8").Output()
}

func unRegRoute(tunAddr string, remoteAddr string, dnsServers []string, oldGw string) {

	comm.CmdHide("route", "delete", "0.0.0.0", "mask", "0.0.0.0", tunAddr, "metric", "6").Output()
	//route add old
	comm.CmdHide("route", "add", "0.0.0.0", "mask", "0.0.0.0", oldGw, "metric", "8").Output()
	//delete remoteAddr
	comm.CmdHide("route", "delete", remoteAddr).Output()
	//delete dns
	for _, v := range dnsServers {
		// delete dns
		comm.CmdHide("route", "delete", v).Output()
	}
}

type SafeDns interface {
	Resolve(remoteHost string, ipType int) (string, error)
}
