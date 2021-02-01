package comm

import (
	"errors"
	"fmt"
	"golang.org/x/time/rate"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"

	//"github.com/google/netstack/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	//"github.com/google/netstack/tcpip/buffer"
	"net"
	"time"
)


const (
	// maxBufferSize is the maximum permitted size of a send/receive buffer.
	maxBufferSize = 4 << 20 // 4 MiB

	// minBufferSize is the smallest size of a receive or send buffer.
	minBufferSize =4 << 10 // 4096 bytes.

	// defaultBufferSize is the default size of the send/recv buffer for
	// a transport endpoint.
	defaultBufferSize = 1 << 20 // 1MB

	// defaultTimeToLive specifies the default TTL used by stack.
	defaultTimeToLive uint8 = 64

	// icmpBurst is the default number of ICMP messages that can be sent in
	// a single burst.
	icmpBurst = 50

	// icmpLimit is the default maximum number of ICMP messages permitted
	// by this rate limiter.
	icmpLimit rate.Limit = 1000

	// ipForwardingEnabled is the value used by stack to enable packet
	// forwarding between NICs.
	ipForwardingEnabled = true

	// tcpCongestionControl is the congestion control algorithm used by
	// stack. ccReno is the default option in gVisor stack.
	tcpCongestionControlAlgorithm = "reno" // "reno" or "cubic"

	// tcpDelayEnabled is the value used by stack to enable or disable
	// tcp delay option. Disable Nagle's algorithm here by default.
	tcpDelayEnabled = false

	// tcpModerateReceiveBufferEnabled is the value used by stack to
	// enable or disable tcp receive buffer auto-tuning option.
	tcpModerateReceiveBufferEnabled = true

	// tcpSACKEnabled is the value used by stack to enable or disable
	// tcp selective ACK.
	tcpSACKEnabled = true
)

type ForwarderCall func(conn *gonet.TCPConn) error
type UdpForwarderCall func(conn *gonet.UDPConn,ep tcpip.Endpoint) error






func NewDefaultStack(mtu int,tcpCallback ForwarderCall,udpCallback UdpForwarderCall) (*stack.Stack,*channel.Endpoint, error) {
	_netStack := stack.New( stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol,icmp.NewProtocol4,icmp.NewProtocol4}})
	//set ttl
	opt := tcpip.DefaultTTLOption(defaultTimeToLive)
	if err := _netStack.SetNetworkProtocolOption(ipv4.ProtocolNumber, &opt); err != nil {
		return nil,nil,errors.New(err.String())
	}
	if err := _netStack.SetNetworkProtocolOption(ipv6.ProtocolNumber, &opt); err != nil {
		return nil,nil,errors.New(err.String())
	}


	//转发开关,必须
	if err := _netStack.SetForwarding(ipv4.ProtocolNumber, ipForwardingEnabled); err != nil {
		return nil,nil,errors.New(err.String())
	}
	if err := _netStack.SetForwarding(ipv6.ProtocolNumber, ipForwardingEnabled); err != nil {
		return nil,nil,errors.New(err.String())
	}

	//
	_netStack.SetICMPBurst(icmpBurst)
	_netStack.SetICMPLimit(icmpLimit)



	rcvOpt := tcpip.TCPReceiveBufferSizeRangeOption{Min: minBufferSize, Default: defaultBufferSize, Max: maxBufferSize}
	if err := _netStack.SetTransportProtocolOption(tcp.ProtocolNumber, &rcvOpt); err != nil {
		return nil,nil,fmt.Errorf("set TCP receive buffer size range: %s", err)
	}
	sndOpt := tcpip.TCPSendBufferSizeRangeOption{Min: minBufferSize, Default: defaultBufferSize, Max: maxBufferSize}
	if err := _netStack.SetTransportProtocolOption(tcp.ProtocolNumber, &sndOpt); err != nil {
		return nil,nil,fmt.Errorf("set TCP send buffer size range: %s", err)
	}

	opt1 := tcpip.CongestionControlOption(tcpCongestionControlAlgorithm)
	if err := _netStack.SetTransportProtocolOption(tcp.ProtocolNumber, &opt1); err != nil {
		return nil,nil,fmt.Errorf("set TCP congestion control algorithm: %s", err)
	}

	opt2 := tcpip.TCPDelayEnabled(tcpDelayEnabled)
	if err := _netStack.SetTransportProtocolOption(tcp.ProtocolNumber, &opt2); err != nil {
		return nil,nil,fmt.Errorf("set TCP delay: %s", err)
	}

	opt3 := tcpip.TCPModerateReceiveBufferOption(tcpModerateReceiveBufferEnabled)
	if err := _netStack.SetTransportProtocolOption(tcp.ProtocolNumber, &opt3); err != nil {
		return nil,nil,fmt.Errorf("set TCP moderate receive buffer: %s", err)
	}

	opt4 := tcpip.TCPSACKEnabled(tcpSACKEnabled)
	if err := _netStack.SetTransportProtocolOption(tcp.ProtocolNumber, &opt4); err != nil {
		return nil,nil,fmt.Errorf("set TCP SACK: %s", err)
	}




	var nicid tcpip.NICID =1;
	macAddr, err := net.ParseMAC("de:ad:be:ee:ee:ef")
	if err != nil {
		fmt.Printf(err.Error());
		return nil,nil,err
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

		// IPv6
		{
			Destination: header.IPv6EmptySubnet,
			NIC:         nicid,
		},
	})
	_netStack.SetPromiscuousMode(nicid, true)
	_netStack.SetSpoofing(nicid, true);

	tcpForwarder := tcp.NewForwarder(_netStack, 0, 2048, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			fmt.Printf("CreateEndpoint err:"+err.String()+"\r\n");
			r.Complete(true)
			return
		}
		defer ep.Close();
		r.Complete(false)
		if err := setKeepalive(ep); err != nil {
			fmt.Printf("setKeepalive"+err.Error()+"\r\n");
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
			fmt.Printf("r.CreateEndpoint() err:%v\r\n", err)
			return
		}
		go udpCallback(gonet.NewUDPConn(_netStack,&wq, ep),ep);
	})
	_netStack.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)

	return _netStack,channelLinkID,nil
}


func setKeepalive(ep tcpip.Endpoint) error {
	if err := ep.SetSockOptBool(tcpip.KeepaliveEnabledOption, true); err != nil {
		return fmt.Errorf("set keepalive: %s", err)
	}
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