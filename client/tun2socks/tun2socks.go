package tun2socks

import (
	"context"
	"io"
	"log"

	"github.com/dosgo/xsocks/comm"

	"gvisor.dev/gvisor/pkg/bufferv2"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

func ForwardTransportFromIo(dev io.ReadWriteCloser, mtu int, tcpCallback comm.ForwarderCall, udpCallback comm.UdpForwarderCall) error {
	_, channelLinkID, err := comm.NewDefaultStack(mtu, tcpCallback, udpCallback)
	if err != nil {
		log.Printf("err:%v", err)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// write tun
	go func(_ctx context.Context) {
		for {
			info := channelLinkID.ReadContext(_ctx)
			if info == nil {
				log.Printf("channelLinkID exit \r\n")
				break
			}
			info.ToView().WriteTo(dev)
			info.DecRef()
		}
	}(ctx)

	// read tun data
	var buf = make([]byte, mtu+80)
	for {
		n, e := dev.Read(buf[:])
		if e != nil {
			log.Printf("err:%v", err)
			break
		}
		//hdr := buf[0:14]
		//	payload := buf[14:n]

		//proto := tcpip.NetworkProtocolNumber(binary.BigEndian.Uint16(buf[12:14]))
		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			//ReserveHeaderBytes: len(hdr),
			Payload: bufferv2.MakeWithData(buf[:n]),
			//IsForwardedPacket: true,
		})
		//pkt.LinkHeader().Push()
		//copy(pkt.LinkHeader().Push(len(hdr)), hdr)
		//channelLinkID.InjectInbound(header.IPv4ProtocolNumber, pkt)
		//pkt.LinkHeader()

		switch header.IPVersion(buf) {
		case header.IPv4Version:
			channelLinkID.InjectInbound(header.IPv4ProtocolNumber, pkt)
		case header.IPv6Version:
			channelLinkID.InjectInbound(header.IPv6ProtocolNumber, pkt)
		}
		pkt.DecRef()
	}
	return nil
}
