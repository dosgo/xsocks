package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"

	//"github.com/google/netstack/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	//"github.com/google/netstack/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"io"
	"context"
	"log"
	"net"
	"strings"
	"time"
	"xSocks/comm"
	"xSocks/param"
)

/*tcp*/
func StartTunTcp() error {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	l, err := net.Listen("tcp", "127.0.0.1:"+param.TunPort)
	if err != nil {
		log.Panic(err)
	}
	for {
		client, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}
		go newTunTcp(client)
	}
}


/*tcp*/
func newTunTcp(client comm.CommConn) error{
	defer client.Close();
	var mtuByte []byte = make([]byte, 2)
	//read Mtu
	_, err := io.ReadFull(client, mtuByte)
	if(err!=nil){
		log.Printf("err:%v\r\n")
		return err;
	}
	mtu := binary.LittleEndian.Uint16(mtuByte)
	if(mtu<1){
		mtu=1024;
	}
	_,channelLinkID,err:=comm.NewDefaultStack(int(mtu),tcpForward,udpForward);
	if(err!=nil){
		log.Printf("err:%v\r\n",err)
		return err;
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()


	// write tun
	go func(_ctx context.Context) {
		var buffer =new(bytes.Buffer)
		var sendBuffer =new(bytes.Buffer)
		var packLenByte []byte = make([]byte, 2)
		defer fmt.Printf("channelLinkID recv exit \r\n");
		for {
				//pkt,res :=channelLinkID.ReadContext(_ctx)
				pkt,res :=channelLinkID.ReadContext(_ctx)
				if(!res){
					break;
				}
				buffer.Reset()
				//buffer.Write(pkt.Pkt.LinkHeader().View())
				buffer.Write(pkt.Pkt.NetworkHeader().View())
				buffer.Write(pkt.Pkt.TransportHeader().View())
				buffer.Write(pkt.Pkt.Data.ToView())
				if(buffer.Len()>0) {
					binary.LittleEndian.PutUint16(packLenByte,uint16(buffer.Len()))
					sendBuffer.Reset()
					sendBuffer.Write(packLenByte)
					sendBuffer.Write(buffer.Bytes())
					_,err=client.Write(sendBuffer.Bytes())
					if(err!=nil){
						return ;
					}
				}
		}
	}(ctx)

	// read tun data
	var buflen=mtu+80;
	var buf=make([]byte,buflen)
	var packLenByte []byte = make([]byte, 2)
	for {
		_, err := io.ReadFull(client, packLenByte)
		if (err != nil) {
			log.Printf("err:%v\r\n",err)
			return err;
		}
		packLen := binary.LittleEndian.Uint16(packLenByte)
		//null
		if(packLen<1||packLen>buflen) {
			continue;
		}
		n, err:= io.ReadFull(client,buf[:int(packLen)])
		if (err != nil) {
			log.Printf("err:%v\r\n",err)
			return err;
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


/*udp 转发*/
func udpForward(conn *gonet.UDPConn,ep tcpip.Endpoint) error{
	defer ep.Close();
	var remoteAddr="";
	//dns 8.8.8.8
	if(strings.HasSuffix(conn.LocalAddr().String(),":53")){
		fmt.Printf("udpForward dnsAddr:%s",conn.LocalAddr().String()+"localAddr:"+conn.RemoteAddr().String()+"SafeDns:"+param.SafeDns+"\r\n")
		remoteAddr=param.SafeDns+":53"
	}else{
		remoteAddr=conn.LocalAddr().String();
	}
	conn2, err := net.Dial("udp",remoteAddr);
	if err != nil {
		fmt.Println("udpForward"+conn.LocalAddr().String()+ err.Error())
		return err;
	}
	comm.UdpPipe(conn,conn2)
	return nil;
}



/*tcpForward*/
func tcpForward(conn *gonet.TCPConn) error{
	conn2, err := net.DialTimeout("tcp", conn.LocalAddr().String(),param.ConnectTime);
	if err != nil {
		fmt.Println("tcpForward"+conn.LocalAddr().String()+ err.Error())
		return err;
	}
	comm.TcpPipe(conn,conn2,time.Minute*5)
	return nil;
}