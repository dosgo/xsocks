package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)


func StartRemoteSocks51(address string) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Panic(err)
	}
	for {
		client, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}
		go handleRemoteRequest(client)
	}
}


/*remote use*/
func handleRemoteRequest(clientConn net.Conn) {
	if clientConn == nil {
		return
	}
	defer clientConn.Close()
	auth:= make([]byte,3)
	_, err := io.ReadFull(clientConn, auth)
	if err != nil {
		return
	}
	if(auth[0]==0x05){
		//resp auth
		clientConn.Write([]byte{0x05, 0x00})
	}


	connectHead:= make([]byte,4)
	_, err = io.ReadFull(clientConn, connectHead)
	if err != nil {
		return
	}

	if(connectHead[0]==0x05) {

		if(connectHead[1]==0x01) {
			var host, port string
			switch connectHead[3] {
			case 0x01: //IP V4
				fmt.Printf("ipv4\r\n");
				ipv4 := make([]byte, 4)
				_, err = io.ReadFull(clientConn, ipv4)
				host = net.IPv4(ipv4[0], ipv4[1], ipv4[2], ipv4[3]).String()
				break;
			case 0x03: //域名
				fmt.Printf("host\r\n");
				hostLen := make([]byte, 1)
				_, err = io.ReadFull(clientConn, hostLen)
				hostBuf := make([]byte, hostLen[0])
				_, err = io.ReadFull(clientConn, hostBuf)
				host = string(hostBuf) //b[4]表示域名的长度
				break;
			case 0x04: //IP V6
				fmt.Printf("ipv6\r\n");
				ipv6 := make([]byte, 16)
				_, err = io.ReadFull(clientConn, ipv6)
				host = net.IP{ipv6[0], ipv6[1], ipv6[2], ipv6[3], ipv6[4], ipv6[5], ipv6[6], ipv6[7], ipv6[8], ipv6[9], ipv6[10], ipv6[11], ipv6[12], ipv6[13], ipv6[14], ipv6[15]}.String()
				break;
			}
			portBuf := make([]byte, 2)
			_, err = io.ReadFull(clientConn, portBuf)
			port = strconv.Itoa(int(portBuf[0])<<8 | int(portBuf[1]))
			server, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)
			if err != nil {
				log.Println(err)
				return
			}
			defer server.Close()
			clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) //响应客户端连接成功
			fmt.Printf("to remote\r\n")
			//进行转发
			go io.Copy(server, clientConn)
			io.Copy(clientConn, server)
			//CopyLoopTimeout();
		}
		//udp
		if(connectHead[1]==0x03) {
			temp := make([]byte, 6)
			_, err = io.ReadFull(clientConn, temp)
			remoteUdp(clientConn);
		}
	}
}

/*udp to udp*/
func remoteUdp(client net.Conn)  error{


	clientInfo:=client.RemoteAddr().String()
	clientIp:=strings.Split(clientInfo,":")[0];

	udpAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	if err != nil {
		return err
	}
	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	defer udpListener.Close()

	udpConn, err := net.ResolveUDPAddr("udp", udpListener.LocalAddr().String())
	bindPort := udpConn.Port

	//版本 | 代理的应答 |　保留1字节　| 地址类型 | 代理服务器地址 | 绑定的代理端口
	bindMsg := []byte{0x05, 0x00, 0x00, 0x01}
	buffer := bytes.NewBuffer(bindMsg)
	binary.Write(buffer, binary.BigEndian, udpAddr.IP.To4())
	binary.Write(buffer, binary.BigEndian, uint16(bindPort))
	client.Write(buffer.Bytes())

	buf := make([]byte, 2048)
	for {
		n, udpAddr, err := udpListener.ReadFromUDP(buf[0:])
		if err != nil {
			return err
		}


		b := buf[0:n]
		if udpAddr.IP.String() == clientIp|| udpAddr.IP.String()=="127.0.0.1" { // from client
			/*
			   +----+------+------+----------+----------+----------+
			   |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
			   +----+------+------+----------+----------+----------+
			   |  2 |   1  |   1  | Variable |     2    | Variable |
			   +----+------+------+----------+----------+----------+
			*/
			if b[2] != 0x00 {
				log.Printf(" WARN: FRAG do not support.\n")
				continue
			}

			switch b[3] {
			case 0x01: //ipv4
				dstAddr := &net.UDPAddr{
					IP:   net.IPv4(b[4], b[5], b[6], b[7]),
					Port: int(b[8])*256 + int(b[9]),
				}
				udpListener.WriteToUDP(b[10:], dstAddr)
			case 0x03: //domain
				domainLens := int(b[4])
				domain := string(b[5 : 5+domainLens])
				ipAddr, err := net.ResolveIPAddr("ip", domain)
				if err != nil {
					log.Printf("Error -> domain %s dns query err:%v\n", domain, err)
					continue
				}
				dstAddr := &net.UDPAddr{
					IP:   ipAddr.IP,
					Port: int(b[5+domainLens])*256 + int(b[6+domainLens]),
				}
				udpListener.WriteToUDP(b[7+domainLens:], dstAddr)
			default:
				log.Printf(" WARN: ATYP %v do not support.\n", b[3])
				continue
			}
		} else { // from dst Server
			/*
			uS.RLock()
			if v, exist := uS.dstMap[udpAddr.String()]; exist {
				uS.RUnlock()
				head := []byte(v)
				headLens := len(head)
				copy(buf2[0:], head[0:headLens])
				copy(buf2[headLens:], b[0:])
				sendData := buf2[0 : headLens+n]
				udpConn.WriteToUDP(sendData, uS.clientUdpAddr)
				//log.Printf("%s <<< head-> %v\n", uS.prefix, head)
				//log.Printf("%s <<< b-> %v\n", uS.prefix, b)
				//log.Printf("%s <<< data-> %v\n", uS.prefix, sendData)
			} else {
				fmt.Printf("%s WARN -> %s not in dstMap.\n", uS.prefix, udpAddr.String())
				uS.RUnlock()
				continue
			}

			 */
		}
	}



}


type timeoutConn struct {
	c net.Conn
	t time.Duration
}

func (tc timeoutConn) Read(buf []byte) (int, error) {
	tc.c.SetDeadline(time.Now().Add(tc.t))
	return tc.c.Read(buf)
}

func (tc timeoutConn) Write(buf []byte) (int, error) {
	tc.c.SetDeadline(time.Now().Add(tc.t))
	return tc.c.Write(buf)
}

func (tc timeoutConn) Close() {
	tc.c.Close()
}

func CopyLoopTimeout(c1 net.Conn, c2 net.Conn, timeout time.Duration) {
	tc1 := timeoutConn{c: c1, t: timeout}
	tc2 := timeoutConn{c: c2, t: timeout}
	var wg sync.WaitGroup
	copyer := func(dst timeoutConn, src timeoutConn) {
		defer wg.Done()
		_, e := io.Copy(dst, src)
		dst.Close()
		if e != nil {
			src.Close()
		}
	}
	wg.Add(2)
	go copyer(tc1, tc2)
	go copyer(tc2, tc1)
	wg.Wait()
}