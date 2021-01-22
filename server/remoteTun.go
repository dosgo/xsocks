package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/google/netstack/tcpip"
	"github.com/google/netstack/tcpip/adapters/gonet"
	"github.com/google/netstack/tcpip/buffer"
	"github.com/google/netstack/tcpip/header"
	"io"
	"log"
	"net"
	"os"
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

/*unix socket*/
func StartTun() error{
	os.Remove(param.LocalTunSock)
	addr, err := net.ResolveUnixAddr("unixpacket", param.LocalTunSock)
	if err != nil {
		return err;
	}
	lis, err := net.ListenUnix("unixpacket", addr)
	if err != nil {                      //如果监听失败，一般是文件已存在，需要删除它
		log.Println("UNIX Domain Socket 创 建失败，正在尝试重新创建 -> ", err)
		os.Remove(param.LocalTunSock)
		return err;
	}
	defer lis.Close() //虽然本次操作不会执行， 不过还是加上比较好
	for {
		client, err := lis.Accept()
		if err != nil {
			log.Panic(err)
		}
		client.RemoteAddr();
		go newTunUnix(client)
	}
}

func newTunUnix(client comm.CommConn) error{
	var mtuByte []byte = make([]byte, 2)
	//read Mtu
	n, err := client.Read(mtuByte[:])
	if(err!=nil||n!=2){
		log.Printf("err:%v\r\n")
		return err;
	}
	mtu := binary.LittleEndian.Uint16(mtuByte)
	if(mtu<1){
		mtu=1024;
	}
	stack,channelLinkID,err:=comm.GenChannelLinkID(int(mtu),tcpForward,udpForward);
	if(err!=nil){
		log.Printf("err:%v\r\n",err)
		return err;
	}
	defer stack.CleanupEndpoints()
	defer stack.Close();
	// write tun
	go func() {
		var buffer =new(bytes.Buffer)
		for {
			select {
			case pkt := <-channelLinkID.C:
				buffer.Write(pkt.Pkt.Header.View())
				buffer.Write(pkt.Pkt.Data.ToView())
				if(buffer.Len()>0) {
					client.Write(buffer.Bytes())
					buffer.Reset()
				}
				break;
			}
		}
		fmt.Printf("channelLinkID recv exit \r\n");
	}()
	// read tun data
	var buflen=mtu+80;
	var buf=make([]byte,buflen)
	for {
		n, err := client.Read(buf)
		if (err != nil) {
			log.Printf("err:%v\r\n",err)
			return err;
		}
		tmpView:=buffer.NewVectorisedView(n,[]buffer.View{
			buffer.NewViewFromBytes(buf[:n]),
		})
		channelLinkID.InjectInbound(header.IPv4ProtocolNumber, tcpip.PacketBuffer{
			Data: tmpView,
		})
	}
	return nil
}

/*tcp*/
func newTunTcp(client comm.CommConn) error{
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
	stack,channelLinkID,err:=comm.GenChannelLinkID(int(mtu),tcpForward,udpForward);
	if(err!=nil){
		log.Printf("err:%v\r\n",err)
		return err;
	}
	defer stack.CleanupEndpoints()
	defer stack.Close();
	// write tun
	go func() {
		var buffer =new(bytes.Buffer)
		var packLenByte []byte = make([]byte, 2)
		for {
			select {
				case pkt := <-channelLinkID.C:
					buffer.Write(pkt.Pkt.Header.View())
					buffer.Write(pkt.Pkt.Data.ToView())
					if(buffer.Len()>0) {
						binary.LittleEndian.PutUint16(packLenByte,uint16(buffer.Len()))
						client.Write(packLenByte)
						client.Write(buffer.Bytes())
						buffer.Reset()
					}
					break;
			}
		}
		fmt.Printf("channelLinkID recv exit \r\n");
	}()

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
		channelLinkID.InjectInbound(header.IPv4ProtocolNumber, tcpip.PacketBuffer{
			Data: tmpView,
		})
	}
	return nil
}


/*udp 转发*/
func udpForward(conn *gonet.Conn,ep tcpip.Endpoint) error{
	defer conn.Close();
	defer ep.Close();
	conn2, err := net.Dial("udp", conn.LocalAddr().String());
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	defer conn2.Close();
	go func() {
		buf := make([]byte, 65535)
		for {
			n, e := conn2.Read(buf)
			if e != nil {
				return ;
			}
			if _, e := conn.Write(buf[:n]); e != nil {
				return ;
			}
		}
	}()
	buf := make([]byte, 65535)
	for {
		conn.SetDeadline(time.Now().Add(time.Second*120))
		n, e := conn.Read(buf)
		if e != nil {
			return err;
		}
		conn2.SetDeadline(time.Now().Add(time.Second*120))
		if _, e := conn2.Write(buf[:n]); e != nil {
			return err;
		}
	}
	return nil;
}



/*udp 转发*/
func tcpForward(conn *gonet.Conn) error{
	conn2, err := net.DialTimeout("tcp", conn.LocalAddr().String(),param.ConnectTime);
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	defer conn2.Close();
	go io.Copy(conn,conn2)
	io.Copy(conn2,conn)
	return nil;
}