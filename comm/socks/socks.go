package socks

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
)
import "fmt"
import "errors"





/*socks 5 udp header*/
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


/* udp req res*/
func UdpProxyRes(clientConn net.Conn,udpAddr *net.UDPAddr)  error{
	fmt.Printf("req Udp addr:%v \r\n",udpAddr.String())
	/*
		|VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
		| 1  |  1  | X'00' |  1   | Variable |    2     |
	*/
	temp := make([]byte, 6)
	_, err := io.ReadFull(clientConn, temp)
	if err!=nil {
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