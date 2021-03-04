package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"context"
	"sync"
	"time"
	"xSocks/comm"
	"xSocks/param"
)



func StartRemoteSocks51(address string) {
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
	param.Sock5UdpPort=fmt.Sprintf("%d",udpAddr.Port);
	for {
		client, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}
		go handleRemoteRequest(client,udpAddr)
	}
}

func startUdpProxy(address string) ( *net.UDPAddr ,error){
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil,err
	}
	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil,err
	}
	buf := make([]byte, 65535)
	var udpNat sync.Map
	go func() {
		for {
			n, localAddr, err := udpListener.ReadFromUDP(buf[0:])
			if err != nil {
				break;
			}
			data := buf[0:n]
			dstAddr,dataStart,err:=comm.UdpHeadDecode(data);
			if err!=nil||dstAddr==nil {
				continue;
			}
			natSawp(udpListener,udpNat,data,dataStart,localAddr,dstAddr);
		}
	}()
	return udpAddr,nil;
}

/*udp nat sawp*/
func natSawp(udpGate *net.UDPConn,udpNat sync.Map,data []byte,dataStart int,localAddr *net.UDPAddr, dstAddr *net.UDPAddr){
	fmt.Printf("natSawp localAddr:%v dstAddr:%v\r\n",localAddr,dstAddr)
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
		go func(_remoteConn net.Conn) {
			defer udpNat.Delete(natKey);
			defer _remoteConn.Close()
			for {
				_remoteConn.SetReadDeadline(time.Now().Add(60*2*time.Second))
				n, err:= _remoteConn.Read(buf);
				if err!=nil {
					log.Printf("err:%v\r\n",err);
					return ;
				}
				buffer.Reset();
				buffer.Write(comm.UdpHeadEncode(dstAddr))
				buffer.Write(buf[:n])
				udpGate.WriteToUDP(buffer.Bytes(), localAddr)
			}
		}(remoteConn)
	}else{
		remoteConn=_conn.(net.Conn)
	}
	remoteConn.Write(data[dataStart:])
}




/*remote use*/
func handleRemoteRequest(clientConn net.Conn,udpAddr *net.UDPAddr) {
	if clientConn == nil {
		return
	}
	clientConn.SetDeadline(time.Now().Add(time.Second*59))
	defer clientConn.Close()
	auth:= make([]byte,3)
	_, err := io.ReadFull(clientConn, auth)
	if err != nil {
		log.Printf("err:%v\r\n",err);
		return
	}
	if auth[0]==0x05 {
		//resp auth
		clientConn.Write([]byte{0x05, 0x00})
	}else{
		log.Printf("auth[0]!=0x05\r\n");
		return
	}
	connectHead:= make([]byte,4)
	_, err = io.ReadFull(clientConn, connectHead)
	if err != nil {
		log.Printf("err:%v\r\n",err);
		return
	}

	if connectHead[0]==0x05 {

		if connectHead[1]==0x01 {
			var host, port string
			switch connectHead[3] {
			case 0x01: //IP V4
				ipv4 := make([]byte, 4)
				_, err = io.ReadFull(clientConn, ipv4)
				host = net.IPv4(ipv4[0], ipv4[1], ipv4[2], ipv4[3]).String()
				break;
			case 0x03: //域名
				hostLen := make([]byte, 1)
				_, err = io.ReadFull(clientConn, hostLen)
				hostBuf := make([]byte, hostLen[0])
				_, err = io.ReadFull(clientConn, hostBuf)
				host = string(hostBuf) //b[4]表示域名的长度
				fmt.Printf("host:%s\r\n",hostBuf);
				break;
			case 0x04: //IP V6
				fmt.Printf("ipv6\r\n");
				ipv6 := make([]byte, 16)
				_, err = io.ReadFull(clientConn, ipv6)
				host = net.IP{ipv6[0], ipv6[1], ipv6[2], ipv6[3], ipv6[4], ipv6[5], ipv6[6], ipv6[7], ipv6[8], ipv6[9], ipv6[10], ipv6[11], ipv6[12], ipv6[13], ipv6[14], ipv6[15]}.String()
				break;
			}
			if len(host)==0 {
				log.Println("host null\r\n");
				return
			}


			portBuf := make([]byte, 2)
			_, err = io.ReadFull(clientConn, portBuf)
			port = strconv.Itoa(int(portBuf[0])<<8 | int(portBuf[1]))
			server, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), param.ConnectTime)
			if err != nil {
				log.Println(err)
				return
			}
			defer server.Close()
			clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) //响应客户端连接成功
			//进行转发
			comm.TcpPipe(server,clientConn,time.Minute*10)
		}
		//udp
		if connectHead[1]==0x03 {
			comm.UdpProxyRes(clientConn,udpAddr);
		}
	}
}

/*single user test*/
func startUdpGate() ( *net.UDPAddr ,error){
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return nil,err
	}
	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil,err
	}
	buf := make([]byte, 65535)
	var gateNat sync.Map
	var gateNatTime sync.Map
	var buffer bytes.Buffer
	var lastTime=time.Now().Unix();

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(_ctx context.Context){
		ticker := time.NewTicker(time.Second * 60)
		defer ticker.Stop();
		for {
			gateNatTime.Range(func(_k, _v interface{}) bool {
				lastTime := _v.(int64)
				if lastTime+600 < time.Now().Unix() {
					gateNat.Delete(_k)
					gateNatTime.Delete(_k)
				}
				return true
			})
			select {
				case  <-ticker.C:
					fmt.Println("d")
				case <-_ctx.Done():
					return
					break;
			}
		}
	}(ctx)

	go func() {
		for {
			udpListener.SetDeadline(time.Now().Add(30*time.Second))
			n, recvAddr, err := udpListener.ReadFromUDP(buf[0:])
			if err != nil {
				continue;
			}
			if lastTime+60*10<time.Now().Unix() {
				break;
			}

			lastTime=time.Now().Unix();

			data := buf[0:n]
			_udpAddr,ok:=gateNat.Load(recvAddr.String())
			//client to remote
			if !ok{
				var dstAddr *net.UDPAddr
				dstAddr,dataStart,err:=comm.UdpHeadDecode(data);
				if err!=nil||dstAddr==nil {
					continue;
				}
				gateNat.Store(dstAddr.String(),recvAddr)
				gateNatTime.Store(dstAddr.String(),time.Now().Unix())
				udpListener.WriteTo(data[dataStart:],dstAddr)
			}else{
				buffer.Reset()
				buffer.Write(comm.UdpHeadEncode(recvAddr))
				buffer.Write(data)
				//remote to client
				udpListener.WriteTo(buffer.Bytes(),_udpAddr.(*net.UDPAddr))
			}
		}
	}()
	return udpAddr,nil;
}
