package client

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	socksTapComm "github.com/dosgo/goSocksTap/comm"
	"github.com/dosgo/xsocks/client/tunnelcomm"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/comm/socks"
	"github.com/dosgo/xsocks/param"
)

type LocalSocks struct {
	l                 net.Listener
	PolluteDomainName sync.Map
	udpProxy          *UdpProxyV1
	tunnel            *tunnelcomm.TunelComm
}

func (lSocks *LocalSocks) Start(address string) error {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var err error
	lSocks.l, err = net.Listen("tcp", address)
	if err != nil {
		return err
	}
	//start udpProxy
	lSocks.udpProxy = &UdpProxyV1{}
	lSocks.udpProxy.tunnel = lSocks.tunnel
	err = lSocks.udpProxy.startUdpProxy("127.0.0.1:" + param.Args.Sock5UdpPort)

	if err != nil {
		return err
	}
	go func() {
		for {
			client, err := lSocks.l.Accept()
			if err != nil {
				return
			}
			go lSocks.handleLocalRequest(client)
		}
	}()
	return nil
}
func (lSocks *LocalSocks) Shutdown() {
	lSocks.l.Close()
	lSocks.udpProxy.Shutdown()
}

type UdpProxyV1 struct {
	udpListener *net.UDPConn
	udpNat      sync.Map
	tunnel      *tunnelcomm.TunelComm
}

/*这里得保持socks5协议兼容*/
func (udpProxy *UdpProxyV1) startUdpProxy(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	udpProxy.udpListener, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	go udpProxy.recv()
	return nil
}

/*这里得保持socks5协议兼容*/
func (udpProxy *UdpProxyV1) recv() {
	buf := make([]byte, 1024*5)
	for {
		n, localAddr, err := udpProxy.udpListener.ReadFromUDP(buf[0:])
		if err != nil {
			break
		}
		data := buf[0:n]
		dstAddr, dataStart, err := socks.UdpHeadDecode(data)
		if err != nil || dstAddr == nil {
			continue
		}
		//判断地址是否合法
		_address := net.ParseIP(dstAddr.IP.String())
		if _address == nil {
			continue
		}
		natKey := localAddr.String() + "_" + dstAddr.String()
		remoteConn, ok := udpProxy.udpNat.Load(natKey)
		if !ok {
			//本地转发
			if !IsProxy(dstAddr.IP) {
				remoteConn, err = net.DialTimeout("udp", dstAddr.String(), time.Second*15)
				if err != nil {
					log.Printf("err:%v\r\n", err)
					return
				}
				udpProxy.udpNat.Store(natKey, remoteConn)
			} else {

				var remoteConn, err = udpProxy.tunnel.NewTunnel()
				if err != nil || remoteConn == nil {
					log.Printf("err:%v\r\n", err)
					return
				}
				defer remoteConn.Close()
				//to tcp
				remoteConn.Write([]byte{0x04})
				var addrLen uint16 = uint16(len(dstAddr.String()))
				err = binary.Write(remoteConn, binary.BigEndian, addrLen)
				if err != nil {
					return
				}
				remoteConn.Write([]byte(dstAddr.String()))
			}
			udpProxy.socksNatSawp(udpProxy.udpListener, remoteConn.(comm.CommConn), localAddr, dstAddr)
		}
		remoteConn.(comm.CommConn).Write(data[dataStart:])
	}
}

/*udp socks5 nat sawp*/
func (udpProxy *UdpProxyV1) socksNatSawp(udpGate *net.UDPConn, remoteConn comm.CommConn, localAddr *net.UDPAddr, dstAddr *net.UDPAddr) {

	buf := make([]byte, 1024*5)
	var buffer bytes.Buffer
	go func() {
		defer remoteConn.Close()
		for {
			remoteConn.SetDeadline(time.Now().Add(2 * time.Minute))
			n, err := remoteConn.Read(buf)
			if err != nil {
				log.Printf("socksNatSawp time out eixt err:%v\r\n", err)
				return
			}
			buffer.Reset()
			buffer.Write(socks.UdpHeadEncode(dstAddr))
			buffer.Write(buf[:n])
			udpGate.WriteToUDP(buffer.Bytes(), localAddr)
		}
	}()
}

/*这里得保持socks5协议兼容*/
func (udpProxy *UdpProxyV1) Shutdown() {
	udpProxy.udpListener.Close()
}

