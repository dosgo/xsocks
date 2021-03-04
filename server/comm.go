package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
	"xSocks/comm"
	"xSocks/param"
)



/*共享内存避免GC*/
var poolAuthHeadBuf = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 16)
	},
}
/*save uniqueId Tun */
var uniqueIdTun sync.Map

func proxy(conn comm.CommConn){
	defer conn.Close()
	//read auth Head
	var authHead = poolAuthHeadBuf.Get().([]byte)
	defer poolAuthHeadBuf.Put(authHead)
	_, err := io.ReadFull(conn, authHead[:16])
	if err != nil {
		log.Printf("err:%v\r\n",err);
		return
	}
	//autherr;
	if string(authHead)!= comm.GenPasswordHead(param.Password) {
		fmt.Printf("password err\r\n");
		return ;
	}
	//read cmd
	cmd := make([]byte,1)
	_, err = io.ReadFull(conn, cmd)
	if err != nil {
		fmt.Printf("err:%v\r\n",err)
		return
	}
	switch cmd[0] {
		//dns
		case 0x01:
			dnsResolve(conn);
			break
		//to socks5
		case 0x02:
			//连接socks5
			sConn, err := net.DialTimeout("tcp", "127.0.0.1:"+param.Sock5Port,param.ConnectTime)
			if(err!=nil){
				log.Printf("err:%v\r\n",param.Sock5Port)
				return ;
			}
			defer sConn.Close();
			//交换数据
			comm.TcpPipe(sConn,conn,time.Minute*10)
			break;
		//to tun
		case 0x03:
			tcpToTun(conn)
			break;
			//to udp socket
		case 0x04:
			tcpToUdpProxy(conn);
			break;
	}
}

/*转发到本地的udp网关*/
func tcpToUdpProxy(conn comm.CommConn){
	var packLenByte []byte = make([]byte, 2)
	var bufByte []byte = make([]byte,65535)
	remoteConn, err := net.DialTimeout("udp", "127.0.0.1:"+param.Sock5UdpPort,time.Second*15);
	if err!=nil {
		log.Printf("err:%v\r\n",err);
		return
	}
	defer remoteConn.Close()

	go func() {
		var bufByte1 []byte = make([]byte,65535)
		var buffer bytes.Buffer
		var packLenByte []byte = make([]byte, 2)
		for {
			remoteConn.SetDeadline(time.Now().Add(60*10*time.Second))
			n, err := remoteConn.Read(bufByte1)
			if err != nil {
				log.Printf("err:%v\r\n",err);
				break;
			}
			buffer.Reset()
			binary.LittleEndian.PutUint16(packLenByte, uint16(n))
			buffer.Write(packLenByte)
			buffer.Write(bufByte1[:n])
			//remote to client
			conn.Write(buffer.Bytes())
		}
	}();

	for {
		//remoteConn.SetDeadline();
		conn.SetDeadline(time.Now().Add(60*10*time.Second))
		_, err := io.ReadFull(conn, packLenByte)
		packLen := binary.LittleEndian.Uint16(packLenByte)
		if err != nil||int(packLen)>len(bufByte) {
			log.Printf("err:%v\r\n",err);
			break;
		}
		conn.SetDeadline(time.Now().Add(60*10*time.Second))
		_, err = io.ReadFull(conn, bufByte[:int(packLen)])
		if (err != nil) {
			log.Printf("err:%v\r\n",err);
			break;
		}else {
			_, err = remoteConn.Write(bufByte[:int(packLen)])
			if (err != nil) {
				log.Printf("err:%v\r\n",err);
			}
		}
	}
}

