package comm

import (
	"bytes"
	"encoding/binary"
	"net"
	"errors"
)

/*
nat src dst
*/
func UdpNatDecode(data []byte) (  *net.UDPAddr, *net.UDPAddr, error){
	if(len(data)<12){
		return nil,nil,	errors.New("data len <12")
	}
	var src *net.UDPAddr
	var dst *net.UDPAddr
	src = &net.UDPAddr{
		IP:   net.IPv4(data[0], data[1], data[2], data[3]),
		Port: int(data[4])*256 + int(data[5]),
	}
	dst = &net.UDPAddr{
		IP:   net.IPv4(data[6], data[7], data[8], data[9]),
		Port: int(data[10])*256 + int(data[11]),
	}
	return src,dst,nil;
}

/*
nat src dst
*/
func UdpNatEncode(src *net.UDPAddr,dst *net.UDPAddr) ([]byte) {
	buffer := bytes.NewBuffer( []byte{})
	binary.Write(buffer, binary.BigEndian, src.IP.To4())
	binary.Write(buffer, binary.BigEndian, uint16(src.Port))
	binary.Write(buffer, binary.BigEndian, dst.IP.To4())
	binary.Write(buffer, binary.BigEndian, uint16(dst.Port))
	return buffer.Bytes();
}
