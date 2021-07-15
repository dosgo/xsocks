package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/dosgo/xsocks/client/tun"
	"github.com/dosgo/xsocks/client/tunnelcomm"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/comm/udpHeader"
	"github.com/dosgo/xsocks/param"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"io"
	"log"
	"net"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"
)


type RemoteTun struct {
	tunDev io.ReadWriteCloser
	remoteAddr string;
	dnsServers []string;
	oldGw string;
	tunGW string;
}


func (remoteTun *RemoteTun)Start(tunDevice string,tunAddr string,tunMask string,tunGW string,tunDNS string) error {
	remoteTun.oldGw=comm.GetGateway();
	remoteTun.tunGW=tunGW;
	var err error;
	if len(param.Args.UnixSockTun)>0 {
		remoteTun.tunDev,err=tun.UsocketToTun(param.Args.UnixSockTun)
		if err!=nil {
			return err;
		}
		go tunRecv(remoteTun.tunDev, param.Args.Mtu)
		return nil;
	}else{
		if runtime.GOOS=="windows" {
			urlInfo, _ := url.Parse(param.Args.ServerAddr)
			addr, err := net.ResolveIPAddr("ip",urlInfo.Hostname())
			if err == nil {
				remoteTun.remoteAddr = addr.String()
			}
			fmt.Printf("remoteAddr:%s\r\n", remoteTun.remoteAddr)
		}

		//old gw
		remoteTun.dnsServers = strings.Split(tunDNS, ",")
		remoteTun.tunDev, err = tun.RegTunDev(tunDevice,tunAddr,tunMask,tunGW,tunDNS)
		if err != nil {
			fmt.Println("start tun err:", err)
			return err;
		}

		//windows
		if runtime.GOOS=="windows" {
			oldDns:=comm.GetDnsServer();
			if oldDns!=nil&&len(oldDns)>0 {
				remoteTun.dnsServers = append(remoteTun.dnsServers, oldDns...)
			}
			routeEdit(tunGW,remoteTun.remoteAddr,remoteTun.dnsServers, remoteTun.oldGw);
		}
		go tunRecv(remoteTun.tunDev, param.Args.Mtu)
		return nil;
	}
	return nil;
}

/**/
func (remoteTun *RemoteTun) Shutdown(){
	if remoteTun.tunDev!=nil {
		remoteTun.tunDev.Close();
	}
	unRegRoute(remoteTun.tunGW ,remoteTun.remoteAddr ,remoteTun.dnsServers ,remoteTun.oldGw)
}

