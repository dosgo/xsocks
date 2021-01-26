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
	for {
		client, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}
		go handleLocalRequest(client)
	}
}


/*local use  smart dns*/
func handleLocalRequest(clientConn net.Conn) error {
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
				go io.Copy(stream, clientConn)
				io.Copy(clientConn, stream)
			}
		}
		//UDP  代理
		if connectHead[1]==0x03 {
			toLocalUdp(clientConn);
		}
	}
	return nil;
}
/*发到远程udp处理*/
func ToRemoteUdp(clientConn net.Conn,auth []byte,connectHead []byte) error{
	//解析
	var stream,err=NewTunnel();
	if(err!=nil){
		return err;
	}
	defer stream.Close()
	cmdBuf := make([]byte, 1)
	cmdBuf[0] = 0x02; //cmd 0x02 to socks5
	stream.Write(cmdBuf);
	//写入sock5认证头
	stream.Write(auth)
	//获取认证丢弃
	socks5AuthBack := make([]byte, 2)
	_, err = io.ReadFull(stream, socks5AuthBack)
	if err != nil {
		return err
	}
	//写入sock5请求head
	stream.Write(connectHead)
	//获取请求的IP跟端口
	ip_port := make([]byte, 6)
	_, err = io.ReadFull(clientConn, ip_port)
	//写入请求IP+port
	stream.Write(ip_port)


	//获取返回的服务器IP跟端口
	tmpBuf:= make([]byte, 10)
	_, err = io.ReadFull(stream, tmpBuf)
	if err != nil {
		return err
	}
	fmt.Printf("tmpBuf:%v\r\n",tmpBuf)
	ipAddr := net.IPv4(tmpBuf[4], tmpBuf[5], tmpBuf[6], tmpBuf[7])
	port := strconv.Itoa(int(tmpBuf[8])<<8 | int(tmpBuf[9]))
	fmt.Printf("listen :%s--%s\r\n",ipAddr,port)
	clientConn.Write(tmpBuf);
	return nil;
}

/*udp to */
func toLocalUdp(clientConn net.Conn )  error{
	//
	temp := make([]byte, 6)
	_, err := io.ReadFull(clientConn, temp)
	if(err!=nil){
		return err;
	}

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	defer udpListener.Close()

	udpListAddr, err := net.ResolveUDPAddr("udp", udpListener.LocalAddr().String())
	bindPort := udpListAddr.Port

	//版本 | 代理的应答 |　保留1字节　| 地址类型 | 代理服务器地址 | 绑定的代理端口
	bindMsg := []byte{0x05, 0x00, 0x00, 0x01}
	buffer := bytes.NewBuffer(bindMsg)
	binary.Write(buffer, binary.BigEndian, udpAddr.IP.To4())
	binary.Write(buffer, binary.BigEndian, uint16(bindPort))
	clientConn.Write(buffer.Bytes())

	buf := make([]byte, 2048)
	buf2 := make([]byte, 2048)
	for {
		n, udpAddr, err := udpListener.ReadFromUDP(buf[0:])
		if err != nil {
			return err
		}
		b := buf[0:n]
		/*
		   +----+------+------+----------+----------+----------+
		   |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
		   +----+------+------+----------+----------+----------+
		   |  2 |   1  |   1  | Variable |     2    | Variable |
		   +----+------+------+----------+----------+----------+
		*/
		if b[2] != 0x00 {
			fmt.Printf(" WARN: FRAG do not support.\n")
			continue
		}

		switch b[3] {
		case 0x01: //ipv4
			dstAddr := &net.UDPAddr{
				IP:   net.IPv4(b[4], b[5], b[6], b[7]),
				Port: int(b[8])*256 + int(b[9]),
			}
			//dns
			if(dstAddr.Port==53){
				addr:=fmt.Sprintf("%s:%d",dstAddr.IP,dstAddr.Port);
				fmt.Printf("addr:%s\r\n",addr)
				var conn net.Conn
				if conn, err = net.Dial("udp", addr); err != nil {
					fmt.Println(err.Error())
				}
				conn.Write(b[10:])
				rlen,_:=conn.Read(buf2);
				sendBuf:=[]byte{};
				sendBuf =append(sendBuf,b[0:10]...);//dns
				sendBuf =append(sendBuf,buf2[0:rlen]...);//dns
				udpListener.WriteToUDP(sendBuf,udpAddr)
				defer conn.Close()

			}
		}

	}
}