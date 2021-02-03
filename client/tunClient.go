package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"gvisor.dev/gvisor/pkg/tcpip/stack"

	//"github.com/google/netstack/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip"
	//"github.com/google/netstack/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	//"github.com/google/netstack/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	//"github.com/google/netstack/tcpip/header"
	"github.com/yinghuocho/gotun2socks/tun"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
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
		os.Remove(param.UnixSockTun)
		addr, err := net.ResolveUnixAddr("unixpacket", param.UnixSockTun)
		if err != nil {
			return err;
		}
		lis, err := net.ListenUnix("unixpacket", addr)
		if err != nil { //如果监听失败，一般是文件已存在，需要删除它
			log.Println("UNIX Domain Socket 创 建失败，正在尝试重新创建 -> ", err)
			return err;
		}
		defer lis.Close() //虽然本次操作不会执行， 不过还是加上比较好
		conn, err := lis.Accept() //开始接 受数据
		defer conn.Close()
		defer os.Remove(param.UnixSockTun)
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
		_,channelLinkID, err := comm.NewDefaultStack(mtu, tcpForward, udpForward);
		if (err != nil) {
			return err;
		}
		// write tun
		go func() {
			var buffer = new(bytes.Buffer)
			for {
				pkt,res:=channelLinkID.Read()
				if(!res){
					continue;
				}
				buffer.Reset()
				//buffer.Write(pkt.Pkt.LinkHeader().View())
				buffer.Write(pkt.Pkt.NetworkHeader().View())
				buffer.Write(pkt.Pkt.TransportHeader().View())
				buffer.Write(pkt.Pkt.Data.ToView())
				//tmpBuf:=append(pkt.Pkt.Header.View(),pkt.Pkt.Data.ToView()...)
				if (buffer.Len() > 0) {
					dev.Write(buffer.Bytes())
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
				tmpView:=buffer.NewVectorisedView(n,[]buffer.View{
					buffer.NewViewFromBytes(buf[:n]),
				})
				channelLinkID.InjectInbound(header.IPv4ProtocolNumber, stack.NewPacketBuffer(stack.PacketBufferOptions{
					Data: tmpView,
				}))
			}
		}
	}else{
		if (strings.HasPrefix(param.ServerAddr,"sudp")) {
			packetSwapTun(dev, mtu);
		}else {
			StreamSwapTun(dev, mtu)
		}
	}
	return nil
}



type TunConn struct {
	Tunnel comm.CommConn
	UdpConn *net.UDPConn
	UniqueId string
	Mtu int;
	sync.Mutex
}
func (rd *TunConn) GetTunnel()(comm.CommConn){
	rd.Lock();
	defer rd.Unlock();
	return rd.Tunnel;
}
func (rd *TunConn) PutTunnel(tunnel comm.CommConn){
	rd.Lock();
	defer rd.Unlock();
	rd.Tunnel=tunnel;
}


func (rd *TunConn) GetPacket()(*net.UDPConn){
	rd.Lock();
	defer rd.Unlock();
	return rd.UdpConn;
}
func (rd *TunConn) PutPacket(tunnel *net.UDPConn){
	rd.Lock();
	defer rd.Unlock();
	rd.UdpConn=tunnel;
}


