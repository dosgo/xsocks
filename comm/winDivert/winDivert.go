// +build windows

package winDivert

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"github.com/macronut/godivert"
	"log"
	"net"
	"os"
	"time"
)

var winDivert *godivert.WinDivertHandle;
var winDivertRun=false;


var divertDll="WinDivert32.dll";
var divertSys="WinDivert32.sys";



func dllInit() {
	_,err:=os.Stat(divertDll)
	if err==nil {
		godivert.LoadDLL("WinDivert64.dll", "WinDivert32.dll")
	}else{
		fmt.Printf("not found WinDivert.dll WinDivert32.dll\r\n")
	}
}


/*windows转发*/
func RedirectDNS(dnsAddr string,_port string,sendPort string) {
	var err error;
	_,err=os.Stat(divertDll)
	if err!=nil {
		fmt.Printf("not found WinDivert.dll WinDivert32.dll\r\n")
		return
	}
	winDivertRun=true;
	winDivert, err = godivert.WinDivertOpen("outbound and !loopback and !impostor and udp.DstPort=53 and udp.SrcPort!="+sendPort, 0, 0, 0)
	if err != nil {
		fmt.Printf("winDivert open failed: %v\r\n", err)
		return
	}

	rawbuf := make([]byte, 1500)
	var bufByte1 []byte = make([]byte,1500)
	dnsConn, _ := net.DialTimeout("udp", dnsAddr+":"+_port,time.Second*15);
	defer dnsConn.Close()
	for winDivertRun {
		if winDivert==nil {
			continue;
		}
		packet, err := winDivert.Recv()
		if err != nil  {
			log.Println(1, err)
			continue
		}
		ipv6 := packet.Raw[0]>>4 == 6
		var ipheadlen int
		if ipv6 {
			ipheadlen = 40
		} else {
			ipheadlen = int(packet.Raw[0]&0xF) * 4
		}
		udpheadlen := 8
		request := packet.Raw[ipheadlen+udpheadlen:]
		dnsConn.Write(request);
		n, err := dnsConn.Read(bufByte1)
		if err==nil {
			var response =bufByte1[:n]
			udpsize := len(response) + 8
			var packetsize int
			if ipv6 {
				copy(rawbuf, []byte{96, 12, 19, 68, 0, 98, 17, 128})
				packetsize = 40 + udpsize
				binary.BigEndian.PutUint16(rawbuf[4:], uint16(udpsize))
				copy(rawbuf[8:], packet.Raw[24:40])
				copy(rawbuf[24:], packet.Raw[8:24])
				copy(rawbuf[ipheadlen:], packet.Raw[ipheadlen+2:ipheadlen+4])
				copy(rawbuf[ipheadlen+2:], packet.Raw[ipheadlen:ipheadlen+2])
			} else {
				copy(rawbuf, []byte{69, 0, 1, 32, 141, 152, 64, 0, 64, 17, 150, 46})
				packetsize = 20 + udpsize
				binary.BigEndian.PutUint16(rawbuf[2:], uint16(packetsize))
				copy(rawbuf[12:], packet.Raw[16:20])
				copy(rawbuf[16:], packet.Raw[12:16])
				copy(rawbuf[20:], packet.Raw[ipheadlen+2:ipheadlen+4])
				copy(rawbuf[22:], packet.Raw[ipheadlen:ipheadlen+2])
				ipheadlen = 20
			}

			binary.BigEndian.PutUint16(rawbuf[ipheadlen+4:], uint16(udpsize))
			copy(rawbuf[ipheadlen+8:], response)
			packet.PacketLen = uint(packetsize)
			packet.Raw = rawbuf[:packetsize]
			packet.Addr.Data |= 0x1
			packet.CalcNewChecksum(winDivert)
			_, err = winDivert.Send(packet)
			if err != nil {
				log.Println(1, err)
				return
			}
		}
	}
}

func CloseWinDivert(){
	winDivertRun=false;
	if winDivert!=nil {
		winDivert.Close()
	}
}