func tunRecv(dev io.ReadWriteCloser ,mtu int) error{
	if param.Args.TunSmartProxy {
		_,channelLinkID, err := comm.NewDefaultStack(mtu, tcpForward, udpForward);
		if err != nil {
			return err;
		}
		// write tun
		go func() {
			var buffer = new(bytes.Buffer)
			for {
				pkt,res:=channelLinkID.Read()
				if !res {
					continue;
				}
				buffer.Reset()
				//buffer.Write(pkt.Pkt.LinkHeader().View())
				buffer.Write(pkt.Pkt.NetworkHeader().View())
				buffer.Write(pkt.Pkt.TransportHeader().View())
				buffer.Write(pkt.Pkt.Data.ToView())
				//tmpBuf:=append(pkt.Pkt.Header.View(),pkt.Pkt.Data.ToView()...)
				if buffer.Len() > 0 {
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
			if true {
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
		if strings.HasPrefix(param.Args.ServerAddr,"sudp") {
			packetSwapTun(dev, mtu);
		}else {
			StreamSwapTun(dev, mtu)
		}
	}
	return nil
}



type TunConn struct {
	Tunnel comm.CommConn
	UdpConn *udpHeader.UdpConn
	UdpAddr *net.UDPAddr
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


func (rd *TunConn) GetPacket()(*udpHeader.UdpConn){
	rd.Lock();
	defer rd.Unlock();
	return rd.UdpConn;
}
func (rd *TunConn) PutPacket(tunnel *udpHeader.UdpConn){
	rd.Lock();
	defer rd.Unlock();
	rd.UdpConn=tunnel;
}


/*send cmd  and UniqueId  and mtu*/
func  connectTun(uniqueId string,mtu int)(comm.CommConn,error){
	var err error;
	tunnelcomm.ResetTunnel();
	tunnel,err:= tunnelcomm.NewTunnel();
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

func connectUdp()(*udpHeader.UdpConn,error){
	udpAddr, err := net.ResolveUDPAddr("udp4", 	param.Args.ServerAddr[7:])
	if err!=nil {
		return nil,err;
	}
	_conn,err:=net.DialUDP("udp4", nil, udpAddr)
	if err!=nil {
		return nil,err;
	}
	return udpHeader.NewUdpConn(_conn),nil;
}


/*udp packet*/
func  packetSwapTun(dev  io.ReadWriteCloser,mtu int){
	tunPacket:=&TunConn{}
	var aesGcm=comm.NewAesGcm();
	if aesGcm==nil {
		fmt.Println("aesGcm init error")
	}
	go func(_tunPacket *TunConn) {
		var bufByte []byte = make([]byte,mtu+80)
		var buffer2 bytes.Buffer
		for {
			n, err := dev.Read(bufByte[:])
			if err != nil {
				log.Printf("dev err%v\r\n", err)
				break;
			}


			buffer2.Reset();
			//MTU
			var mtuByte []byte = make([]byte, 2)
			binary.LittleEndian.PutUint16(mtuByte,uint16(mtu))
			buffer2.Write(mtuByte)
			//packet
			buffer2.Write(bufByte[:n]);
			ciphertext,err:=aesGcm.AesGcm(buffer2.Bytes(),true);
			udpConn:=_tunPacket.GetPacket();
			if udpConn!=nil&&err==nil {
				_,err=udpConn.Write(ciphertext)
				if err!=nil {
					log.Printf("err:%v\r\n",err)
					udpConn.Close();
					_tunPacket.PutPacket(nil)
				}
			}else{
				log.Printf("err:%v\r\n",err)
			}
		}
	}(tunPacket);

	var buffer []byte = make([]byte,65535)
	for {
		tunnel:=tunPacket.GetPacket();
		if tunnel==nil {
			_tunnel,err:=connectUdp();
			if err==nil {
				tunPacket.PutPacket(_tunnel)
			}else {
				time.Sleep(10 * time.Second);
				fmt.Printf("re tunPacket 3 e:%v\r\n", err)
			}
			continue;
		}


		n, _, err := tunnel.ReadFrom(buffer)
		if err != nil {
			tunPacket.PutPacket(nil)
			continue;
		}
		ciphertext,err:=aesGcm.AesGcm(buffer[:n],false);
		if err==nil {
			_, err = dev.Write(ciphertext)
			if err != nil {
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
			if tunnel!=nil {
				_,err=tunnel.Write(buffer.Bytes())
				if err != nil {
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
		if tunnel==nil {
			_tunnel,err:=connectTun(tunStream.UniqueId,tunStream.Mtu);
			if err==nil {
				tunStream.PutTunnel(_tunnel)
			}else {
				time.Sleep(10 * time.Second);
				fmt.Printf("re TunStream 3 e:%v\r\n", err)
			}
			continue;
		}
		_, err := io.ReadFull(tunnel, packLenByte)
		packLen := binary.LittleEndian.Uint16(packLenByte)
		if err != nil||int(packLen)>len(bufByte) {
			tunStream.PutTunnel(nil)
			continue;
		}
		_, err = io.ReadFull(tunnel, bufByte[:int(packLen)])
		if err != nil {
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
	conn2, err := net.DialTimeout("udp",conn.LocalAddr().String(),param.Args.ConnectTime);
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	defer conn2.Close();
	comm.UdpPipe(conn,conn2,time.Minute*5);
	return nil;
}

/*udp 转发*/
func tcpForward(conn *gonet.TCPConn) error{
	conn2, err := net.DialTimeout("tcp", conn.LocalAddr().String(),param.Args.ConnectTime);
	if err != nil {
		fmt.Println(err.Error())
		return err;
	}
	comm.TcpPipe(conn,conn2,time.Minute*5)
	return nil;
}