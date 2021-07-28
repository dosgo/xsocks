// +build !windows


package netstat

import (
	"github.com/cakturk/go-netstat/netstat"
	"strconv"
	"strings"
)

/*为啥要用这方法,因为Process在一些电脑比较耗时间只有匹配的才获取*/
func PortGetPid(lSocks string) (int,error) {
	socksAddrs:=strings.Split(lSocks,":")
	lPort, err := strconv.Atoi(socksAddrs[1])
	tbl, err := netstat.TCPSocks(netstat.NoopFilter)
	if err != nil {
		return 0, err
	}
	for _,ent := range tbl {
		if ent.State == netstat.Listen && ent.LocalAddr.Port==uint16(lPort) {
			if ent.Process!=nil {
				return ent.Process.Pid, nil;
			}
		}
	}
	return 0, nil;
}

func IsSocksServerAddr(pid int,addr string)bool{
	tbl, err := netstat.TCPSocks(netstat.NoopFilter)
	if err != nil {
		return false;
	}
	for _,ent := range tbl {
		if ent.State == netstat.Established  || ent.State == netstat.FinWait1 ||ent.State == netstat.FinWait2 || ent.State == netstat.SynSent  {
			if strings.Index(ent.RemoteAddr.String(),addr)!=-1 {
				if ent.Process!=nil  && ent.Process.Pid==pid  {
					return true;
				}
			}
		}
	}
	return false;
}