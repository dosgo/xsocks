package comm

import (
	"errors"
	"fmt"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
	"log"

	//"github.com/google/netstack/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	//"github.com/google/netstack/tcpip/buffer"
	"net"
	"time"
)


const (
	tcpCongestionControlAlgorithm = "cubic" // "reno" or "cubic"
)

type ForwarderCall func(conn *gonet.TCPConn) error
type UdpForwarderCall func(conn *gonet.UDPConn,ep tcpip.Endpoint) error





func NewDefaultStack(mtu int,tcpCallback ForwarderCall,udpCallback UdpForwarderCall) (*stack.Stack,*channel.Endpoint, error) {

	_netStack := stack.New( stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol}})

	//转发开关,必须
	//_netStack.SetForwarding(ipv4.ProtocolNumber,true);
	_netStack.SetForwardingDefaultAndAllNICs(ipv4.ProtocolNumber, true);
	//_netStack.SetForwardingDefaultAndAllNICs(ipv6.ProtocolNumber, true);
	var nicid tcpip.NICID =1;
	macAddr, err := net.ParseMAC("de:ad:be:ee:ee:ef")
	if err != nil {
		fmt.Printf(err.Error());
		return _netStack,nil,err
	}

	opt1 := tcpip.CongestionControlOption(tcpCongestionControlAlgorithm)
	if err := _netStack.SetTransportProtocolOption(tcp.ProtocolNumber, &opt1); err != nil {
		return nil,nil,fmt.Errorf("set TCP congestion control algorithm: %s", err)
	}

	var linkID stack.LinkEndpoint
	var channelLinkID= channel.New(1024, uint32(mtu),   tcpip.LinkAddress(macAddr))
	linkID=channelLinkID;
	if err := _netStack.CreateNIC(nicid, linkID); err != nil {
		return _netStack,nil,errors.New(err.String())
	}
	_netStack.SetRouteTable([]tcpip.Route{
		// IPv4
		{
			Destination: header.IPv4EmptySubnet,
			NIC:         nicid,
		},
	})
	//promiscuous mode 必须
	_netStack.SetPromiscuousMode(nicid, true)
	_netStack.SetSpoofing(nicid, true);

	tcpForwarder := tcp.NewForwarder(_netStack, 0, 512, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			log.Printf("CreateEndpoint"+err.String()+"\r\n");
			r.Complete(true)
			return
		}
		defer ep.Close();
		r.Complete(false)
		if err := setKeepalive(ep); err != nil {
			log.Printf("setKeepalive"+err.Error()+"\r\n");
		}
		conn:=gonet.NewTCPConn(&wq, ep)
		defer conn.Close();
		tcpCallback(conn);
	})
	_netStack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	udpForwarder := udp.NewForwarder( _netStack, func(r *udp.ForwarderRequest) {
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			log.Printf("r.CreateEndpoint() = %v", err)
			return
		}
		go udpCallback(gonet.NewUDPConn(_netStack,&wq, ep),ep);
	})
	_netStack.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)

	return _netStack,channelLinkID,nil
}




func setKeepalive(ep tcpip.Endpoint) error {
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