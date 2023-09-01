//go:build windows
// +build windows

package comm

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var oldDns = "114.114.114.114"
var defaultDns = "114.114.114.114"

func GetGateway() string {
	item := getGatewayInfo()
	return item.GwIp
}

type GatewayInfo struct {
	GwIp    string
	IfIndex uint32
}

func getGatewayInfo() GatewayInfo {
	var gatewayInfo = GatewayInfo{IfIndex: 0}
	var rows, err = getRoutes()
	if err != nil {
		return gatewayInfo
	}
	var minMetric uint32 = 0
	var forwardMask uint32 = 0
	for _, row := range rows {
		if inet_ntoa(row.ForwardDest, false) == "0.0.0.0" {
			if minMetric == 0 {
				minMetric = row.ForwardMetric1
				gatewayInfo.IfIndex = row.ForwardIfIndex
				gatewayInfo.GwIp = inet_ntoa(row.ForwardNextHop, false)
			} else {
				if row.ForwardMetric1 < minMetric || row.ForwardMask > forwardMask {
					minMetric = row.ForwardMetric1
					gatewayInfo.IfIndex = row.ForwardIfIndex
					gatewayInfo.GwIp = inet_ntoa(row.ForwardNextHop, false)
				}
			}
		}
	}
	return gatewayInfo
}

func inet_ntoa(ipnr uint32, isBig bool) string {
	ip := net.IPv4(0, 0, 0, 0)
	var bo binary.ByteOrder
	if isBig {
		bo = binary.BigEndian
	} else {
		bo = binary.LittleEndian
	}
	bo.PutUint32([]byte(ip.To4()), ipnr)
	return ip.String()
}

func AddRoute(tunAddr string, tunGw string, tunMask string) error {
	var netNat = make([]string, 4)
	//masks:=strings.Split(tunMask,".")
	masks := net.ParseIP(tunMask).To4()
	Addrs := strings.Split(tunAddr, ".")
	for i := 0; i <= 3; i++ {
		if masks[i] == 255 {
			netNat[i] = Addrs[i]
		} else {
			netNat[i] = "0"
		}
	}
	maskAddr := net.IPNet{IP: net.ParseIP(tunAddr), Mask: net.IPv4Mask(masks[0], masks[1], masks[2], masks[3])}
	maskAddrs := strings.Split(maskAddr.String(), "/")
	var iName = ""
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, ifi := range ifaces {
			lAdds, _ := ifi.Addrs()
			for _, v := range lAdds {
				if strings.Index(v.String(), tunAddr) != -1 {
					iName = ifi.Name
					break
				}
			}
		}
	}

	//clear old
	CmdHide("route", "delete", strings.Join(netNat, ".")).Output()
	cmd := CmdHide("netsh", "interface", "ipv4", "add", "route", strings.Join(netNat, ".")+"/"+maskAddrs[1], iName, tunGw, "metric=6", "store=active")
	log.Printf("cmd:%s\r\n", cmd.Args)
	cmd.Run()

	log.Printf("cmd:%s\r\n", strings.Join(cmd.Args, " "))
	CmdHide("ipconfig", "/flushdns").Run()
	return nil
}

func CmdHide(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func ExistStdOutput() bool {
	handle, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	return err == nil && handle > 0
}

type TableRow struct {
	ForwardDest      uint32
	ForwardMask      uint32
	ForwardPolicy    uint32
	ForwardNextHop   uint32
	ForwardIfIndex   uint32
	ForwardType      uint32
	ForwardProto     uint32
	ForwardAge       uint32
	ForwardNextHopAS uint32
	ForwardMetric1   uint32
	ForwardMetric2   uint32
	ForwardMetric3   uint32
	ForwardMetric4   uint32
	ForwardMetric5   uint32
}

type SliceHeader struct {
	Addr uintptr
	Len  int
	Cap  int
}

func getRoutes() ([]TableRow, error) {
	var bufLen uint32
	var getIpForwardTable = syscall.NewLazyDLL("iphlpapi.dll").NewProc("GetIpForwardTable")
	getIpForwardTable.Call(uintptr(0), uintptr(unsafe.Pointer(&bufLen)), 0)

	var r1 uintptr
	var buf = make([]byte, bufLen)
	r1, _, _ = getIpForwardTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufLen)), 0)

	if r1 != 0 {
		return nil, fmt.Errorf("call to GetIpForwardTable failed with result value：%v", r1)
	}

	var (
		num     = *(*uint32)(unsafe.Pointer(&buf[0]))
		routes  = make([]TableRow, num)
		sr      = uintptr(unsafe.Pointer(&buf[0])) + unsafe.Sizeof(num)
		rowSize = unsafe.Sizeof(TableRow{})

		expectedBufferSize = int(bufLen)
		actualBufferSize   = int(unsafe.Sizeof(num) + rowSize*uintptr(num))
	)

	if expectedBufferSize < actualBufferSize {
		return nil, fmt.Errorf("buffer exceeded the expected size of %v while having a size of: %v", expectedBufferSize, actualBufferSize)
	}
	/*
		for i := 0; i < int(num); i++ {
			routes[i] = *((*TableRow)(unsafe.Pointer(uintptr(sr) + (rowSize * uintptr(i)))))
		}
	*/

	sh_rows := (*SliceHeader)(unsafe.Pointer(&routes))
	sh_rows.Addr = sr
	sh_rows.Len = int(num)
	sh_rows.Cap = int(num)
	return routes, nil
}
