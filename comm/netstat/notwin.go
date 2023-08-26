//go:build !windows && !wasm
// +build !windows,!wasm

package netstat

import (
	"strconv"
	"strings"

	"github.com/cakturk/go-netstat/netstat"
)

/*为啥要用这方法,因为Process在一些电脑比较耗时间只有匹配的才获取*/
func PortGetPid(lSocks string) (int, error) {
	socksAddrs := strings.Split(lSocks, ":")
	lPort, err := strconv.Atoi(socksAddrs[1])
	// get only listening TCP sockets
	tabs, err := netstat.TCPSocks(func(s *netstat.SockTabEntry) bool {
		return s.State == netstat.Listen && s.LocalAddr.Port == uint16(lPort)
	})
	if err != nil {
		return 0, err
	}
	for _, ent := range tabs {
		if ent.Process != nil {
			return ent.Process.Pid, nil
		}
	}
	return 0, err
}

func IsUdpSocksServerAddr(pid int, addr string) bool {
	tbl, err := netstat.UDPSocks(netstat.NoopFilter)
	if err != nil {
		return false
	}
	for _, ent := range tbl {
		if ent.State == netstat.Established || ent.State == netstat.FinWait1 || ent.State == netstat.FinWait2 || ent.State == netstat.SynSent {
			if strings.Index(ent.RemoteAddr.String(), addr) != -1 {
				if ent.Process != nil && ent.Process.Pid == pid {
					return true
				}
			}
		}
	}
	return false
}
