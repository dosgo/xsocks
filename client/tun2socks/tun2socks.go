package tun2socks

import (
	"bytes"
	"context"
	"io"
	"log"

	"github.com/dosgo/xsocks/comm"
	"gvisor.dev/gvisor/pkg/buffer"
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
		var sendBuffer = new(bytes.Buffer)
		for {
			info := channelLinkID.ReadContext(_ctx)
			if info == nil {
				log.Printf("channelLinkID exit \r\n")
				break
			}
			info.Data().AsRange().AsView()
			sendBuffer.Reset()
			sendBuffer.Write(info.NetworkHeader().View())
			sendBuffer.Write(info.TransportHeader().View())
			sendBuffer.Write(info.Data().AsRange().ToOwnedView())
			if sendBuffer.Len() > 0 {
				dev.Write(sendBuffer.Bytes())
			}
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

		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.NewWithData(buf[:n]),
		})
		//channelLinkID.InjectInbound(header.IPv4ProtocolNumber, pkt)
		switch header.IPVersion(buf[:]) {
		case header.IPv4Version:
			channelLinkID.InjectInbound(header.IPv4ProtocolNumber, pkt)
		case header.IPv6Version:
			channelLinkID.InjectInbound(header.IPv6ProtocolNumber, pkt)
		}
		pkt.DecRef()
	}
	return nil
}
