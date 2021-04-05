package tun2socks

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
	"github.com/dosgo/xSocks/comm"
	"github.com/dosgo/xSocks/comm/socks"
	"github.com/dosgo/xSocks/param"
)

func ForwardTransportFromIo(dev io.ReadWriteCloser,mtu int,tcpCallback comm.ForwarderCall,udpCallback comm.UdpForwarderCall) error {
	_,channelLinkID,err:=comm.NewDefaultStack(mtu,tcpCallback,udpCallback);
	if err!=nil {
		log.Printf("err:%v",err)
		return err;
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// write tun
	go func(_ctx context.Context) {
		var sendBuffer =new(bytes.Buffer)
		for {
			info,res:=channelLinkID.ReadContext(_ctx)
			if !res {
				log.Printf("channelLinkID exit \r\n")
				break;
			}
			sendBuffer.Reset()
			//buffer.Write(pkt.Pkt.LinkHeader().View())
			sendBuffer.Write(info.Pkt.NetworkHeader().View())
			sendBuffer.Write(info.Pkt.TransportHeader().View())
			sendBuffer.Write(info.Pkt.Data.ToView())
			if sendBuffer.Len()>0 {
				dev.Write(sendBuffer.Bytes())
			}
		}
	}(ctx)


	// read tun data
	var buf=make([]byte,mtu+80)
	for {
		n, e := dev.Read(buf[:])
		if e != nil {
			log.Printf("err:%v",err)
			break;
		}
		tmpView:=buffer.NewVectorisedView(n,[]buffer.View{
			buffer.NewViewFromBytes(buf[:n]),
		})
		channelLinkID.InjectInbound(header.IPv4ProtocolNumber, stack.NewPacketBuffer(stack.PacketBufferOptions{
			Data: tmpView,
		}))
	}
	return nil
}

/*to socks5*/
func SocksCmd(socksConn net.Conn,cmd uint8,host string) error{
	//socks5 auth
	socksConn.Write([]byte{0x05, 0x01,0x00});
	//connect head
	hosts:=strings.Split(host,":");
	rAddr:=net.ParseIP(hosts[0])
	_port, _ := strconv.Atoi(hosts[1])
	msg := []byte{0x05, cmd, 0x00, 0x01}
	buffer := bytes.NewBuffer(msg)
	//ip
	binary.Write(buffer, binary.BigEndian, rAddr.To4())
	//port
	binary.Write(buffer, binary.BigEndian, uint16(_port))
	socksConn.Write(buffer.Bytes());

	//recv auth back
	authBack := make([]byte, 2)
	_, err:= io.ReadFull(socksConn, authBack)
	if err != nil {
		log.Println(err)
		return err
	}
	if authBack[0]!=0x05||authBack[1]!=0x00 {
		log.Println("auth error")
		return errors.New("auth error");
	}

	//recv connectBack
	connectBack := make([]byte, 10)
	_, err = io.ReadFull(socksConn, connectBack)
	if err!= nil {
		log.Println(err)
		return err
	}
	return nil;
}

/*socks5  udp gate 这里必须保持socks5兼容 */
func SocksUdpGate(conn *gonet.UDPConn,dstAddr *net.UDPAddr) error{
	gateConn, err := net.DialTimeout("udp", "127.0.0.1:"+param.Args.Sock5UdpPort,time.Second*15);
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	defer conn.Close()
	defer gateConn.Close()

	go func() {
		var buffer bytes.Buffer
		var b1=make([]byte,65535);
		for {
			conn.SetReadDeadline(time.Now().Add(3*time.Minute))
			n,err:=conn.Read(b1);
			if err != nil {
				return ;
			}
			buffer.Reset()
			buffer.Write(socks.UdpHeadEncode(dstAddr))
			buffer.Write(b1[:n])
			_, _ = gateConn.Write(buffer.Bytes())
		}
	}()
	for {
		var b2=make([]byte,65535);
		gateConn.SetReadDeadline(time.Now().Add(3*time.Minute))
		n,err:=gateConn.Read(b2);
		if err != nil {
			return err;
		}
		_,dataStart,err:= socks.UdpHeadDecode(b2[:n])
		if err != nil {
			return nil;
		}
		_, _ = conn.Write(b2[dataStart:n])
	}
}


