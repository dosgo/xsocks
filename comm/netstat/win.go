//go:build windows
// +build windows

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
	tbl, err := netstat.GetTCPTable2(true)
	if err != nil {
		return 0, err
	}
	s := tbl.Rows()
	for i := range s {
		ent := netstat.SockTabEntry{
			LocalAddr:  s[i].LocalSock(),
			RemoteAddr: s[i].RemoteSock(),
			State:      s[i].SockState(),
		}
		if ent.State == netstat.Listen && ent.LocalAddr.Port == uint16(lPort) {
			return int(s[i].WinPid), nil
		}
	}
	return 0, nil
}

func IsUdpSocksServerAddr(pid int, addr string) bool {
	tbl, err := netstat.GetUDPTableOwnerPID(true)
	if err != nil {
		return false
	}
	s := tbl.Rows()
	for i := range s {
		ent := netstat.SockTabEntry{
			LocalAddr:  s[i].LocalSock(),
			RemoteAddr: s[i].RemoteSock(),
			State:      s[i].SockState(),
		}
		if strings.Index(ent.RemoteAddr.String(), addr) != -1 {
			if int(s[i].WinPid) == pid {
				return true
			}
		}
	}
	return false
}
