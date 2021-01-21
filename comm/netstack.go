package comm

import (
	"fmt"
	"github.com/google/netstack/tcpip"
	"github.com/google/netstack/tcpip/adapters/gonet"
	"github.com/google/netstack/tcpip/link/channel"
	"github.com/google/netstack/tcpip/network/ipv4"
	"github.com/google/netstack/tcpip/stack"
	"github.com/google/netstack/tcpip/transport/tcp"
	"github.com/google/netstack/tcpip/transport/udp"
	"github.com/google/netstack/waiter"
	"errors"
	"net"
	"time"
)

type ForwarderCall func(conn *gonet.Conn) error
type UdpForwarderCall func(conn *gonet.Conn,ep tcpip.Endpoint) error
func GenChannelLinkID(mtu int,tcpCallback ForwarderCall,udpCallback UdpForwarderCall)(*channel.Endpoint,*stack.Stack, error){
	var nicID tcpip.NICID =1;
	macAddr, err := net.ParseMAC("de:ad:be:ee:ee:ef")
	if err != nil {
		fmt.Printf(err.Error());
		return nil,nil,err
	}
	//[]string{ipv4.ProtocolName, ipv6.ProtocolName, arp.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName},
	s := stack.New( stack.Options{NetworkProtocols:   []stack.NetworkProtocol{ipv4.NewProtocol()},
		TransportProtocols: []stack.TransportProtocol{tcp.NewProtocol(), udp.NewProtocol()}})
	//转发开关,必须
	s.SetForwarding(true);
	var linkID stack.LinkEndpoint
	var channelLinkID= channel.New(256, uint32(mtu),   tcpip.LinkAddress(macAddr))
	linkID=channelLinkID;
	if err := s.CreateNIC(nicID, linkID); err != nil {
		return nil,nil,errors.New(err.String())
	}
	//promiscuous mode 必须
	s.SetPromiscuousMode(nicID, true)
	tcpForwarder := tcp.NewForwarder(s, 0, 256, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			fmt.Printf(err.String());
			r.Complete(true)
			return
		}
		defer ep.Close();
		r.Complete(false)
		if err := setKeepalive(ep); err != nil {
			fmt.Printf(err.Error());
		}
		conn:=gonet.NewConn(&wq, ep)
		defer conn.Close();
		tcpCallback(conn);
	})
	s.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)
	udpForwarder := udp.NewForwarder(s, func(r *udp.ForwarderRequest) {
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			fmt.Printf("r.CreateEndpoint() = %v", err)
		}
		go udpCallback(gonet.NewConn(&wq, ep),ep);
	})
	s.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)

	return channelLinkID,s,nil
}


func setKeepalive(ep tcpip.Endpoint) error {
	//if err := ep.SetSockOptBool(tcpip.KeepaliveEnabledOption, true); err != nil {
	//	return fmt.Errorf("set keepalive: %s", err)
	//}
	idleOpt := tcpip.KeepaliveIdleOption(60 * time.Second)
	if err := ep.SetSockOpt(&idleOpt); err != nil {
		return fmt.Errorf("set keepalive idle: %s", err)
	}
	intervalOpt := tcpip.KeepaliveIntervalOption( 30 * time.Second)
	if err := ep.SetSockOpt(&intervalOpt); err != nil {
		return fmt.Errorf("set keepalive interval: %s", err)
	}
	return nil
}