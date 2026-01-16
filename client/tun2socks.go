package client

import (
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/dosgo/go-tun2socks/core"
	"github.com/dosgo/go-tun2socks/tun"
	"github.com/dosgo/go-tun2socks/tun2socks"
	"github.com/dosgo/xsocks/comm"
	socksTapComm "github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/comm/socks"
	"github.com/dosgo/xsocks/param"
)

type Tun2Socks struct {
	tunDev     io.ReadWriteCloser
	remoteAddr string
	dnsServers []string
	oldGw      string
	tunGW      string
}

/*tunType==1*/
func (_tun2socks *Tun2Socks) Start(tunDevice string, tunAddr string, tunMask string, tunGW string, tunDNS string) error {
	_tun2socks.oldGw = comm.GetGateway()
	_tun2socks.tunGW = tunGW
	_tun2socks.dnsServers = strings.Split(tunDNS, ",")
	var err error
	if len(param.Args.UnixSockTun) > 0 {
		_tun2socks.tunDev, err = SocketToTun(param.Args.UnixSockTun)
		if err != nil { //如果监听失败，一般是文件已存在，需要删除它
			return err
		}
	} else if param.Args.TunFd > 0 {
		_tun2socks.tunDev, err = FdToIO(param.Args.TunFd)
		if err != nil {
			return err
		}
	} else {
		_tun2socks.tunDev, err = tun.RegTunDev(tunDevice, tunAddr, tunMask, tunGW, tunDNS)
		if err != nil {
			log.Println("start tun err:", err)
			return err
		}
	}
	go tun2socks.ForwardTransportFromIo(_tun2socks.tunDev, param.Args.Mtu, rawTcpForwarder, rawUdpForwarder)
	return nil
}

/**/
func (_tun2socks *Tun2Socks) Shutdown() {
	if _tun2socks.tunDev != nil {
		_tun2socks.tunDev.Close()
	}
	unRegRoute(_tun2socks.tunGW, _tun2socks.remoteAddr, _tun2socks.dnsServers, _tun2socks.oldGw)
}

func rawTcpForwarder(conn core.CommTCPConn) error {
	var remoteAddr = conn.LocalAddr().String()
	//dns ,use 8.8.8.8
	if strings.HasSuffix(remoteAddr, ":53") {
		dnsReqTcp(conn)
		return nil
	}
	socksConn, err1 := net.DialTimeout("tcp", param.Args.Sock5Addr, time.Second*15)
	if err1 != nil {
		log.Printf("err:%v", err1)
		return nil
	}
	defer socksConn.Close()
	if socks.SocksCmd(socksConn, 1, 1, remoteAddr, true) == nil {
		socksTapComm.ConnPipe(conn, socksConn, time.Minute*2)
	}
	return nil
}

func rawUdpForwarder(conn core.CommUDPConn, ep core.CommEndpoint) error {
	//dns port
	if strings.HasSuffix(conn.LocalAddr().String(), ":53") {
		defer ep.Close()
		dnsReqUdp(conn)
	} else {
		defer ep.Close()
		dstAddr, _ := net.ResolveUDPAddr("udp", conn.LocalAddr().String())
		socks.SocksUdpGate(conn, "127.0.0.1:"+param.Args.Sock5UdpPort, dstAddr)
	}
	return nil
}
func dnsReqUdp(conn core.CommUDPConn) error {
	defer conn.Close()
	buf := make([]byte, 2048)
	gateConn, err := net.DialTimeout("udp", "127.0.0.1:"+param.Args.DnsPort, time.Second*15)
	go func() {
		buf1 := make([]byte, 2048)
		var readLen = 0
		for {
			gateConn.SetReadDeadline(time.Now().Add(time.Second * 65))
			readLen, err = gateConn.Read(buf1)
			if err != nil {
				break
			}
			conn.Write(buf1[:readLen])
		}
	}()

	for {
		conn.SetReadDeadline(time.Now().Add(time.Second * 65))
		udpSize, err := conn.Read(buf)
		if err == nil {
			gateConn.Write(buf[:udpSize])
		} else {
			break
		}
	}
	return nil
}

/*to dns*/
func dnsReqTcp(conn core.CommTCPConn) error {
	dnsConn, err := net.DialTimeout("tcp", "127.0.0.1:"+param.Args.DnsPort, time.Second*15)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	socksTapComm.ConnPipe(conn, dnsConn, time.Minute*2)
	log.Printf("dnsReq Tcp\r\n")
	return nil
}
