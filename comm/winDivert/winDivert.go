//go:build windows
// +build windows

package winDivert

import (
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/imgk/divert-go"
)

var winDivert *divert.Handle
var winDivertRun = false

var divertDll = "WinDivert.dll"
var divertSys = "WinDivert32.sys"

func dllInit(_divertDll string) {
	_, err := os.Stat(_divertDll)
	if err != nil {
		fmt.Printf("not found :%s\r\n", _divertDll)
	}
}

/*windows转发*/
func RedirectDNS(dnsAddr string, _port string, sendPort string) error {
	if _port == "53" {
		return errors.New("_port error ")
	}

	var err error
	_, err = os.Stat(divertDll)
	if err != nil {
		fmt.Printf("not found :%s\r\n", divertDll)
		return err
	}
	winDivertRun = true
	winDivert, err = divert.Open("outbound and !loopback and !impostor and udp.DstPort=53 and udp.SrcPort!="+sendPort, divert.LayerNetwork, divert.PriorityDefault, divert.FlagDefault)
	if err != nil {
		fmt.Printf("winDivert open failed: %v\r\n", err)
		return err
	}

	rawbuf := make([]byte, 1500)
	var dnsRecvBuf []byte = make([]byte, 1500)
	var recvBuf []byte = make([]byte, 1500)
	addr := divert.Address{}
	dnsConn, _ := net.DialTimeout("udp", dnsAddr+":"+_port, time.Second*15)
	defer dnsConn.Close()
	for winDivertRun {
		if winDivert == nil {
			continue
		}
		recvLen, err := winDivert.Recv(recvBuf, &addr)
		if err != nil {
			log.Println(1, err)
			continue
		}
		ipv6 := recvBuf[0]>>4 == 6
		var ipheadlen int
		if ipv6 {
			ipheadlen = 40
		} else {
			ipheadlen = int(recvBuf[0]&0xF) * 4
		}
		udpheadlen := 8
		request := recvBuf[ipheadlen+udpheadlen : recvLen]
		dnsConn.Write(request)
		n, err := dnsConn.Read(dnsRecvBuf)
		if err == nil {
			var response = dnsRecvBuf[:n]
			udpsize := len(response) + udpheadlen
			var packetsize = ipheadlen + udpsize
			if ipv6 {
				//copy ip head
				copy(rawbuf, recvBuf[:8])
				binary.BigEndian.PutUint16(rawbuf[4:], uint16(udpsize))
				copy(rawbuf[8:], recvBuf[24:40])
				copy(rawbuf[24:], recvBuf[8:24])
			} else {
				//copy ip head
				copy(rawbuf, recvBuf[:12])
				//write size
				binary.BigEndian.PutUint16(rawbuf[2:], uint16(packetsize))
				//swap  src addr dest addr
				copy(rawbuf[12:], recvBuf[16:20])
				copy(rawbuf[16:], recvBuf[12:16])
			}

			//swap  src port dest port
			copy(rawbuf[ipheadlen:], recvBuf[ipheadlen+2:ipheadlen+4])
			copy(rawbuf[ipheadlen+2:], recvBuf[ipheadlen:ipheadlen+2])

			//write udp size
			binary.BigEndian.PutUint16(rawbuf[ipheadlen+4:], uint16(udpsize))
			copy(rawbuf[ipheadlen+udpheadlen:], response)
			packet := rawbuf[:packetsize]
			divert.CalcChecksums(packet, &addr, 0)
			_, err = winDivert.Send(packet, &addr)
			if err != nil {
				log.Println(1, err)
			}
		}
	}
	return nil
}

func RedirectDNSv1(dnsAddr string, _port string, sendPort string) error {
	//error
	if dnsAddr == "127.0.0.1" || _port == "53" {
		return errors.New("_port error ")
	}
	var err error
	_, err = os.Stat(divertDll)
	if err != nil {
		fmt.Printf("not found :%s\r\n", divertDll)
		return err
	}
	winDivertRun = true
	winDivert, err = divert.Open("outbound and !loopback and !impostor and udp.DstPort=53 and udp.SrcPort!="+sendPort, divert.LayerNetwork, divert.PriorityDefault, divert.FlagDefault)
	if err != nil {
		fmt.Printf("winDivert open failed: %v\r\n", err)
		return err
	}
	var recvBuf []byte = make([]byte, 1500)
	addr := divert.Address{}
	dstIp := net.ParseIP(dnsAddr)
	for winDivertRun {
		if winDivert == nil {
			continue
		}
		recvLen, err := winDivert.Recv(recvBuf, &addr)
		if err != nil {
			log.Println(1, err)
			continue
		}
		ipv6 := recvBuf[0]>>4 == 6
		var ipheadlen int
		if ipv6 {
			ipheadlen = 40
			//dst ip
			copy(recvBuf[24:40], dstIp)
		} else {
			ipheadlen = int(recvBuf[0]&0xF) * 4
			//dst ip
			copy(recvBuf[16:20], dstIp[12:16])
		}
		//dst port
		port, _ := strconv.ParseUint(_port, 10, 16)
		recvBuf[ipheadlen+2] = uint8(port >> 8)
		recvBuf[ipheadlen+3] = uint8(port & 0xff)
		packet := recvBuf[:recvLen]
		divert.CalcChecksums(packet, &addr, 0)
		_, err = winDivert.Send(packet, &addr)
		if err != nil {
			log.Println(1, err)
		}
	}
	return nil
}

func CloseWinDivert() {
	winDivertRun = false
	if winDivert != nil {
		winDivert.Close()
	}
}
