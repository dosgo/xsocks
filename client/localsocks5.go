package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"runtime"
	"strconv"
	"sync"
	"time"
	"xSocks/comm"
	"xSocks/param"
)



func StartLocalSocks5(address string) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Panic(err)
	}

	//start udpProxy
	udpAddr,err:=startUdpProxy("127.0.0.1:"+param.Sock5UdpPort);
	if err != nil {
		log.Panic(err)
	}
	for {
		client, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}
		go handleLocalRequest(client,udpAddr)
	}
}


func startUdpProxy(addr string) ( *net.UDPAddr ,error){
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil,err
	}
	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil,err
	}
	buf := make([]byte, 65535)
	var udpNat sync.Map
	var remoteUdpNat sync.Map
	go func() {
		for {
			n, localAddr, err := udpListener.ReadFromUDP(buf[0:])
			if err != nil {
				break;
			}
			data := buf[0:n]
			dstAddr,dataStart,err:=comm.UdpHeadDecode(data);
			if(err!=nil||dstAddr==nil){
				continue;
			}
			//本地转发
			if ((!comm.IsPublicIP(dstAddr.IP) || comm.IsChinaMainlandIP(dstAddr.IP.String()))&&(runtime.GOOS!="windows"||param.TunType!=1)) {
				natSawp(udpListener,udpNat,data,dataStart,localAddr,dstAddr);
			} else{
				remoteUdpProxy(udpListener,data,remoteUdpNat,localAddr);
			}
		}
	}()
	return udpAddr,nil;
}


func remoteUdpProxy(udpGate *net.UDPConn,data []byte, remoteUdpNat sync.Map,localAddr *net.UDPAddr) error{
	fmt.Printf("remoteUdpProxy local addr :%v\r\n",localAddr);
	natKey:=localAddr.String()
	var tunnel comm.CommConn
	_conn,ok:=remoteUdpNat.Load(natKey)
	if !ok{
		sendBuf:=[]byte{};
		//cmd
		sendBuf =append(sendBuf,0x04);//dns\
		var err error;
		tunnel, err= NewTunnel();
		if(err!=nil){
			log.Printf("err:%v\r\n",err);
			return err;
		}
		_,err=tunnel.Write(sendBuf)
		if(err!=nil){
			log.Printf("err:%v\r\n",err);
			return err;
		}
		go func() {
			remoteUdpNat.Store(natKey,tunnel)
			defer remoteUdpNat.Delete(natKey);
			defer tunnel.Close()
			var packLenByte []byte = make([]byte, 2)
			var bufByte []byte = make([]byte,65535)
			for {
				//remoteConn.SetDeadline();
				tunnel.SetDeadline(time.Now().Add(60*10*time.Second))
				_, err := io.ReadFull(tunnel, packLenByte)
				packLen := binary.LittleEndian.Uint16(packLenByte)
				if (err != nil||int(packLen)>len(bufByte)) {
					log.Printf("err:%v\r\n",err);
					break;
				}
				tunnel.SetDeadline(time.Now().Add(300*time.Second))
				_, err = io.ReadFull(tunnel, bufByte[:int(packLen)])
				if (err != nil) {
					log.Printf("err:%v\r\n",err);
					break;
				}else {
					_, err = udpGate.WriteToUDP(bufByte[:int(packLen)],localAddr)
					if (err != nil) {
						log.Printf("err:%v\r\n",err);
					}
				}
			}
		}()
	}else{
		tunnel=_conn.(net.Conn)
	}
	var packLenByte []byte = make([]byte, 2)
	var buffer bytes.Buffer
	//fmt.Printf("dev read len:%d\r\n",n);
	binary.LittleEndian.PutUint16(packLenByte, uint16(len(data)))
	buffer.Reset()
	buffer.Write(packLenByte)
	buffer.Write(data)
	tunnel.Write(buffer.Bytes())
	return nil;
}



/*udp nat sawp*/
func natSawp(udpGate *net.UDPConn,udpNat sync.Map,data []byte,dataStart int,localAddr *net.UDPAddr, dstAddr *net.UDPAddr){
	natKey:=localAddr.String()+"_"+dstAddr.String()
	var remoteConn net.Conn
	var err error
	_conn,ok:=udpNat.Load(natKey)
	if !ok{
		remoteConn, err = net.DialTimeout("udp", dstAddr.String(),time.Second*15);
		if err != nil {
			return
		}
		buf:= make([]byte, 65535)
		var buffer bytes.Buffer
		udpNat.Store(natKey,remoteConn)
		defer udpNat.Delete(natKey);
		go func() {
			defer remoteConn.Close()
			for {
				//remoteConn.SetDeadline();
				remoteConn.SetReadDeadline(time.Now().Add(60*10*time.Second))
				n, err:= remoteConn.Read(buf);
				if(err!=nil){
					log.Printf("err:%v\r\n",err);
					return ;
				}
				buffer.Reset();
				buffer.Write(comm.UdpHeadEncode(dstAddr))
				buffer.Write(buf[:n])
				udpGate.WriteToUDP(buffer.Bytes(), localAddr)
			}
		}()
	}else{
		remoteConn=_conn.(net.Conn)
	}
	remoteConn.Write(data[dataStart:])
}



