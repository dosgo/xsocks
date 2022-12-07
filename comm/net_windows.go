//go:build windows
// +build windows

package comm

import (
	"log"
	"net"
	"os/exec"
	"strings"
	"syscall"

	routetable "github.com/yijunjun/route-table"
	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var oldDns = "114.114.114.114"
var defaultDns = "114.114.114.114"

/*set system proxy*/
func SetSystenProxy(proxyServer string, whiteList string, open bool) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, "Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings", registry.ALL_ACCESS)
	if err != nil {
		log.Printf("err:%s", err.Error())
		return false
	}
	defer key.Close()
	if open {
		err = key.SetDWordValue("ProxyEnable", 0x01)
		if err != nil {
			log.Printf("err:%s", err.Error())
			return false
		}
	} else {
		err = key.SetDWordValue("ProxyEnable", 0x00)
		if err != nil {
			log.Printf("err:%s", err.Error())
			return false
		}
	}

	err = key.SetStringValue("ProxyServer", proxyServer)
	if err != nil {
		log.Printf("err:%s", err.Error())
		return false
	}
	if len(whiteList) > 0 {
		err = key.SetStringValue("ProxyOverride", whiteList)
		if err != nil {
			log.Printf("err:%s", err.Error())
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
	item := getGatewayInfo()
	return item.GwIp
}

type GatewayInfo struct {
	GwIp    string
	IfIndex uint32
}

func getGatewayInfo() GatewayInfo {
	var gatewayInfo = GatewayInfo{IfIndex: 0}
	table, err := routetable.NewRouteTable()
	if err != nil {
		return gatewayInfo
	}
	defer table.Close()
	rows, err := table.Routes()
	if err != nil {
		return gatewayInfo
	}
	var minMetric uint32 = 0
	var forwardMask uint32 = 0
	for _, row := range rows {
		if routetable.Inet_ntoa(row.ForwardDest, false) == "0.0.0.0" {
			if minMetric == 0 {
				minMetric = row.ForwardMetric1
				gatewayInfo.IfIndex = row.ForwardIfIndex
				gatewayInfo.GwIp = routetable.Inet_ntoa(row.ForwardNextHop, false)
			} else {
				if row.ForwardMetric1 < minMetric || row.ForwardMask > forwardMask {
					minMetric = row.ForwardMetric1
					gatewayInfo.IfIndex = row.ForwardIfIndex
					gatewayInfo.GwIp = routetable.Inet_ntoa(row.ForwardNextHop, false)
				}
			}
		}
	}
	return gatewayInfo
}

func GetGatewayIndex() uint32 {
	item := getGatewayInfo()
	return item.IfIndex
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
	var adapters = []NetworkAdapter{}
	//DNSServerSearchOrder
	err := GetNetworkAdapter(&adapters)
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

func GetNetworkAdapter(s *[]NetworkAdapter) error {
	//var s = []NetworkAdapter{}
	err := wmi.Query("SELECT Caption,SettingID,InterfaceIndex,DNSServerSearchOrder,DefaultIPGateway,ServiceName,IPAddress,IPSubnet,DHCPEnabled       FROM Win32_NetworkAdapterConfiguration WHERE IPEnabled=True", s) // WHERE (BIOSVersion IS NOT NULL)
	if err != nil {
		log.Printf("err:%v\r\n", err)
		return err
	}
	return nil
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
