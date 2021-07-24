package tun2socks

import (
	"bytes"
	"context"
	"github.com/dosgo/xsocks/comm"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"io"
	"log"
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