/*local use  smart dns*/
func handleLocalRequest(clientConn net.Conn,udpAddr *net.UDPAddr ) error {
	if clientConn == nil {
		return nil
	}
	defer clientConn.Close()

	auth:= make([]byte,3)
	_, err := io.ReadFull(clientConn, auth)
	if err != nil {
		return err
	}
	if(auth[0]==0x05){
		//resp auth
		clientConn.Write([]byte{0x05, 0x00})
	}

	connectHead:= make([]byte,4)
	_, err = io.ReadFull(clientConn, connectHead)
	if err != nil {
		return err
	}


	if(connectHead[0]==0x05) {
		//connect tcp
		if(connectHead[1]==0x01) {
			var ipAddr net.IP
			var port string
			var hostBuf []byte;
			var hostBufLen []byte
			ipv4 := make([]byte, 4)

			//解析
			switch connectHead[3] {
			case 0x01: //IP V4
				_, err = io.ReadFull(clientConn, ipv4)
				ipAddr = net.IPv4(ipv4[0], ipv4[1], ipv4[2], ipv4[3])
				break;
			case 0x03: //域名
				hostBufLen = make([]byte, 1)
				_, err = io.ReadFull(clientConn, hostBufLen)
				hostBuf = make([]byte, hostBufLen[0])
				_, err = io.ReadFull(clientConn, hostBuf)
				ip := "8.8.8.8"; //随便一个国外的IP地址
				//如果在列表無需解析，直接用遠程
				_, ok := PolluteDomainName.Load(string(hostBuf))
				if (!ok) {
					addr, err := net.ResolveIPAddr("ip", string(hostBuf))
					if err == nil {
						ip = addr.String();
					}else{
						fmt.Printf("dnserr host:%s  addr:%s err:%v\r\n", string(hostBuf), addr.String(), err)
					}
				}
				ipAddr = net.ParseIP(ip)
				break;
			case 0x04: //IP V6
				fmt.Printf("ipv6\r\n");
				ipv6 := make([]byte, 16)
				_, err = io.ReadFull(clientConn, ipv6)
				ipAddr = net.IP{ipv6[0], ipv6[1], ipv6[2], ipv6[3], ipv6[4], ipv6[5], ipv6[6], ipv6[7], ipv6[8], ipv6[9], ipv6[10], ipv6[11], ipv6[12], ipv6[13], ipv6[14], ipv6[15]}

				break;
			default:
				fmt.Printf("default connectHead[3]:%v\r\n", connectHead[3])
				break;
			}

			portBuf := make([]byte, 2)
			_, err = io.ReadFull(clientConn, portBuf)
			port = strconv.Itoa(int(portBuf[0])<<8 | int(portBuf[1]))

			//解析失败直接关闭
			if(ipAddr==nil||ipAddr.String()=="0.0.0.0"){
				return nil;
			}
			//如果是内网IP,或者是中国IP(如果被污染的IP一定返回的是国外IP地址ChinaDNS也是这个原理)
			if ((!comm.IsPublicIP(ipAddr) || comm.IsChinaMainlandIP(ipAddr.String()))&&(runtime.GOOS!="windows"||param.TunType!=1)) {
				server, err := net.DialTimeout("tcp", net.JoinHostPort(ipAddr.String(), port),param.ConnectTime)
				if err != nil {
					fmt.Println(ipAddr.String(), connectHead[3], err)
					log.Printf("err:%v\r\n",err);
					return err
				}
				defer server.Close()
				clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) //响应客户端连接成功
				//进行转发
				comm.TcpPipe(server,clientConn,time.Minute*5)
				return nil;
			} else {
				//保存記錄
				PolluteDomainName.Store(string(hostBuf), 1)
				fmt.Printf("remote addr:%s port:%s\r\n", ipAddr.String(),port)
				var stream,err=NewTunnel();

				if err != nil || stream == nil {
					fmt.Printf("streamerror err:%v\r\n",err)
					return err
				}
				defer stream.Close()
				cmdBuf := make([]byte, 1)
				cmdBuf[0] = 0x02; //cmd 0x02 to socks5
				stream.Write(cmdBuf);
				//写入sock5认证头
				stream.Write(auth)
				//写入sock5请求head
				stream.Write(connectHead)
				//使用host
				if (connectHead[3] == 0x03) {
					stream.Write(hostBufLen)
					stream.Write(hostBuf)
				} else {
					binary.Write(stream, binary.BigEndian, ipAddr.To4())
				}
				//写入端口
				stream.Write(portBuf)

				//read auth back
				socks5AuthBack := make([]byte, 2)
				_, err = io.ReadFull(stream, socks5AuthBack)
				if err != nil {
					fmt.Printf("read remote error err:%v\r\n ",err)
					return err
				}
				comm.TcpPipe(stream,clientConn,time.Minute*10);
			}
		}
		//UDP  代理
		if connectHead[1]==0x03 {
			//toLocalUdp(clientConn);
			comm.UdpProxyRes(clientConn,udpAddr);
		}
	}
	return nil;
}

