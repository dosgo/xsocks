//go:build windows
// +build windows

package comm

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"github.com/StackExchange/wmi"
	routetable "github.com/yijunjun/route-table"
	"golang.org/x/sys/windows/registry"
)

var oldDns = "114.114.114.114"
var defaultDns = "114.114.114.114"

/*set system proxy*/
func SetSystenProxy(proxyServer string, whiteList string, open bool) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, "Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings", registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("err:%s", err.Error())
		return false
	}
	defer key.Close()
	if open {
		err = key.SetDWordValue("ProxyEnable", 0x01)
		if err != nil {
			fmt.Printf("err:%s", err.Error())
			return false
		}
	} else {
		err = key.SetDWordValue("ProxyEnable", 0x00)
		if err != nil {
			fmt.Printf("err:%s", err.Error())
			return false
		}
	}

	err = key.SetStringValue("ProxyServer", proxyServer)
	if err != nil {
		fmt.Printf("err:%s", err.Error())
		return false
	}
	if len(whiteList) > 0 {
		err = key.SetStringValue("ProxyOverride", whiteList)
		if err != nil {
			fmt.Printf("err:%s", err.Error())
			return false
		}
	}
	return true
}

func CloseSystenProxy() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, "Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings", registry.ALL_ACCESS)
	if err != nil {
		return false
	}
	defer key.Close()
	key.SetDWordValue("ProxyEnable", 0x00)
	return true
}

func GetGateway() string {
	table, err := routetable.NewRouteTable()
	if err != nil {
		return ""
	}
	defer table.Close()
	rows, err := table.Routes()
	if err != nil {
		return ""
	}
	var minMetric uint32 = 0
	var gwIp = ""
	for _, row := range rows {
		if routetable.Inet_ntoa(row.ForwardDest, false) == "0.0.0.0" {

			if minMetric == 0 {
				minMetric = row.ForwardMetric1
				gwIp = routetable.Inet_ntoa(row.ForwardNextHop, false)
			} else {
				if row.ForwardMetric1 < minMetric {
					minMetric = row.ForwardMetric1
					gwIp = routetable.Inet_ntoa(row.ForwardNextHop, false)
				}
			}
		}
	}
	return gwIp
}

func GetGatewayIndex() uint32 {
	table, err := routetable.NewRouteTable()
	if err != nil {
		return 0
	}
	defer table.Close()
	rows, err := table.Routes()
	if err != nil {
		return 0
	}
	var minMetric uint32 = 0
	var ifIndex uint32 = 0
	for _, row := range rows {
		if routetable.Inet_ntoa(row.ForwardDest, false) == "0.0.0.0" {
			if minMetric == 0 {
				minMetric = row.ForwardMetric1
				ifIndex = row.ForwardIfIndex
			} else {
				if row.ForwardMetric1 < minMetric {
					minMetric = row.ForwardMetric1
					ifIndex = row.ForwardIfIndex
				}
			}
		}
	}
	return ifIndex
}

func getAdapterList() (*syscall.IpAdapterInfo, error) {
	b := make([]byte, 1000)
	l := uint32(len(b))
	a := (*syscall.IpAdapterInfo)(unsafe.Pointer(&b[0]))
	err := syscall.GetAdaptersInfo(a, &l)
	if err == syscall.ERROR_BUFFER_OVERFLOW {
		b = make([]byte, l)
		a = (*syscall.IpAdapterInfo)(unsafe.Pointer(&b[0]))
		err = syscall.GetAdaptersInfo(a, &l)
	}
	if err != nil {
		return nil, os.NewSyscallError("GetAdaptersInfo", err)
	}
	return a, nil
}

func GetLocalAddresses() ([]lAddr, error) {
	lAddrs := []lAddr{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	aList, err := getAdapterList()
	if err != nil {
		return nil, err
	}

	for _, ifi := range ifaces {
		for ai := aList; ai != nil; ai = ai.Next {
			index := ai.Index
			if ifi.Index == int(index) {
				ipl := &ai.IpAddressList
				gwl := &ai.GatewayList
				for ; ipl != nil; ipl = ipl.Next {
					itemAddr := lAddr{}
					itemAddr.Name = ifi.Name
					itemAddr.IpAddress = fmt.Sprintf("%s", ipl.IpAddress.String)
					itemAddr.IpMask = fmt.Sprintf("%s", ipl.IpMask.String)
					itemAddr.GateWay = fmt.Sprintf("%s", gwl.IpAddress.String)
					lAddrs = append(lAddrs, itemAddr)
				}
			}
		}
	}
	return lAddrs, err
}

/*获取旧的dns,内网解析用*/
func GetOldDns(dnsAddr string, tunGW string, _tunGW string) string {
	ifIndex := GetGatewayIndex()
	dnsServers, _, _ := GetDnsServerByIfIndex(ifIndex)
	for _, v := range dnsServers {
		if v != dnsAddr && v != tunGW && v != _tunGW {
			oldDns = v
			break
		}
	}
	return oldDns
}

func GetDnsServerByIfIndex(ifIndex uint32) ([]string, bool, bool) {
	//DNSServerSearchOrder
	adapters, err := GetNetworkAdapter()
	var isIpv6 = false
	if err != nil {
		return nil, false, isIpv6
	}
	for _, v := range adapters {
		if v.InterfaceIndex == ifIndex {
			for _, v2 := range v.IPAddress {
				if len(v2) > 16 {
					isIpv6 = true
					break
				}
			}
			return v.DNSServerSearchOrder, v.DHCPEnabled, isIpv6
		}
	}
	return nil, false, isIpv6
}

type NetworkAdapter struct {
	DNSServerSearchOrder []string
	DefaultIPGateway     []string
	IPAddress            []string
	Caption              string
	DHCPEnabled          bool
	ServiceName          string
	IPSubnet             []string
	InterfaceIndex       uint32
	SettingID            string
}

func GetNetworkAdapter() ([]NetworkAdapter, error) {
	var s = []NetworkAdapter{}
	err := wmi.Query("SELECT Caption,SettingID,InterfaceIndex,DNSServerSearchOrder,DefaultIPGateway,ServiceName,IPAddress,IPSubnet,DHCPEnabled       FROM Win32_NetworkAdapterConfiguration WHERE IPEnabled=True", &s) // WHERE (BIOSVersion IS NOT NULL)
	if err != nil {
		log.Printf("err:%v\r\n", err)
		return nil, err
	}
	return s, nil
}
func SetNetConf(dnsIpv4 string, dnsIpv6 string) {

}

func ResetNetConf(ip string) {

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
	lAdds, err := GetLocalAddresses()
	var iName = ""
	if err == nil {
		for _, v := range lAdds {
			if strings.Index(v.IpAddress, tunAddr) != -1 {
				iName = v.Name
				break
			}
		}
	}

	//clear old
	CmdHide("route", "delete", strings.Join(netNat, ".")).Output()
	cmd := CmdHide("netsh", "interface", "ipv4", "add", "route", strings.Join(netNat, ".")+"/"+maskAddrs[1], iName, tunGw, "metric=6", "store=active")
	fmt.Printf("cmd:%s\r\n", cmd.Args)
	cmd.Run()

	fmt.Printf("cmd:%s\r\n", strings.Join(cmd.Args, " "))
	CmdHide("ipconfig", "/flushdns").Run()
	return nil
}

func CmdHide(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}
