package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/vishalkuo/bimap"
	"io"
	"log"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"xSocks/comm"
	"xSocks/comm/socks"
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

var udpNat sync.Map
var remoteUdpNat sync.Map
/*这里得保持socks5协议兼容*/
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
	//隧道
	udpTunnel:=UdpTunnel{}
	udpTunnel.natTable=bimap.NewBiMap();
	udpTunnel.udpGate=udpListener;
	go udpTunnel.recv()

	go func() {
		for {
			n, localAddr, err := udpListener.ReadFromUDP(buf[0:])
			if err != nil {
				break;
			}
			data := buf[0:n]
			dstAddr,dataStart,err:= socks.UdpHeadDecode(data);
			if err!=nil||dstAddr==nil {
				continue;
			}
			//判断地址是否合法
			_address := net.ParseIP(dstAddr.IP.String())
			if _address==nil {
				continue;
			}
			//本地转发
			if (!comm.IsPublicIP(dstAddr.IP) || comm.IsChinaMainlandIP(dstAddr.IP.String()))&&(runtime.GOOS!="windows"||param.TunType!=1) {
				socksNatSawp(udpListener,data,dataStart,localAddr,dstAddr);
			} else{
				udpTunnel.sendRemote(data,localAddr)
			}
		}
	}()
	return udpAddr,nil;
}



type UdpTunnel struct {
	Tunnel comm.CommConn
	natTable  *bimap.BiMap
	udpGate *net.UDPConn
	sync.Mutex
}
func (rd *UdpTunnel) GetTunnel()(comm.CommConn){
	rd.Lock();
	defer rd.Unlock();
	return rd.Tunnel;
}
func (rd *UdpTunnel) PutTunnel(tunnel comm.CommConn){
	rd.Lock();
	defer rd.Unlock();
	rd.Tunnel=tunnel;
}

func (ut *UdpTunnel)Connect() (comm.CommConn,error){
	sendBuf:=[]byte{};
	//cmd
	sendBuf =append(sendBuf,0x04);//dns
	var err error;
	tunnel, err:= NewTunnel();
	if err!=nil {
		return nil,err;
	}
	_,err=tunnel.Write(sendBuf)
	if err!=nil {
		return nil,err;
	}
	return tunnel,nil;
}
/*收到的还是socks5 udp协议,转发到远程是自定义的nat协议*/
func (ut *UdpTunnel)sendRemote(data []byte,localAddr *net.UDPAddr) (error){
	var err error
	dstAddr,dataStart,err:= socks.UdpHeadDecode(data);
	if err!=nil||dstAddr==nil {
		return err;
	}

	tunnel:=ut.GetTunnel();
	if tunnel==nil {
		fmt.Printf("sendRemote Tunnel null connect\r\n")
		tunnel,err := ut.Connect();
		if err != nil {
			fmt.Printf("sendRemote1\r\n")
			return err
		}
		ut.PutTunnel(tunnel);
	}

	var packLenByte []byte = make([]byte, 2)
	var buffer bytes.Buffer
	binary.LittleEndian.PutUint16(packLenByte, uint16(len(data)))
	buffer.Reset()
	buffer.Write(packLenByte)
	buffer.Write(comm.UdpNatEncode(localAddr,dstAddr))
	buffer.Write(data[dataStart:])
	sendBuf:=buffer.Bytes()
	tunnel=ut.GetTunnel();
	if tunnel!=nil {
		_,err=tunnel.Write(buffer.Bytes())
		if err != nil {
			log.Printf("tunnel wrtie err:%v\r\n", err)
		}
	}
	//失败重新连接
	if err != nil {
		fmt.Printf("sendRemote-2\r\n")
		tunnel,err1 := ut.Connect();
		if err1!=nil {
			fmt.Printf("sendRemote2\r\n")
			return err1
		}
		ut.PutTunnel(tunnel);
		_, err = tunnel.Write(sendBuf)
		if err!=nil {
			fmt.Printf("sendRemote3\r\n")
			return err
		}
	}
	return nil;
}

func (ut *UdpTunnel) recv(){
	var packLenByte []byte = make([]byte, 2)
	var bufByte []byte = make([]byte,65535)
	var tunnel comm.CommConn
	var buffer bytes.Buffer
	for {
		tunnel=ut.GetTunnel();
		if tunnel==nil {
			_tunnel,err:=ut.Connect();
			if err==nil {
				ut.PutTunnel(_tunnel)
			}else {
				time.Sleep(10 * time.Second);
				fmt.Printf("re TunStream 3 e:%v\r\n", err)
			}
			continue;
		}
		//remoteConn.SetDeadline();
		tunnel.SetDeadline(time.Now().Add(3*time.Minute))
		_, err := io.ReadFull(tunnel, packLenByte)
		packLen := binary.LittleEndian.Uint16(packLenByte)
		if err != nil||int(packLen)>len(bufByte) {
			log.Printf("err:%v\r\n",err);
			ut.PutTunnel(nil)
			continue;
		}
		tunnel.SetDeadline(time.Now().Add(3*time.Minute))
		_, err = io.ReadFull(tunnel, bufByte[:int(packLen)])
		if err != nil {
			log.Printf("err:%v\r\n",err);
			ut.PutTunnel(nil)
			continue;
		}else {
			localAddr,dstAddr,err:=comm.UdpNatDecode(bufByte[:int(packLen)]);
			if err!=nil||localAddr==nil {
				continue;
			}

			buffer.Reset();
			buffer.Write(socks.UdpHeadEncode(dstAddr))
			buffer.Write(bufByte[11:int(packLen)])
			_, err = ut.udpGate.WriteToUDP(buffer.Bytes(), localAddr)
			if err != nil {
				log.Printf("err:%v\r\n", err);
			}
		}
	}
}



