package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/dosgo/xsocks/comm"
	socksTapComm "github.com/dosgo/goSocksTap/comm"
	"github.com/dosgo/xsocks/comm/socks"
	"github.com/dosgo/xsocks/param"
)

func StartRemoteSocks51(address string) error {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	l, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	//start udp Gate
	udpAddr, err := startUdpGate("127.0.0.1:" + param.Args.UdpGatePort)
	if err != nil {
		return err
	}
	param.Args.UdpGatePort = fmt.Sprintf("%d", udpAddr.Port)
	for {
		client, err := l.Accept()
		if err != nil {
			return err
		}
		go handleRemoteRequest(client, udpAddr)
	}
	return nil
}

var udpNat sync.Map

/*这不是socks5协议的*/
func startUdpGate(address string) (*net.UDPAddr, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 1024*10)
	go func() {
		for {
			n, localAddr, err := udpListener.ReadFromUDP(buf[0:])
			if err != nil {
				break
			}
			data := buf[0:n]
			natSawp(udpListener, data, localAddr)
		}
	}()
	return udpAddr, nil
}

/*udp nat sawp*/
func natSawp(udpGate *net.UDPConn, data []byte, localAddr *net.UDPAddr) {
	srcAddr, dstAddr, err := comm.UdpNatDecode(data)
	if err != nil || dstAddr == nil {
		return
	}
	natKey := srcAddr.String() + "_" + dstAddr.String()
	var remoteConn net.Conn
	_conn, ok := udpNat.Load(natKey)
	if !ok {
		remoteConn, err = net.DialTimeout("udp", dstAddr.String(), time.Second*15)
		if err != nil {
			return
		}
		buf := make([]byte, 1024*10)
		var buffer bytes.Buffer
		udpNat.Store(natKey, remoteConn)
		go func(_remoteConn net.Conn) {
			defer udpNat.Delete(natKey)
			defer _remoteConn.Close()
			for {
				_remoteConn.SetReadDeadline(time.Now().Add(2 * time.Minute))
				n, err := _remoteConn.Read(buf)
				if err != nil {
					log.Printf("err:%v\r\n", err)
					return
				}
				buffer.Reset()
				buffer.Write(comm.UdpNatEncode(srcAddr, dstAddr))
				buffer.Write(buf[:n])
				udpGate.WriteToUDP(buffer.Bytes(), localAddr)
			}
		}(remoteConn)
	} else {
		remoteConn = _conn.(net.Conn)
	}
	remoteConn.Write(data[12:])
}

/*remote use*/
func handleRemoteRequest(clientConn net.Conn, udpAddr *net.UDPAddr) {
	if clientConn == nil {
		return
	}
	clientConn.SetDeadline(time.Now().Add(time.Second * 59))
	defer clientConn.Close()
	auth := make([]byte, 3)
	_, err := io.ReadFull(clientConn, auth)
	if err != nil {
		log.Printf("err:%v\r\n", err)
		return
	}
	if auth[0] == 0x05 {
		//resp auth
		clientConn.Write([]byte{0x05, 0x00})
	} else {
		log.Printf("auth[0]!=0x05\r\n")
		return
	}
	connectHead := make([]byte, 4)
	_, err = io.ReadFull(clientConn, connectHead)
	if err != nil {
		log.Printf("err:%v\r\n", err)
		return
	}

	if connectHead[0] == 0x05 {

		if connectHead[1] == 0x01 {
			var host, port string
			switch connectHead[3] {
			case 0x01: //IP V4
				ipv4 := make([]byte, 4)
				_, err = io.ReadFull(clientConn, ipv4)
				host = net.IPv4(ipv4[0], ipv4[1], ipv4[2], ipv4[3]).String()
				break
			case 0x03: //域名
				hostLen := make([]byte, 1)
				_, err = io.ReadFull(clientConn, hostLen)
				hostBuf := make([]byte, hostLen[0])
				_, err = io.ReadFull(clientConn, hostBuf)
				host = string(hostBuf) //b[4]表示域名的长度
				log.Printf("host:%s\r\n", hostBuf)
				break
			case 0x04: //IP V6
				log.Printf("ipv6\r\n")
				ipv6 := make([]byte, 16)
				_, err = io.ReadFull(clientConn, ipv6)
				host = net.IP{ipv6[0], ipv6[1], ipv6[2], ipv6[3], ipv6[4], ipv6[5], ipv6[6], ipv6[7], ipv6[8], ipv6[9], ipv6[10], ipv6[11], ipv6[12], ipv6[13], ipv6[14], ipv6[15]}.String()
				break
			}
			if len(host) == 0 {
				log.Println("host null")
				return
			}

			portBuf := make([]byte, 2)
			_, err = io.ReadFull(clientConn, portBuf)
			port = strconv.Itoa(int(portBuf[0])<<8 | int(portBuf[1]))
			server, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), param.Args.ConnectTime)
			if err != nil {
				log.Println(err)
				return
			}
			defer server.Close()
			clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) //响应客户端连接成功
			//进行转发
			socksTapComm.TcpPipe(server, clientConn, time.Minute*2)
		}
		//udp
		if connectHead[1] == 0x03 {
			socks.UdpProxyRes(clientConn, udpAddr)
		}
	}
}
