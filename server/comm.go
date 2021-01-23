package server

import (
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
		fmt.Printf("err:%v\r\n",err)
		return
	}
	//autherr;
	if(string(authHead)!= comm.GenPasswordHead(param.Password)){
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
			go io.Copy(sConn, conn)
			io.Copy(conn, sConn)
			break;
		//to tun
		case 0x03:
			toTunTcp(conn)
			break;
	}
}


/*to tun 处理*/
func toTunTcp(conn comm.CommConn){
	uniqueIdByte := make([]byte,8)
	_, err := io.ReadFull(conn, uniqueIdByte)
	if(err!=nil){
		log.Printf("err:%v\r\n",param.TunPort)
		return ;
	}
	uniqueId:=string(uniqueIdByte)
	fmt.Printf("uniqueId:%s\r\n",uniqueId)
	var sConn net.Conn;
	//连接tun
	sConn, err = net.DialTimeout("tcp", "127.0.0.1:"+param.TunPort, param.ConnectTime)
	if (err != nil) {
		log.Printf("err:%v\r\n", param.TunPort)
		return;
	}

	switch netConn :=conn.(type) {
		case net.Conn:
			comm.TcpPipe(netConn,sConn,time.Minute*5)
			break;
		default:
			TimeoutSConn:=comm.TimeoutConn{sConn,time.Minute*5}
			go io.Copy(TimeoutSConn, conn)
			io.Copy(conn, TimeoutSConn)
			break;
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



