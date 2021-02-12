package client

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"time"
	"xSocks/comm"
	//"github.com/google/netstack/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"github.com/yinghuocho/gotun2socks/tun"
	//"github.com/google/netstack/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"io"
	"log"
	"net"
	"net/url"
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"xSocks/param"
)

var  wrapnet uint32;
var  mask uint32
var  relayip net.IP
var  port  uint16;



func StartTunDevice(tunDevice string,tunAddr string,tunMask string,tunGW string,tunDNS string) {
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

	strings.Split(param.ServerAddr,":");
	dnsServers := strings.Split(tunDNS, ",")
	fmt.Printf("dnsServers:%v\r\n",dnsServers)
	var dev io.ReadWriteCloser;
	var remoteAddr string;
	if(len(param.UnixSockTun)>0) {
		os.Remove(param.UnixSockTun)
		addr, err := net.ResolveUnixAddr("unixpacket", param.UnixSockTun)
		if err != nil {
			return ;
		}
		lis, err := net.ListenUnix("unixpacket", addr)
		if err != nil {                      //如果监听失败，一般是文件已存在，需要删除它
			log.Println("UNIX Domain Socket 创 建失败，正在尝试重新创建 -> ", err)
			os.Remove(param.UnixSockTun)
			return ;
		}
		defer lis.Close() //虽然本次操作不会执行， 不过还是加上比较好
		conn, err := lis.Accept() //开始接 受数据
		if err != nil {                      //如果监听失败，一般是文件已存在，需要删除它
			return ;
		}
		dev=conn;
		defer conn.Close()
	}else{
		if(runtime.GOOS=="windows") {
			urlInfo, _ := url.Parse(param.ServerAddr)
			addr, err := net.ResolveIPAddr("ip",urlInfo.Hostname())
			if err == nil {
				remoteAddr = addr.String()
			}
			fmt.Printf("remoteAddr:%s\r\n", remoteAddr)
		}

		f, err:= tun.OpenTunDevice(tunDevice, tunAddr, tunGW, tunMask, dnsServers)
		if err != nil {
			fmt.Println("Error listening:", err)
			return ;
		}
		dev=f;
	}

	//windows
	if(runtime.GOOS=="windows"){
		routeEdit(tunGW,remoteAddr,dnsServers,oldGw);
	}
	ForwardTransportFromIo(dev,param.Mtu);
}
func ForwardTransportFromIo(dev io.ReadWriteCloser,mtu int) error {
	_,channelLinkID,err:=comm.NewDefaultStack(mtu,tcpForwarder,udpForwarder);
	if(err!=nil){
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
			if(!res){
				break;
			}
			sendBuffer.Reset()
			//buffer.Write(pkt.Pkt.LinkHeader().View())
			sendBuffer.Write(info.Pkt.NetworkHeader().View())
			sendBuffer.Write(info.Pkt.TransportHeader().View())
			sendBuffer.Write(info.Pkt.Data.ToView())
			if(sendBuffer.Len()>0) {
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

func tcpForwarder(conn *gonet.TCPConn)error{
	var remoteAddr=conn.LocalAddr().String()
	//dns ,use 8.8.8.8
	if(strings.HasSuffix(remoteAddr,":53")){
		dnsReqTcp(conn);
		return  nil;
	}
	socksConn,err1:= net.DialTimeout("tcp",param.Sock5Addr,time.Second*15)
	if err1 != nil {
		log.Printf("err:%v",err1)
		return nil
	}
	defer socksConn.Close();
	if(socksCmd(socksConn,1,remoteAddr)==nil) {
		comm.TcpPipe(conn,socksConn,time.Minute*5)
	}
	return nil
}

func udpForwarder(conn *gonet.UDPConn, ep tcpip.Endpoint)error{
	defer ep.Close();
	defer conn.Close();
	//dns port
	if(strings.HasSuffix(conn.LocalAddr().String(),":53")){
		dnsReqUdp(conn);
	}else{
		socksUdpGate(conn);
	}
	return nil;
}
func dnsReqUdp(conn *gonet.UDPConn) error{
	dnsConn, err := net.DialTimeout("udp", "127.0.0.1:"+param.DnsPort,time.Second*15);
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	comm.UdpPipe(conn,dnsConn,time.Minute*5)
	return nil;
}
/*to dns*/
func dnsReqTcp(conn *gonet.TCPConn) error{
	dnsConn, err := net.DialTimeout("tcp", "127.0.0.1:"+param.DnsPort,time.Second*15);
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	comm.TcpPipe(conn,dnsConn,time.Minute*2)
	fmt.Printf("dnsReq Tcp\r\n");
	return nil;
}


/*to socks5 udp gate */
func socksUdpGate(conn *gonet.UDPConn) error{
	gateConn, err := net.DialTimeout("udp", "127.0.0.1:"+param.Sock5UdpPort,time.Second*15);
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	defer conn.Close()
	defer gateConn.Close()
	dstAddr,_:=net.ResolveUDPAddr("udp",conn.LocalAddr().String())

	go func() {
		var buffer bytes.Buffer
		var b1=make([]byte,65535);
		for {
			conn.SetReadDeadline(time.Now().Add(3*time.Minute))
			n,err:=conn.Read(b1);
			if err != nil {
				return ;
			}
			buffer.Reset()
			buffer.Write(comm.UdpHeadEncode(dstAddr))
			buffer.Write(b1[:n])
			_, _ = gateConn.Write(buffer.Bytes())
		}
	}()
	for {
		var b2=make([]byte,65535);
		gateConn.SetReadDeadline(time.Now().Add(3*time.Minute))
		n,err:=gateConn.Read(b2);
		if err != nil {
			return err;
		}
		_,dataStart,err:=comm.UdpHeadDecode(b2[:n])
		if err != nil {
			return nil;
		}
		_, _ = conn.Write(b2[dataStart:])
	}
}


/*to socks5*/
func socksCmd(socksConn net.Conn,cmd uint8,host string) error{
	//socks5 auth
	socksConn.Write([]byte{0x05, 0x01,0x00});
	//connect head
	hosts:=strings.Split(host,":");
	rAddr:=net.ParseIP(hosts[0])
	_port, _ := strconv.Atoi(hosts[1])
	msg := []byte{0x05, cmd, 0x00, 0x01}
	buffer := bytes.NewBuffer(msg)
	//ip
	binary.Write(buffer, binary.BigEndian, rAddr.To4())
	//port
	binary.Write(buffer, binary.BigEndian, uint16(_port))
	socksConn.Write(buffer.Bytes());

	//recv auth back
	authBack := make([]byte, 2)
	_, err:= io.ReadFull(socksConn, authBack)
	if err != nil {
		log.Println(err)
		return err
	}
	if authBack[0]!=0x05||authBack[1]!=0x00 {
		log.Println("auth error")
		return errors.New("auth error");
	}

	//recv connectBack
	connectBack := make([]byte, 10)
	_, err = io.ReadFull(socksConn, connectBack)
	if err!= nil {
		log.Println(err)
		return err
	}
	return nil;
}