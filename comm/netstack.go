package comm

import (
	"errors"
	"fmt"
	"github.com/google/netstack/tcpip"
	"github.com/google/netstack/tcpip/adapters/gonet"
	"github.com/google/netstack/tcpip/link/channel"
	"github.com/google/netstack/tcpip/network/ipv4"
	"github.com/google/netstack/tcpip/stack"
	"github.com/google/netstack/tcpip/transport/tcp"
	"github.com/google/netstack/tcpip/transport/udp"
	"github.com/google/netstack/waiter"
	"net"
	"time"
)

type ForwarderCall func(conn *gonet.Conn) error
type UdpForwarderCall func(conn *gonet.Conn,ep tcpip.Endpoint) error

func GenChannelLinkID(mtu int,tcpCallback ForwarderCall,udpCallback UdpForwarderCall)(*stack.Stack,*channel.Endpoint, error){
	_netStack := stack.New( stack.Options{NetworkProtocols:   []stack.NetworkProtocol{ipv4.NewProtocol()},
		TransportProtocols: []stack.TransportProtocol{tcp.NewProtocol(), udp.NewProtocol()}})
	//转发开关,必须
	_netStack.SetForwarding(true);
	var nicid tcpip.NICID =1;
	macAddr, err := net.ParseMAC("de:ad:be:ee:ee:ef")
	if err != nil {
		fmt.Printf(err.Error());
		_netStack.Close();
		return _netStack,nil,err
	}

	var linkID stack.LinkEndpoint
	var channelLinkID= channel.New(1024, uint32(mtu),   tcpip.LinkAddress(macAddr))
	linkID=channelLinkID;
	if err := _netStack.CreateNIC(nicid, linkID); err != nil {
		_netStack.Close();
		return _netStack,nil,errors.New(err.String())
	}
	//promiscuous mode 必须
	_netStack.SetPromiscuousMode(nicid, true)
	_netStack.SetSpoofing(nicid, true);

	tcpForwarder := tcp.NewForwarder(_netStack, 0, 512, func(r *tcp.ForwarderRequest) {
		if(r==nil){
			return ;
		}
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			fmt.Printf("CreateEndpoint"+err.String()+"\r\n");
			r.Complete(true)
			return
		}
		defer ep.Close();
		r.Complete(false)
		if err := setKeepalive(ep); err != nil {
			fmt.Printf("setKeepalive"+err.Error()+"\r\n");
		}
		conn:=gonet.NewConn(&wq, ep)
		defer conn.Close();
		tcpCallback(conn);
	})
	_netStack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	udpForwarder := udp.NewForwarder( _netStack, func(r *udp.ForwarderRequest) {
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			fmt.Printf("r.CreateEndpoint() = %v", err)
		}
		go udpCallback(gonet.NewConn(&wq, ep),ep);
	})
	_netStack.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)

	return _netStack,channelLinkID,nil
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