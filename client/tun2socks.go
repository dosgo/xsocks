package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"xSocks/comm"

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
		/*
		laddr, err := net.ResolveUnixAddr("unixgram",param.UnixSockTun)
		if err != nil {
			return ;
		}

		conn, err := net.ListenUnixgram("unixgram", laddr)
		if err != nil {                      //如果监听失败，一般是文件已存在，需要删除它
			log.Println("UNIX Domain Socket 创 建失败，正在尝试重新创建 -> ", err)
			os.Remove(param.UnixSockTun)
			return ;
		}
		dev=conn;

		 */
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
	_stack:=comm.NewNetStack();
	defer _stack.Close();
	channelLinkID,err:=comm.GenChannelLinkID(_stack,mtu,tcpForwarder,udpForwarder);
	if(err!=nil){
		return err;
	}
	// write tun
	go func() {
		var buffer =new(bytes.Buffer)
		for {
			select {
			case pkt := <-channelLinkID.C:
				buffer.Write(pkt.Pkt.Header.View())
				buffer.Write(pkt.Pkt.Data.ToView())
				//tmpBuf:=append(pkt.Pkt.Header.View(),pkt.Pkt.Data.ToView()...)
				if(buffer.Len()>0) {
					dev.Write(buffer.Bytes())
					buffer.Reset()
				}
				break;
			}
		}
	}()


	// read tun data
	var buf=make([]byte,mtu+80)
	for {
		n, e := dev.Read(buf[:])
		if e != nil {
			fmt.Printf("e:%v\r\n",e)
			break;
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

func tcpForwarder(conn *gonet.Conn)error{
	var remoteAddr=conn.LocalAddr().String()
	//dns ,use 8.8.8.8
	if(strings.HasSuffix(remoteAddr,":53")){
		dnsReq(conn,"tcp");
		return  nil;
	}
	socksConn,err1:= net.Dial("tcp",param.Sock5Addr)
	if err1 != nil {
		log.Println(err1)
		return nil
	}
	defer socksConn.Close();
	if(socksCmd(socksConn,1,remoteAddr)==nil) {
		go io.Copy(conn, socksConn)
		io.Copy(socksConn, conn)
	}
	return nil
}

func udpForwarder(conn *gonet.Conn, ep tcpip.Endpoint)error{
	defer conn.Close();
	defer ep.Close();
	//dns port
	if(strings.HasSuffix(conn.LocalAddr().String(),":53")){
		dnsReq(conn,"udp");
	}
	return nil;
}

/*to dns*/
func dnsReq(conn *gonet.Conn,action string) error{
	if(action=="tcp"){
		dnsConn, err := net.Dial(action, "127.0.0.1:"+param.DnsPort);
		if err != nil {
			fmt.Println(err.Error())
			return err;
		}
		defer dnsConn.Close();
		go io.Copy(conn, dnsConn)
		io.Copy(dnsConn, conn)
		fmt.Printf("dnsReq Tcp\r\n");
		return nil;
	}else {
		var buf = poolDnsBuf.Get().([]byte)
		defer poolDnsBuf.Put(buf)
		var n = 0;
		var err error;
		n, err = conn.Read(buf)
		if err != nil {
			fmt.Printf("c.Read() = %v", err)
			return err;
		}
		dnsConn, err := net.Dial("udp", "127.0.0.1:"+param.DnsPort);
		if err != nil {
			fmt.Println(err.Error())
			return err;
		}
		defer dnsConn.Close();
		_, err = dnsConn.Write(buf[:n])
		if (err != nil) {
			fmt.Println(err.Error())
			return err;
		}
		n, err = dnsConn.Read(buf);
		if (err != nil) {
			fmt.Println(err.Error())
			return err;
		}
		_, err = conn.Write(buf[:n])
		if (err != nil) {
			fmt.Println(err.Error())
			return err;
		}
	}
	return nil;
}

/*to socks5*/
func socksCmd(socksConn net.Conn,cmd uint8,host string) error{
	//socks5 auth
	socksConn.Write([]byte{0x05, 0x01,0x00});
	authBack := make([]byte, 2)
	_, err:= io.ReadFull(socksConn, authBack)
	if err != nil {
		log.Println(err)
		return err
	}
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
	conectBack := make([]byte, 10)
	_, err = io.ReadFull(socksConn, conectBack)
	if err!= nil {
		log.Println(err)
		return err
	}
	return nil;
}