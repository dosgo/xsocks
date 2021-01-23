package comm

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	crand "crypto/rand"
	"net"
	"time"
)

type CommConn interface {
	 io.Reader
	 io.Writer
	 io.Closer
}


type TimeoutConn struct {
	Conn net.Conn
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

/*udp swap*/
func UdpPipe(src net.Conn, dst net.Conn) {
	defer src.Close()
	defer dst.Close()
	chan1 := chanFromConn(src)
	chan2 := chanFromConn(dst)
	for {
		select {
		case b1 := <-chan1:
			if b1 == nil {
				return
			}
			_, _ = dst.Write(b1)
		case b2 := <-chan2:
			if b2 == nil {
				return
			}
			_, _ = src.Write(b2)
		}
	}
}

/*tcp swap*/
func TcpPipe(src net.Conn, dst net.Conn,duration time.Duration) {
	defer src.Close()
	defer dst.Close()
	srcT:=TimeoutConn{src,duration}
	dstT:=TimeoutConn{dst,duration}
	go io.Copy(srcT, dstT)
	io.Copy(dstT, srcT)
}

func chanFromConn(conn net.Conn) chan []byte {
	c := make(chan []byte)
	go func() {
		b := make([]byte, 65535)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(time.Minute))
			n, err := conn.Read(b)
			if n > 0 {
				res := make([]byte, n)
				copy(res, b[:n])
				c <- res
			}
			if err != nil {
				c <- nil
				break
			}
		}
	}()
	return c
}