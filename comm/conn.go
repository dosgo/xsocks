package comm

import (
	"sync"

	"golang.org/x/time/rate"

	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type UdpLimit struct {
	Limit   *rate.Limiter
	Expired int64
}

type CommConn interface {
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	io.ReadWriteCloser
}

type TimeoutConn struct {
	Conn    CommConn
	TimeOut time.Duration
}

func (conn TimeoutConn) Read(buf []byte) (int, error) {
	conn.Conn.SetReadDeadline(time.Now().Add(conn.TimeOut))
	return conn.Conn.Read(buf)
}

func (conn TimeoutConn) Write(buf []byte) (int, error) {
	conn.Conn.SetWriteDeadline(time.Now().Add(conn.TimeOut))
	return conn.Conn.Write(buf)
}

/*tcp swap*/
func ConnPipe(src CommConn, dst CommConn, duration time.Duration) {
	defer src.Close()
	defer dst.Close()
	srcT := TimeoutConn{src, duration}
	dstT := TimeoutConn{dst, duration}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := io.Copy(srcT, dstT)
		if err != nil {
			return
		}
	}()

	go func() {
		defer wg.Done()
		_, err := io.Copy(dstT, srcT)
		if err != nil {
			return
		}
	}()

	// 等待所有 goroutines 完成
	wg.Wait()
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

func IsPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return false
	}
	if !ip.IsPrivate() {
		return true
	}
	return false
}