/*local use  smart dns*/
func (lSocks *LocalSocks) handleLocalRequest(clientConn net.Conn) error {
	if clientConn == nil {
		return nil
	}
	clientConn.SetDeadline(time.Now().Add(time.Second * 60))
	defer clientConn.Close()

	auth := make([]byte, 3)
	_, err := io.ReadFull(clientConn, auth)
	if err != nil {
		log.Printf("err:%v", err)
		return err
	}
	if auth[0] == 0x05 {
		//resp auth
		clientConn.Write([]byte{0x05, 0x00})
	}

	connectHead := make([]byte, 4)
	_, err = io.ReadFull(clientConn, connectHead)
	if err != nil {
		log.Printf("err:%v", err)
		return err
	}

	if connectHead[0] == 0x05 {
		//connect tcp
		if connectHead[1] == 0x01 {
			var ipAddr net.IP
			var port string
			var hostBuf []byte
			var hostBufLen []byte
			ipv4 := make([]byte, 4)
			ipv6 := make([]byte, 16)
			//解析
			switch connectHead[3] {
			case 0x01: //IP V4
				_, err = io.ReadFull(clientConn, ipv4)
				ipAddr = net.IPv4(ipv4[0], ipv4[1], ipv4[2], ipv4[3])
				break
			case 0x03: //域名
				hostBufLen = make([]byte, 1)
				_, err = io.ReadFull(clientConn, hostBufLen)
				hostBuf = make([]byte, hostBufLen[0])
				_, err = io.ReadFull(clientConn, hostBuf)
				ip := "8.8.8.8" //随便一个国外的IP地址
				//如果在列表無需解析，直接用遠程
				_, ok := lSocks.PolluteDomainName.Load(string(hostBuf))
				if !ok {
					addr, err := net.ResolveIPAddr("ip", string(hostBuf))
					if err == nil {
						ip = addr.String()
					} else {
						log.Printf("dnserr host:%s  addr:%s err:%v\r\n", string(hostBuf), addr.String(), err)
					}
				}
				ipAddr = net.ParseIP(ip)
				break
			case 0x04: //IP V6
				log.Printf("ipv6\r\n")
				_, err = io.ReadFull(clientConn, ipv6)
				ipAddr = net.IP{ipv6[0], ipv6[1], ipv6[2], ipv6[3], ipv6[4], ipv6[5], ipv6[6], ipv6[7], ipv6[8], ipv6[9], ipv6[10], ipv6[11], ipv6[12], ipv6[13], ipv6[14], ipv6[15]}

				break
			default:
				log.Printf("default connectHead[3]:%v\r\n", connectHead[3])
				break
			}

			//解析失败直接关闭
			if ipAddr == nil || ipAddr.String() == "0.0.0.0" {
				return nil
			}

			portBuf := make([]byte, 2)
			_, err = io.ReadFull(clientConn, portBuf)
			port = strconv.Itoa(int(portBuf[0])<<8 | int(portBuf[1]))

			if !IsProxy(ipAddr) {
				server, err := net.DialTimeout("tcp", net.JoinHostPort(ipAddr.String(), port), 5*time.Second)
				if err != nil {
					log.Printf("host:%s err:%v\r\n", net.JoinHostPort(ipAddr.String(), port), err)
					return err
				}
				defer server.Close()
				clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) //响应客户端连接成功
				//进行转发
				socksTapComm.ConnPipe(server, clientConn, time.Minute*2)

			} else {
				//保存記錄
				lSocks.PolluteDomainName.Store(string(hostBuf), 1)
				var remoteHost = ""
				//使用host
				if connectHead[3] == 0x03 {
					remoteHost = string(hostBuf)
				} else if connectHead[3] == 0x01 {
					remoteHost = ipAddr.To4().String()
				} else if connectHead[3] == 0x04 {
					remoteHost = ipAddr.To16().String()
				}
				remoteHost = remoteHost + ":" + port
				var stream, err = lSocks.tunnel.NewTunnel()
				if err != nil || stream == nil {
					log.Printf("err:%v\r\n", err)
					return err
				}
				defer stream.Close()
				//to tcp
				stream.Write([]byte{0x02})
				var addrLen uint16 = uint16(len(remoteHost))
				err = binary.Write(stream, binary.BigEndian, addrLen)
				if err != nil {
					return nil
				}
				stream.Write([]byte(remoteHost))
				clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) //响应客户端连接成功
				socksTapComm.ConnPipe(stream, clientConn, time.Minute*3)
			}
		}
		//UDP  代理
		if connectHead[1] == 0x03 {
			socks.UdpProxyRes(clientConn, lSocks.udpProxy.udpListener.LocalAddr().(*net.UDPAddr))
		}
	}
	return nil
}
