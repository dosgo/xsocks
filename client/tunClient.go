package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/google/netstack/tcpip"
	"github.com/google/netstack/tcpip/adapters/gonet"
	"github.com/google/netstack/tcpip/buffer"
	"github.com/google/netstack/tcpip/header"
	"github.com/yinghuocho/gotun2socks/tun"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"runtime"
	"sync"
	"strings"
	"time"
	"xSocks/comm"
	"xSocks/param"
)



func StartTun(tunDevice string,tunAddr string,tunMask string,tunGW string,tunDNS string) error {
	if(len(tunDevice)==0){
		tunDevice="tun0";
	}
	if(len(tunAddr)==0){
		tunAddr="10.0.0.2";
	}
	if(len(tunMask)==0){
		tunMask="255.255.255.0";
	}
	if(len(tunGW)==0){
		tunGW="10.0.0.1";
	}
	if(len(tunDNS)==0){
		tunDNS="114.114.114.114";
	}
	//
	var oldGw=comm.GetGateway();

	if(len(param.UnixSockTun)>0) {
		addr, err := net.ResolveUnixAddr("unixpacket", param.UnixSockTun)
		if err != nil {
			return err;
		}
		lis, err := net.ListenUnix("unixpacket", addr)
		if err != nil { //如果监听失败，一般是文件已存在，需要删除它
			log.Println("UNIX Domain Socket 创 建失败，正在尝试重新创建 -> ", err)
			os.Remove(param.UnixSockTun)
			return err;
		}
		defer lis.Close() //虽然本次操作不会执行， 不过还是加上比较好
		conn, err := lis.Accept() //开始接 受数据
		defer conn.Close()
		if err != nil {                      //如果监听失败，一般是文件已存在，需要删除它
			return err;
		}
		tunRecv(conn, param.Mtu)
	}else{
		var remoteAddr string;
		if(runtime.GOOS=="windows") {
			urlInfo, _ := url.Parse(param.ServerAddr)
			addr, err := net.ResolveIPAddr("ip",urlInfo.Hostname())
			if err == nil {
				remoteAddr = addr.String()
			}
			fmt.Printf("remoteAddr:%s\r\n", remoteAddr)
		}

		//old gw
		dnsServers := strings.Split(tunDNS, ",")
		f, err:= tun.OpenTunDevice(tunDevice, tunAddr, tunGW, tunMask, dnsServers)
		if err != nil {
			fmt.Println("Error listening:", err)
			return err;
		}
		//windows
		if(runtime.GOOS=="windows"){
			routeEdit(tunGW,remoteAddr,dnsServers,oldGw);
		}
		tunRecv(f, param.Mtu)
	}
	return nil;
}

func tunRecv(dev io.ReadWriteCloser ,mtu int) error{
	if(param.TunSmartProxy) {
		channelLinkID,_stack, err := comm.GenChannelLinkID(mtu, tcpForward, udpForward);
		if (err != nil) {
			return err;
		}
		defer _stack.Close();
		// write tun
		go func() {
			var buffer = new(bytes.Buffer)
			for {
				select {
				case pkt := <-channelLinkID.C:
					buffer.Write(pkt.Pkt.Header.View())
					buffer.Write(pkt.Pkt.Data.ToView())
					//tmpBuf:=append(pkt.Pkt.Header.View(),pkt.Pkt.Data.ToView()...)
					if (buffer.Len() > 0) {
						dev.Write(buffer.Bytes())
						buffer.Reset()
					}
					break;
				}
			}
		}()

		// read tun data
		var buf = make([]byte, mtu)
		for {
			n, e := dev.Read(buf[:])
			if e != nil {
				fmt.Printf("e:%v\r\n", e)
				break;
			}
			//判断是否是本地数据,如果是直接转发给远程
			if (true) {
				fmt.Printf("dsfsd");
			} else {
				tmpView := buffer.NewVectorisedView(n, []buffer.View{
					buffer.NewViewFromBytes(buf[:n]),
				})
				channelLinkID.InjectInbound(header.IPv4ProtocolNumber, tcpip.PacketBuffer{
					Data: tmpView,
				})
			}
		}
	}else{
		tunStream:=TunStream{}
		tunStream.StreamSwapTun(dev,mtu)
	}
	return nil
}



