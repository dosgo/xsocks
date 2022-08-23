package server

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"golang.org/x/time/rate"
	"gvisor.dev/gvisor/pkg/bufferv2"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"

	//"github.com/google/netstack/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"

	"net"
	"strings"
	"time"

	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
)

var remoteTunUdpNat sync.Map

var udpLimit sync.Map

func InjectInbound(channelLinkID *channel.Endpoint, buf []byte) error {
	if channelLinkID == nil {
		log.Println("channelLinkID nil")
		return errors.New("channelLinkID nil")
	}

	pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: bufferv2.MakeWithData(buf),
	})
	switch header.IPVersion(buf[:]) {
	case header.IPv4Version:
		channelLinkID.InjectInbound(header.IPv4ProtocolNumber, pkt)
	case header.IPv6Version:
		channelLinkID.InjectInbound(header.IPv6ProtocolNumber, pkt)
	}
	pkt.DecRef()
	return nil
}

/*start */
func StartTunStack(mtu uint16) (*stack.Stack, *channel.Endpoint, error) {
	go autoFree()
	return comm.NewDefaultStack(int(mtu), tcpForward, udpForward)
}

/*udp 转发*/
func udpForward(conn *gonet.UDPConn, ep tcpip.Endpoint) error {
	//defer ep.Close();
	var remoteAddr = ""
	var duration time.Duration = time.Second * 100
	//dns 8.8.8.8
	if strings.HasSuffix(conn.LocalAddr().String(), ":53") {
		fmt.Printf("udpForward dnsAddr:%s", conn.LocalAddr().String()+"localAddr:"+conn.RemoteAddr().String()+"SafeDns:"+param.Args.SafeDns+"\r\n")
		remoteAddr = param.Args.SafeDns + ":53"
		duration = time.Second * 15
	} else {
		remoteAddr = conn.LocalAddr().String()
	}
	//限流处理
	var limit *comm.UdpLimit
	_limit, ok := udpLimit.Load(remoteAddr)
	if !ok {
		limit = &comm.UdpLimit{Limit: rate.NewLimiter(rate.Every(1*time.Second), 60), Expired: time.Now().Unix() + 5}
	} else {
		limit = _limit.(*comm.UdpLimit)
	}
	//限流
	if limit.Limit.Allow() {
		limit.Expired = time.Now().Unix() + 5
		comm.TunNatSawp(&remoteTunUdpNat, conn, ep, remoteAddr, duration)
		udpLimit.Store(remoteAddr, limit)
	}
	return nil
}

func autoFree() {
	for {
		udpLimit.Range(func(k, v interface{}) bool {
			_v := v.(*comm.UdpLimit)
			if _v.Expired < time.Now().Unix() {
				udpLimit.Delete(k)
			}
			return true
		})
		time.Sleep(time.Second * 30)
	}
}

/*tcpForward*/
func tcpForward(conn *gonet.TCPConn) error {
	conn2, err := net.DialTimeout("tcp", conn.LocalAddr().String(), param.Args.ConnectTime)
	if err != nil {
		fmt.Println("tcpForward" + conn.LocalAddr().String() + err.Error())
		return err
	}
	comm.TcpPipe(conn, conn2, time.Minute*2)
	return nil
}
