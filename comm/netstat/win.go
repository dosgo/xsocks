// +build windows

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
	tbl, err := netstat.GetTCPTable2(true)
	if err != nil {
		return 0, err
	}
	snp, err := netstat.CreateToolhelp32Snapshot(netstat.Th32csSnapProcess, 0)
	if err != nil {
		return 0, err
	}
	s := tbl.Rows()
	for i := range s {
		ent:= netstat.SockTabEntry{
			LocalAddr:  s[i].LocalSock(),
			RemoteAddr: s[i].RemoteSock(),
			State:      s[i].SockState(),
		}
		if ent.State == netstat.Listen && ent.LocalAddr.Port==uint16(lPort) {
			process:=s[i].Process(snp);
			if process!=nil {
				return process.Pid, nil;
			}
		}
	}
	snp.Close()
	return 0, nil;
}


func IsSocksServerAddr(pid int,addr string)bool{
	tbl, err := netstat.GetTCPTable2(true)
	if err != nil {
		return  false;
	}
	snp, err := netstat.CreateToolhelp32Snapshot(netstat.Th32csSnapProcess, 0)
	defer 	snp.Close();
	if err != nil {
		return  false;
	}
	s := tbl.Rows()
	for i := range s {
		ent:= netstat.SockTabEntry{
			LocalAddr:  s[i].LocalSock(),
			RemoteAddr: s[i].RemoteSock(),
			State:      s[i].SockState(),
		}
		if ent.State == netstat.Established  || ent.State == netstat.FinWait1 ||ent.State == netstat.FinWait2 || ent.State == netstat.SynSent {
			if strings.Index(ent.RemoteAddr.String(),addr)!=-1 {
				process:=s[i].Process(snp);
				if process!=nil  && process.Pid==pid  {
					return true;
				}
			}
		}
	}
	return false;
}