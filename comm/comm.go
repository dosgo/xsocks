package comm

import (
	"bytes"
	"crypto/md5"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dosgo/go-tun2socks/core"
	"golang.org/x/time/rate"
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

type CommConnTimeout struct {
	Conn    CommConn
	TimeOut time.Duration
}

func (conn CommConnTimeout) Read(buf []byte) (int, error) {
	conn.Conn.SetDeadline(time.Now().Add(conn.TimeOut))
	return conn.Conn.Read(buf)
}

func (conn CommConnTimeout) Write(buf []byte) (int, error) {
	conn.Conn.SetDeadline(time.Now().Add(conn.TimeOut))
	return conn.Conn.Write(buf)
}

func GenPasswordHead(password string) string {
	h := md5.New()
	h.Write([]byte(password))
	md5Str := hex.EncodeToString(h.Sum(nil))
	return md5Str[:16]
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
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return "0", err
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.LocalAddr().(*net.UDPAddr).Port), nil
}

func IsPublicIP(ip net.IP) bool {
	ip.IsPrivate()
	if ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return false
	}
	if !ip.IsPrivate() {
		return true
	}
	return false
}

// 生成Guid字串
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
func TunNatSawp(_udpNat *sync.Map, conn core.CommUDPConn, ep core.CommEndpoint, dstAddr string, duration time.Duration) {
	natKey := conn.RemoteAddr().String() + "_" + dstAddr
	var remoteConn net.Conn
	var err error
	_remoteConn, ok := _udpNat.Load(natKey)
	if !ok {
		remoteConn, err = net.DialTimeout("udp", dstAddr, time.Second*15)
		if err != nil {
			return
		}
		var buffer bytes.Buffer
		_udpNat.Store(natKey, remoteConn)
		go func(_remoteConn net.Conn, _conn core.CommUDPConn) {
			defer ep.Close()
			defer _udpNat.Delete(natKey)
			defer _remoteConn.Close()
			defer _conn.Close()
			//buf:= make([]byte, 1024*5);
			for {
				_remoteConn.SetReadDeadline(time.Now().Add(duration))
				buf := poolNatBuf.Get().([]byte)
				n, err := _remoteConn.Read(buf)
				if err != nil {
					log.Printf("err:%v\r\n", err)
					return
				}
				buffer.Reset()
				buffer.Write(buf[:n])
				_, err = _conn.Write(buffer.Bytes())
				if err != nil {
					log.Printf("err:%v\r\n", err)
				}
				poolNatBuf.Put(buf)
			}
		}(remoteConn, conn)
	} else {
		remoteConn = _remoteConn.(net.Conn)
	}
	buf := poolNatBuf.Get().([]byte)
	udpSize, err := conn.Read(buf)
	if err == nil {
		_, err = remoteConn.Write(buf[:udpSize])
		if err != nil {
			log.Printf("err:%v\r\n", err)
		}
	}
	poolNatBuf.Put(buf)
}

/*stream swap*/
func TcpPipe(src CommConn, dst CommConn, duration time.Duration) {
	defer src.Close()
	defer dst.Close()
	srcT := CommConnTimeout{src, duration}
	dstT := CommConnTimeout{dst, duration}
	go io.Copy(srcT, dstT)
	io.Copy(dstT, srcT)
}

type lAddr struct {
	Name       string
	IpAddress  string
	IpMask     string
	GateWay    string
	MACAddress string
}

func GetNetworkInfo() ([]lAddr, error) {
	intf, err := net.Interfaces()
	lAddrs := []lAddr{}
	if err != nil {
		log.Printf("get network info failed: %v", err)
		return nil, err
	}
	for _, v := range intf {
		ips, err := v.Addrs()
		if err != nil {
			log.Printf("get network addr failed: %v", err)
			return nil, err
		}
		//此处过滤loopback（本地回环）和isatap（isatap隧道）
		if !strings.Contains(v.Name, "Loopback") && !strings.Contains(v.Name, "isatap") {
			itemAddr := lAddr{}
			itemAddr.Name = v.Name
			itemAddr.MACAddress = v.HardwareAddr.String()
			for _, ip := range ips {
				if strings.Contains(ip.String(), ".") {
					_, ipNet, err1 := net.ParseCIDR(ip.String())
					if err1 == nil {
						itemAddr.IpAddress = ipNet.IP.String()
						itemAddr.IpMask = net.IP(ipNet.Mask).String()
					}
				}
			}
			lAddrs = append(lAddrs, itemAddr)
		}
	}
	return lAddrs, nil
}

/*
get Unused B
return tunaddr tungw
*/
func GetUnusedTunAddr() (string, string) {
	laddrs, err := GetNetworkInfo()
	if err != nil {
		return "", ""
	}
	var laddrInfo = ""
	for _, _laddr := range laddrs {
		laddrInfo = laddrInfo + "net:" + _laddr.IpAddress
	}
	//tunAddr string,tunMask string,tunGW
	for i := 19; i < 254; i++ {
		if strings.Index(laddrInfo, "net:172."+strconv.Itoa(i)) == -1 {
			return "172." + strconv.Itoa(i) + ".0.2", "172." + strconv.Itoa(i) + ".0.1"
		}
	}
	return "", ""
}

type UdpLimit struct {
	Limit   *rate.Limiter
	Expired int64
}

func InitLog(_logfile string, flag int) {
	logfile := _logfile
	if _logfile == "" {
		logfile = "out.log"
	}
	logFile, err := os.OpenFile(logfile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err == nil {
		mw := io.MultiWriter(logFile)
		if ExistStdOutput() {
			mw = io.MultiWriter(logFile, os.Stdout)
		}
		log.SetOutput(mw)
	}
	log.SetFlags(flag)
}
