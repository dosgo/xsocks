package comm

import (
	"bytes"
	"crypto/md5"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"golang.org/x/time/rate"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"io"
	"log"
	"sort"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)


var poolNatBuf = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 4096)
	},
}


type CommConn interface {
	SetDeadline(t time.Time) error
	io.ReadWriteCloser
}


type TimeoutConn struct {
	Conn CommConn
	TimeOut time.Duration;
}

func (conn TimeoutConn) Read(buf []byte) (int, error) {
	conn.Conn.SetDeadline(time.Now().Add(conn.TimeOut))
	return conn.Conn.Read(buf)
}

func (conn TimeoutConn) Write(buf []byte) (int, error) {
	conn.Conn.SetDeadline(time.Now().Add(conn.TimeOut))
	return conn.Conn.Write(buf)
}


func GenPasswordHead(password string)string{
	h := md5.New()
	h.Write([]byte(password))
	md5Str:=hex.EncodeToString(h.Sum(nil))
	return md5Str[:16];
}


func GetFreePort() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return "0", err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "0", err
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port), nil
}


func GetFreeUdpPort() (string, error) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return "0", err
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.LocalAddr().(*net.UDPAddr).Port), nil
}



func IsPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return false
	}
	// IPv4私有地址空间
	// A类：10.0.0.0到10.255.255.255
	// B类：172.16.0.0到172.31.255.255
	// C类：192.168.0.0到192.168.255.255
	if ip4 := ip.To4(); ip4 != nil {
		switch true {
		case ip4[0] == 10:
			return false
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return false
		case ip4[0] == 192 && ip4[1] == 168:
			return false
		case ip4[0] == 169 && ip4[1] == 254:
			return false
		default:
			return true
		}
	}
	// IPv6私有地址空间：以前缀FEC0::/10开头
	if ip6 := ip.To16(); ip6 != nil {
		if ip6[0] == 15 && ip6[1] == 14 && ip6[2] <= 12 {
			return false
		}
		return true
	}
	return false
}
func GetRandomString(n int) string {
	str := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	var result []byte
	for i := 0; i < n; i++ {
		result = append(result, bytes[rand.Intn(len(bytes))])
	}
	return string(result)
}


//生成32位md5字串
func GetMd5String(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

//生成Guid字串
func UniqueId(_len int) string {
	b := make([]byte, 48)
	if _, err := io.ReadFull(crand.Reader, b); err != nil {
		return ""
	}
	h := md5.New()
	h.Write([]byte(b))
	return hex.EncodeToString(h.Sum(nil))[:_len]
}



/*udp nat sawp*/
func TunNatSawp(_udpNat *sync.Map,conn *gonet.UDPConn,ep tcpip.Endpoint,dstAddr string,duration time.Duration){
	natKey:=conn.RemoteAddr().String()+"_"+dstAddr;
	var remoteConn net.Conn
	var err error;
	_remoteConn,ok:=_udpNat.Load(natKey)
	if !ok{
		remoteConn, err = net.DialTimeout("udp", dstAddr,time.Second*15);
		if err != nil {
			return
		}
		var buffer bytes.Buffer
		_udpNat.Store(natKey,remoteConn)
		go func(_remoteConn net.Conn,_conn *gonet.UDPConn) {
			defer ep.Close();
			defer _udpNat.Delete(natKey);
			defer _remoteConn.Close()
			defer _conn.Close();
			//buf:= make([]byte, 1024*5);
			for {
				_remoteConn.SetReadDeadline(time.Now().Add(duration))
				buf := poolNatBuf.Get().([]byte)
				n, err:= _remoteConn.Read(buf);
				if err!=nil {
					log.Printf("err:%v\r\n",err);
					return ;
				}
				buffer.Reset();
				buffer.Write(buf[:n])
				_,err=_conn.Write(buffer.Bytes())
				if err!=nil {
					log.Printf("err:%v\r\n",err);
				}
				poolNatBuf.Put(buf)
			}
		}(remoteConn,conn)
	}else{
		remoteConn=_remoteConn.(net.Conn)
	}
	buf := poolNatBuf.Get().([]byte)
	udpSize,err:=conn.Read(buf);
	if err==nil {
		_,err=remoteConn.Write(buf[:udpSize])
		if err!=nil {
			log.Printf("err:%v\r\n",err);
		}
	}
	poolNatBuf.Put(buf)
}





/*tcp swap*/
func TcpPipe(src CommConn, dst CommConn,duration time.Duration) {
	defer src.Close()
	defer dst.Close()
	srcT:=TimeoutConn{src,duration}
	dstT:=TimeoutConn{dst,duration}
	go io.Copy(srcT, dstT)
	io.Copy(dstT, srcT)
}


type lAddr struct {
	Name  string
	IpAddress  string
	IpMask  string
	GateWay  string
	MACAddress string
}




func GetNetworkInfo() ([]lAddr,error) {
	intf, err := net.Interfaces()
	lAddrs := []lAddr{}
	if err != nil {
		log.Fatal("get network info failed: %v", err)
		return nil,err
	}
	for _, v := range intf {
		ips, err := v.Addrs()
		if err != nil {
			log.Fatal("get network addr failed: %v", err)
			return nil,err
		}
		//此处过滤loopback（本地回环）和isatap（isatap隧道）
		if !strings.Contains(v.Name, "Loopback") && !strings.Contains(v.Name, "isatap") {
			itemAddr := lAddr{}
			itemAddr.Name=v.Name;
			itemAddr.MACAddress=v.HardwareAddr.String()
			for _, ip := range ips {
				if strings.Contains(ip.String(), ".") {
					_,ipNet,err1:=net.ParseCIDR(ip.String())
					if err1==nil {
						itemAddr.IpAddress=ipNet.IP.String()
						itemAddr.IpMask=net.IP(ipNet.Mask).String()
					}
				}
			}
			lAddrs=append(lAddrs,itemAddr)
		}
	}
	return lAddrs,nil
}
/*
get Unused B
return tunaddr tungw
*/
func GetUnusedTunAddr()(string,string){
	laddrs,err:=GetNetworkInfo();
	if err!=nil {
		return "","";
	}
	var laddrInfo="";
	for _, _laddr := range laddrs {
		laddrInfo=laddrInfo+"net:"+_laddr.IpAddress;
	}
	//tunAddr string,tunMask string,tunGW
	for i:=19;i<254;i++{
		if strings.Index(laddrInfo,"net:172."+strconv.Itoa(i))==-1 {
			return "172."+strconv.Itoa(i)+".0.2","172."+strconv.Itoa(i)+".0.1"
		}
	}
	return "","";
}

type UdpLimit struct{
	Limit *rate.Limiter
	Expired int64;
}


func ArrMatch(target string, str_array []string) bool {
	sort.Strings(str_array)
	index := sort.SearchStrings(str_array, target)
	if index < len(str_array) && str_array[index] == target {
		return true
	}
	return false
}