/*udp socks5 nat sawp*/
func socksNatSawp(udpGate *net.UDPConn,data []byte,dataStart int,localAddr *net.UDPAddr, dstAddr *net.UDPAddr){
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
		go func() {
			defer udpNat.Delete(natKey);
			defer remoteConn.Close()
			for {
				//remoteConn.SetDeadline();
				remoteConn.SetReadDeadline(time.Now().Add(5*time.Minute))
				n, err:= remoteConn.Read(buf);
				if err!=nil {
					log.Printf("err:%v\r\n",err);
					return ;
				}
				buffer.Reset();
				buffer.Write(socks.UdpHeadEncode(dstAddr))
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
	clientConn.SetDeadline(time.Now().Add(time.Second*20))
	defer clientConn.Close()

	auth:= make([]byte,3)
	_, err := io.ReadFull(clientConn, auth)
	if err != nil {
		log.Printf("err:%v",err);
		return err
	}
	if auth[0]==0x05 {
		//resp auth
		clientConn.Write([]byte{0x05, 0x00})
	}

	connectHead:= make([]byte,4)
	_, err = io.ReadFull(clientConn, connectHead)
	if err != nil {
		log.Printf("err:%v",err);
		return err
	}


	if connectHead[0]==0x05 {
		//connect tcp
		if connectHead[1]==0x01 {
			var ipAddr net.IP
			var port string
			var hostBuf []byte;
			var hostBufLen []byte
			ipv4 := make([]byte, 4)
			ipv6 := make([]byte, 16)
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
				if !ok {
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
				_, err = io.ReadFull(clientConn, ipv6)
				ipAddr = net.IP{ipv6[0], ipv6[1], ipv6[2], ipv6[3], ipv6[4], ipv6[5], ipv6[6], ipv6[7], ipv6[8], ipv6[9], ipv6[10], ipv6[11], ipv6[12], ipv6[13], ipv6[14], ipv6[15]}

				break;
			default:
				fmt.Printf("default connectHead[3]:%v\r\n", connectHead[3])
				break;
			}

			//解析失败直接关闭
			if ipAddr==nil||ipAddr.String()=="0.0.0.0" {
				return nil;
			}

			portBuf := make([]byte, 2)
			_, err = io.ReadFull(clientConn, portBuf)
			port = strconv.Itoa(int(portBuf[0])<<8 | int(portBuf[1]))


			//如果是内网IP,或者是中国IP(如果被污染的IP一定返回的是国外IP地址ChinaDNS也是这个原理)
			if (!comm.IsPublicIP(ipAddr) || comm.IsChinaMainlandIP(ipAddr.String()))&&runtime.GOOS!="windows" {
				server, err := net.DialTimeout("tcp", net.JoinHostPort(ipAddr.String(), port),param.ConnectTime)
				if err != nil {
					log.Printf("host:%s err:%v\r\n", net.JoinHostPort(ipAddr.String(), port),err);
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
				var stream,err=NewTunnel();
				if err != nil || stream == nil {
					log.Printf("err:%v\r\n",err);
					return err
				}
				var buffer bytes.Buffer
				defer stream.Close()
				buffer.Reset()
				buffer.WriteByte(0x02)//cmd 0x02 to socks5
				buffer.Write(auth)//写入sock5认证头
				buffer.Write(connectHead)//写入sock5请求head


				//使用host
				if connectHead[3] == 0x03 {
					buffer.Write(hostBufLen)
					buffer.Write(hostBuf)
				}else if connectHead[3] == 0x01 {
					buffer.Write(ipv4)
				} else if connectHead[3] == 0x04 {
					buffer.Write(ipv6)
				}
				//写入端口
				buffer.Write(portBuf)
				stream.SetDeadline(time.Now().Add(time.Second*45))
				_, err =stream.Write(buffer.Bytes());
				if err != nil {
					if strings.Contains(err.Error(),"deadline"){
						ResetTunnel();
					}
					fmt.Printf("read remote error err:%v errStr:%s\r\n ",err)
					return err
				}
				//read auth back
				socks5AuthBack := make([]byte, 2)
				stream.SetDeadline(time.Now().Add(time.Second*45))
				_, err = io.ReadFull(stream, socks5AuthBack)
				if err != nil {
					if strings.Contains(err.Error(),"deadline"){
						ResetTunnel()
					}
					fmt.Printf("read remote error err:%v\r\n ",err)
					return err
				}
				comm.TcpPipe(stream,clientConn,time.Minute*10);
			}
		}
		//UDP  代理
		if connectHead[1]==0x03 {
			//toLocalUdp(clientConn);
			socks.UdpProxyRes(clientConn,udpAddr);
		}
	}
	return nil;
}

