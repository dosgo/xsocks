package server

import (
	"errors"
	"fmt"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"log"

	//"github.com/google/netstack/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"

	"net"
	"strings"
	"time"
	"github.com/dosgo/xSocks/comm"
	"github.com/dosgo/xSocks/param"
)


func InjectInbound(channelLinkID *channel.Endpoint,buf []byte) error{
	tmpView:=buffer.NewVectorisedView(len(buf),[]buffer.View{
		buffer.NewViewFromBytes(buf),
	})
	if channelLinkID==nil {
		log.Println("channelLinkID nil")
		return errors.New("channelLinkID nil");
	}
	channelLinkID.InjectInbound(header.IPv4ProtocolNumber, stack.NewPacketBuffer(stack.PacketBufferOptions{
		Data: tmpView,
	}))
	return nil;
}

/*tcp*/
func StartTunStack(mtu uint16) (*stack.Stack,*channel.Endpoint,error){
	return comm.NewDefaultStack(int(mtu),tcpForward,udpForward);
}


/*udp 转发*/
func udpForward(conn *gonet.UDPConn,ep tcpip.Endpoint) error{
	defer ep.Close();
	var remoteAddr="";
	var duration time.Duration=time.Second*100;
	//dns 8.8.8.8
	if strings.HasSuffix(conn.LocalAddr().String(),":53") {
		fmt.Printf("udpForward dnsAddr:%s",conn.LocalAddr().String()+"localAddr:"+conn.RemoteAddr().String()+"SafeDns:"+param.Args.SafeDns+"\r\n")
		remoteAddr=param.Args.SafeDns+":53"
		duration=time.Second*15;
	}else{
		remoteAddr=conn.LocalAddr().String();
	}
	conn2, err := net.DialTimeout("udp",remoteAddr,time.Second*15);
	if err != nil {
		fmt.Println("udpForward"+conn.LocalAddr().String()+ err.Error())
		return err;
	}
	comm.UdpPipe(conn,conn2,duration)
	return nil;
}



/*tcpForward*/
func tcpForward(conn *gonet.TCPConn) error{
	conn2, err := net.DialTimeout("tcp", conn.LocalAddr().String(),param.Args.ConnectTime);
	if err != nil {
		fmt.Println("tcpForward"+conn.LocalAddr().String()+ err.Error())
		return err;
	}
	comm.TcpPipe(conn,conn2,time.Minute*5)
	return nil;
}