type TunStream struct {
	Tunnel comm.CommConn
	sync.Mutex
}

/*send cmd  and UniqueId  and mtu*/
func (rd *TunStream) Connect(uniqueId string,mtu int)(comm.CommConn,error){
	var err error;
	tunnel,err:=NewTunnel();
	if err != nil {
		fmt.Printf("connect tunnel err:%v\r\n",err)
		return nil,err;
	}
	cmdBuf := make([]byte, 1)
	cmdBuf[0] = 0x03; //cmd 0x03 to tun
	tunnel.Write(cmdBuf);
	////wtite UniqueId byte (8byte)
	tunnel.Write([]byte(uniqueId))

	//wtite mtu byte
	var mtuByte []byte = make([]byte, 2)
	binary.LittleEndian.PutUint16(mtuByte,uint16(mtu))
	tunnel.Write(mtuByte)
	return tunnel,nil;
}

/*  */
func (rd *TunStream) StreamSwapTun(dev comm.CommConn,mtu int){
	rd.Lock();
	defer  rd.Unlock()
	var err error;
	//uniqueId
	var uniqueId=comm.UniqueId(8);
	if(rd.Tunnel==nil) {
		for{
			rd.Tunnel,err=rd.Connect(uniqueId,mtu);
			if(err==nil){
				break;
			}
			time.Sleep(30*time.Second);
		}
	}
	go func() {
		var packLenByte []byte = make([]byte, 2)
		var bufByte []byte = make([]byte,mtu)
		for {
			n, err := dev.Read(bufByte[:])
			if err != nil {
				fmt.Printf("e:%v\r\n", err)
				break;
			}
			//fmt.Printf("dev read len:%d\r\n",n);
			binary.LittleEndian.PutUint16(packLenByte, uint16(n))
			_,err=rd.Tunnel.Write(packLenByte)
			if (err != nil) {
				tunnel,err:=rd.Connect(uniqueId,mtu);
				if(err==nil){
					rd.Tunnel=tunnel;
				}
				fmt.Printf("re TunStream 1 e:%v\r\n", err)
			}
			_,err=rd.Tunnel.Write(bufByte[:n])
			if (err != nil) {
				tunnel,err:=rd.Connect(uniqueId,mtu);
				if(err==nil){
					rd.Tunnel=tunnel;
				}
				fmt.Printf("re TunStream 2 e:%v\r\n", err)
			}
		}
	}();

	var packLenByte []byte = make([]byte, 2)
	var bufByte []byte = make([]byte,mtu)
	for {
		_, err := io.ReadFull(rd.Tunnel, packLenByte)
		if (err != nil) {
			tunnel,err:=rd.Connect(uniqueId,mtu);
			if(err==nil){
				rd.Tunnel=tunnel;
			}
			fmt.Printf("re TunStream 3 e:%v\r\n", err)
		}
		packLen := binary.LittleEndian.Uint16(packLenByte)
		_, err = io.ReadFull(rd.Tunnel, bufByte[:int(packLen)])
		if (err != nil) {
			tunnel,err:=rd.Connect(uniqueId,mtu);
			if(err==nil){
				rd.Tunnel=tunnel;
			}
			fmt.Printf("re TunStream 4 e:%v\r\n", err)
		}
		_,err=dev.Write(bufByte[:int(packLen)])
		if (err != nil) {
			fmt.Printf("e:%v\r\n", err)
		}
	}
}


/*udp 转发*/
func udpForward(conn *gonet.Conn, ep tcpip.Endpoint) error{
	defer conn.Close();
	defer ep.Close();
	conn2, err := net.Dial("udp",conn.LocalAddr().String());
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	defer conn2.Close();
	go io.Copy(conn,conn2)
	io.Copy(conn2,conn)
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