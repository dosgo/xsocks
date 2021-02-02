package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
	"xSocks/comm"
	"xSocks/param"
)

var udpNat sync.Map

func StartRemoteSocks51(address string) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Panic(err)
	}
	//start udpProxy
	udpAddr,err:=startUdpProxy();
	if err != nil {
		log.Panic(err)
	}

	for {
		client, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}
		go handleRemoteRequest(client,udpAddr)
	}
}

func startUdpProxy() ( *net.UDPAddr ,error){
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return nil,err
	}
	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil,err
	}
	buf := make([]byte, 65535)

	go func() {
		for {
			n, localAddr, err := udpListener.ReadFromUDP(buf[0:])
			if err != nil {
				break;
			}
			data := buf[0:n]
			dstAddr,dataStart,err:=UdpHeadDecode(data);
			if(err!=nil||dstAddr==nil){
				continue;
			}
			natKey:=localAddr.String()+"_"+dstAddr.String()
			var remoteConn net.Conn
			_conn,ok:=udpNat.Load(natKey)
			if !ok{
				remoteConn, err = net.Dial("udp", dstAddr.String());
				if err != nil {
					break;
				}
				natSawp(udpListener,natKey,data[:dataStart],localAddr,remoteConn);
			}else{
				remoteConn=_conn.(net.Conn)
			}
			remoteConn.Write(data[dataStart:])
		}
	}()
	return udpAddr,nil;
}

/*udp nat sawp*/
func natSawp(udpGate *net.UDPConn,natKey string,udpHead []byte,localAddr *net.UDPAddr,  remoteConn net.Conn){
	buf:= make([]byte, 65535)
	var buffer bytes.Buffer
	udpNat.Store(natKey,remoteConn)
	defer udpNat.Delete(natKey);
	defer remoteConn.Close()
	go func() {
		for {
			//remoteConn.SetDeadline();
			n, err:= remoteConn.Read(buf);
			if(err!=nil){
				return ;
			}
			buffer.Reset();
			buffer.Write(udpHead)
			buffer.Write(buf[:n])
			udpGate.WriteToUDP(buffer.Bytes(), localAddr)
		}
	}()
}


func udpProxyRes(clientConn net.Conn,udpAddr *net.UDPAddr )  error{
	//
	temp := make([]byte, 6)
	_, err := io.ReadFull(clientConn, temp)
	if(err!=nil){
		return err;
	}
	bindPort := udpAddr.Port
	//版本 | 代理的应答 |　保留1字节　| 地址类型 | 代理服务器地址 | 绑定的代理端口
	bindMsg := []byte{0x05, 0x00, 0x00, 0x01}
	buffer := bytes.NewBuffer(bindMsg)
	binary.Write(buffer, binary.BigEndian, udpAddr.IP.To4())
	binary.Write(buffer, binary.BigEndian, uint16(bindPort))
	clientConn.Write(buffer.Bytes())
	return nil;
}


/*remote use*/
func handleRemoteRequest(clientConn net.Conn,udpAddr *net.UDPAddr) {
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
				hostLen := make([]byte, 1)
				_, err = io.ReadFull(clientConn, hostLen)
				hostBuf := make([]byte, hostLen[0])
				_, err = io.ReadFull(clientConn, hostBuf)
				host = string(hostBuf) //b[4]表示域名的长度
				fmt.Printf("host:%s\r\n",hostBuf);
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
			server, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), param.ConnectTime)
			if err != nil {
				log.Println(err)
				return
			}
			defer server.Close()
			clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) //响应客户端连接成功
			fmt.Printf("to remote\r\n")
			//进行转发
			comm.TcpPipe(server,clientConn,time.Minute*10)
		}
		//udp
		if(connectHead[1]==0x03) {
			udpProxyRes(clientConn,udpAddr);
		}
	}
}

/*test single user*/
func startUdpGate() ( *net.UDPAddr ,error){
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return nil,err
	}
	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil,err
	}
	buf := make([]byte, 65535)
	var gateNat sync.Map
	var buffer bytes.Buffer
	go func() {
		for {
			n, recvAddr, err := udpListener.ReadFromUDP(buf[0:])
			if err != nil {
				break;
			}
			data := buf[0:n]
			_udpAddr,ok:=gateNat.Load(recvAddr.String())
			//client to remote
			if !ok{
				var dstAddr *net.UDPAddr
				dstAddr,dataStart,err:=UdpHeadDecode(data);
				if(err!=nil||dstAddr==nil){
					continue;
				}
				gateNat.Store(dstAddr.String(),recvAddr)
				udpListener.WriteTo(data[dataStart:],dstAddr)
			}else{
				buffer.Reset()
				buffer.Write(UdpHeadEncode(recvAddr))
				buffer.Write(data)
				//remote to client
				udpListener.WriteTo(buffer.Bytes(),_udpAddr.(*net.UDPAddr))
			}
		}
	}()
	return udpAddr,nil;
}

func UdpHeadDecode(data []byte) ( *net.UDPAddr,int, error){

	/*
	   +----+------+------+----------+----------+----------+
	   |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
	   +----+------+------+----------+----------+----------+
	   |  2 |   1  |   1  | Variable |     2    | Variable |
	   +----+------+------+----------+----------+----------+
	*/
	if data[2] != 0x00 {
		return nil,0,errors.New("WARN: FRAG do not support");
	}

	var dstAddr *net.UDPAddr
	var dataStart=0;
	switch data[3] {
	case 0x01: //ipv4
		dstAddr = &net.UDPAddr{
			IP:   net.IPv4(data[4], data[5], data[6], data[7]),
			Port: int(data[8])*256 + int(data[9]),
		}
		dataStart=10;
		break;
	case 0x03: //domain
		domainLen := int(data[4])
		domain := string(data[5 : 5+domainLen])
		ipAddr, err := net.ResolveIPAddr("ip", domain)
		if err != nil {
			return nil,0,errors.New(fmt.Sprintf("Error -> domain %s dns query err:%v\n", domain, err));
		}
		dstAddr = &net.UDPAddr{
			IP:   ipAddr.IP,
			Port: int(data[5+domainLen])*256 + int(data[6+domainLen]),
		}
		dataStart=6+domainLen;
		break;
	default:
		return nil,0,errors.New(fmt.Sprintf( " WARN: ATYP %v do not support.\n", data[3]));

	}
	return dstAddr,dataStart,nil;
}
func UdpHeadEncode(addr *net.UDPAddr) (  []byte) {
	bindMsg := []byte{0x05, 0x00, 0x00, 0x01}
	buffer := bytes.NewBuffer(bindMsg)
	binary.Write(buffer, binary.BigEndian, addr.IP.To4())
	binary.Write(buffer, binary.BigEndian, uint16(addr.Port))
	return buffer.Bytes();
}