/*to tun 处理*/
func tcpToTun(conn comm.CommConn){
	uniqueIdByte := make([]byte,8)
	_, err := io.ReadFull(conn, uniqueIdByte)
	if err!=nil {
		log.Printf("err:%v\r\n",err)
		return ;
	}
	uniqueId:=string(uniqueIdByte)
	fmt.Printf("uniqueId:%s\r\n",uniqueId)
	var mtuByte []byte = make([]byte, 2)
	//read Mtu
	_, err = io.ReadFull(conn, mtuByte)
	if err!=nil {
		log.Printf("err:%v\r\n")
		return ;
	}
	mtu := binary.LittleEndian.Uint16(mtuByte)
	if mtu<1 {
		mtu=1024;
	}
	_stack,channelLinkID,err:=StartTunStack(mtu);
	if err!=nil {
		return;
	}
	defer _stack.Close();
	var buffer =new(bytes.Buffer)
	defer fmt.Printf("channelLinkID recv exit \r\n");
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		var sendBuffer =new(bytes.Buffer)
		var packLenByte []byte = make([]byte, 2)
		for {
			pkt,res :=channelLinkID.ReadContext(ctx)
			if(!res){
				break;
			}
			buffer.Reset()
			buffer.Write(pkt.Pkt.NetworkHeader().View())
			buffer.Write(pkt.Pkt.TransportHeader().View())
			buffer.Write(pkt.Pkt.Data.ToView())
			if(buffer.Len()>0) {
				binary.LittleEndian.PutUint16(packLenByte,uint16(buffer.Len()))
				sendBuffer.Reset()
				sendBuffer.Write(packLenByte)
				sendBuffer.Write(buffer.Bytes())
				_,err=conn.Write(sendBuffer.Bytes())
				if(err!=nil){
					return ;
				}
			}
		}

	}()
	var buflen=mtu+80;
	var buf=make([]byte,buflen)
	var packLenByte []byte = make([]byte, 2)
	for {
		conn.SetDeadline(time.Now().Add(time.Minute*3))
		_, err := io.ReadFull(conn, packLenByte)
		if (err != nil) {
			log.Printf("err:%v\r\n", err)
			return;
		}
		packLen := binary.LittleEndian.Uint16(packLenByte)
		//null
		if (packLen < 1 || packLen > buflen) {
			continue;
		}
		conn.SetDeadline(time.Now().Add(time.Minute*3))
		n, err := io.ReadFull(conn, buf[:int(packLen)])
		if (err != nil) {
			log.Printf("err:%v\r\n", err)
			return;
		}
		InjectInbound(channelLinkID,buf[:n])
	}
}



/*dns解析*/
func dnsResolve(conn comm.CommConn) {
	hostLenBuf := make([]byte,1)
	var hostBuf =  make([]byte,1024)
	var hostLen int;
	var err error
	for{
		_, err = io.ReadFull(conn, hostLenBuf)
		if err != nil {
			return
		}
		hostLen=int(hostLenBuf[0])
		_, err = io.ReadFull(conn, hostBuf[:hostLen])
		if err != nil {
			fmt.Printf("hostLen:%d\r\n",hostLen)
			return
		}
		addr, err := net.ResolveIPAddr("ip4", string(hostBuf[:hostLen]))
		if(err!=nil){
			fmt.Printf("host:%s hostLen:%d\r\n",string(hostBuf[:hostLen]),hostLen)
			//err
			conn.Write([]byte{0x01, 0x04}) //0x01==error  0x04==ipv4
			continue;//解析失败跳过不关闭连接
		}
		_, err =conn.Write([]byte{0x00, 0x04}) //响应客户端
		_, err =conn.Write(addr.IP.To4()) //响应客户端
		if(err!=nil){
			return ;
		}
	}
}
func GetPublicIP() (ip string, err error) {
	var (
		addrs   []net.Addr
		addr    net.Addr
		ipNet   *net.IPNet // IP地址
		isIpNet bool
	)
	// 获取所有网卡
	if addrs, err = net.InterfaceAddrs(); err != nil {
		return
	}
	//取IP
	for _, addr = range addrs {
		// 这个网络地址是IP地址: ipv4, ipv6
		if ipNet, isIpNet = addr.(*net.IPNet); isIpNet && !ipNet.IP.IsLoopback() {
			//
			if(ipNet.IP.To4() != nil){
				if(comm.IsPublicIP(ipNet.IP)){
					ip = ipNet.IP.String()
					return ;
				}
			}
		}
	}
	err = errors.New("no public ip")
	return
}



