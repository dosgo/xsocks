//go:build windows
// +build windows

package winDivert

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
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
func RedirectDNS(dnsAddr string, _port string, sendPort string) {
	var err error
	_, err = os.Stat(divertDll)
	if err != nil {
		fmt.Printf("not found :%s\r\n", divertDll)
		return
	}
	winDivertRun = true
	winDivert, err = divert.Open("outbound and !loopback and !impostor and udp.DstPort=53 and udp.SrcPort!="+sendPort, divert.LayerNetwork, divert.PriorityDefault, divert.FlagDefault)
	if err != nil {
		fmt.Printf("winDivert open failed: %v\r\n", err)
		return
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
			udpsize := len(response) + 8
			var packetsize int
			if ipv6 {
				copy(rawbuf, []byte{96, 12, 19, 68, 0, 98, 17, 128})
				packetsize = 40 + udpsize
				binary.BigEndian.PutUint16(rawbuf[4:], uint16(udpsize))
				copy(rawbuf[8:], recvBuf[24:40])
				copy(rawbuf[24:], recvBuf[8:24])
				copy(rawbuf[ipheadlen:], recvBuf[ipheadlen+2:ipheadlen+4])
				copy(rawbuf[ipheadlen+2:], recvBuf[ipheadlen:ipheadlen+2])
			} else {
				copy(rawbuf, []byte{69, 0, 1, 32, 141, 152, 64, 0, 64, 17, 150, 46})
				packetsize = 20 + udpsize
				binary.BigEndian.PutUint16(rawbuf[2:], uint16(packetsize))
				copy(rawbuf[12:], recvBuf[16:20])
				copy(rawbuf[16:], recvBuf[12:16])
				copy(rawbuf[20:], recvBuf[ipheadlen+2:ipheadlen+4])
				copy(rawbuf[22:], recvBuf[ipheadlen:ipheadlen+2])
				ipheadlen = 20
			}

			binary.BigEndian.PutUint16(rawbuf[ipheadlen+4:], uint16(udpsize))
			copy(rawbuf[ipheadlen+8:], response)
			packet := rawbuf[:packetsize]
			divert.CalcChecksums(packet, &addr, 0)
			_, err = winDivert.Send(packet, &addr)
			if err != nil {
				log.Println(1, err)
				return
			}
		}
	}
}

func CloseWinDivert() {
	winDivertRun = false
	if winDivert != nil {
		winDivert.Close()
	}
}