/*send cmd  and UniqueId  and mtu*/
func  ConnectTun(uniqueId string,mtu int)(comm.CommConn,error){
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

func connectUdp()(*net.UDPConn,error){
	udpAddr, err := net.ResolveUDPAddr("udp4", 	param.ServerAddr[7:])
	if(err!=nil){
		return nil,err;
	}
	return net.DialUDP("udp4", nil, udpAddr)
}


/*udp packet*/
func  packetSwapTun(dev  io.ReadWriteCloser,mtu int){
	tunPacket:=&TunConn{}
	videoHeader:=comm.NewVideoChat();
	var aesGcm=comm.NewAesGcm();
	if(aesGcm==nil){
		fmt.Println("aesGcm init error")
	}
	go func(_tunPacket *TunConn) {
		var bufByte []byte = make([]byte,mtu+80)
		var buffer bytes.Buffer
		var buffer2 bytes.Buffer
		var header []byte = make([]byte, videoHeader.Size())
		for {
			n, err := dev.Read(bufByte[:])
			if err != nil {
				fmt.Printf("dev err%v\r\n", err)
				break;
			}
			buffer.Reset()
			videoHeader.Serialize(header)
			buffer.Write(header)


			buffer2.Reset();
			//MTU
			var mtuByte []byte = make([]byte, 2)
			binary.LittleEndian.PutUint16(mtuByte,uint16(mtu))
			buffer2.Write(mtuByte)
			//packet
			buffer2.Write(bufByte[:n]);

			ciphertext,_:=aesGcm.AesGcm(buffer2.Bytes(),true);
			buffer.Write(ciphertext)
			udpConn:=_tunPacket.GetPacket();
			if(udpConn!=nil) {
				_,err=udpConn.Write(buffer.Bytes())
				if(err!=nil){
					udpConn.Close();
					_tunPacket.PutPacket(nil)
				}
			}
		}
	}(tunPacket);

	var buffer []byte = make([]byte,65535)
	for {
		tunnel:=tunPacket.GetPacket();
		if(tunnel==nil){
			_tunnel,err:=connectUdp();
			if(err==nil){
				tunPacket.PutPacket(_tunnel)
			}else {
				time.Sleep(10 * time.Second);
				fmt.Printf("re tunPacket 3 e:%v\r\n", err)
			}
			continue;
		}


		n, _, err := tunnel.ReadFromUDP(buffer)
		if err != nil {
			tunPacket.PutPacket(nil)
			continue;
		}
		ciphertext,err:=aesGcm.AesGcm(buffer[videoHeader.Size():n],false);
		if (err==nil){
			_, err = dev.Write(ciphertext)
			if (err != nil) {
				fmt.Printf("e:%v\r\n", err)
			}
		}else{
			timeStr:=fmt.Sprintf("%d",time.Now().Unix())
			nonce:=timeStr[:len(timeStr)-2]
			fmt.Println("Decryption failed nonce:",nonce,err);
		}
	}
}



/*tcp  Stream */
func  StreamSwapTun(dev io.ReadWriteCloser,mtu int){
	tunStream:=&TunConn{}
	tunStream.UniqueId=comm.UniqueId(8)
	tunStream.Mtu=mtu;

	go func(_tunStream *TunConn) {
		var packLenByte []byte = make([]byte, 2)
		var bufByte []byte = make([]byte,mtu+80)
		var buffer bytes.Buffer
		var tunnel comm.CommConn
		for {
			n, err := dev.Read(bufByte[:])
			if err != nil {
				fmt.Printf("dev err%v\r\n", err)
				break;
			}
			//fmt.Printf("dev read len:%d\r\n",n);
			binary.LittleEndian.PutUint16(packLenByte, uint16(n))
			buffer.Reset()
			buffer.Write(packLenByte)
			buffer.Write(bufByte[:n])
			tunnel=_tunStream.GetTunnel();
			if(tunnel!=nil){
				_,err=tunnel.Write(buffer.Bytes())
				if (err != nil) {
					fmt.Printf("tunnel wrtie err:%v\r\n", err)
				}
			}
		}
	}(tunStream);

	var packLenByte []byte = make([]byte, 2)
	var bufByte []byte = make([]byte,mtu+80)
	var tunnel comm.CommConn
	for {
		tunnel=tunStream.GetTunnel();
		if(tunnel==nil){
			_tunnel,err:=ConnectTun(tunStream.UniqueId,tunStream.Mtu);
			if(err==nil){
				tunStream.PutTunnel(_tunnel)
			}else {
				time.Sleep(10 * time.Second);
				fmt.Printf("re TunStream 3 e:%v\r\n", err)
			}
			continue;
		}
		_, err := io.ReadFull(tunnel, packLenByte)
		packLen := binary.LittleEndian.Uint16(packLenByte)
		if (err != nil||int(packLen)>len(bufByte)) {
			tunStream.PutTunnel(nil)
			continue;
		}
		_, err = io.ReadFull(tunnel, bufByte[:int(packLen)])
		if (err != nil) {
			fmt.Printf("recv pack err :%v\r\n", err)
			tunStream.PutTunnel(nil)
			continue;
		}else {
			_, err = dev.Write(bufByte[:int(packLen)])
			if (err != nil) {
				fmt.Printf("e:%v\r\n", err)
			}
		}
	}
}


/*udp 转发*/
func udpForward(conn *gonet.UDPConn, ep tcpip.Endpoint) error{
	defer conn.Close();
	defer ep.Close();
	conn2, err := net.Dial("udp",conn.LocalAddr().String());
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	defer conn2.Close();
	comm.UdpPipe(conn,conn2);
	return nil;
}

/*udp 转发*/
func tcpForward(conn *gonet.TCPConn) error{
	conn2, err := net.DialTimeout("tcp", conn.LocalAddr().String(),param.ConnectTime);
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	comm.TcpPipe(conn,conn2,time.Minute*5)
	return nil;
}