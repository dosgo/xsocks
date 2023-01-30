package client

import (
	"errors"
	"io"
	"log"
	"net"
	"os"

	"github.com/dosgo/xsocks/comm"
)

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

/*android use unix Socket */
func SocketToTun(unixSockTun string) (io.ReadWriteCloser, error) {
	if len(unixSockTun) > 0 {
		os.Remove(unixSockTun)
		addr, err := net.ResolveUnixAddr("unixpacket", unixSockTun)
		if err != nil {
			return nil, err
		}
		lis, err := net.ListenUnix("unixpacket", addr)
		if err != nil { //如果监听失败，一般是文件已存在，需要删除它
			log.Println("UNIX Domain Socket 创 建失败，正在尝试重新创建 -> ", err)
			os.Remove(unixSockTun)
			return nil, err
		}
		conn, err := lis.Accept() //开始接 受数据
		if err != nil {           //如果监听失败，一般是文件已存在，需要删除它
			return nil, err
		}
		return conn, nil
	}
	return nil, errors.New("unixSockTun null")
}

func FdToConn(fd int) (io.ReadWriteCloser, error) {
	f := os.NewFile(uintptr(fd), "Socket")
	return net.FileConn(f)
}
