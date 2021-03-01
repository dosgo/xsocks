// +build windows

package comm

import (
	"github.com/StackExchange/wmi"
	"github.com/yijunjun/route-table"
	"fmt"
	"golang.org/x/sys/windows"
	"net"
	"os"
	"os/exec"
	"strings"
	"log"
	"syscall"
	"unsafe"
)


func GetGateway()string {
	table, err := routetable.NewRouteTable()
	if err != nil {
		panic(err.Error())
	}
	defer table.Close()
	rows, err := table.Routes()
	if err != nil {
		panic(err.Error())
	}
	for _, row := range rows {
		if routetable.Inet_ntoa(row.ForwardDest, false)=="0.0.0.0" {
			return routetable.Inet_ntoa(row.ForwardNextHop, false);
		}
	}
	return "";
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



func GetLocalAddresses() ([]lAddr ,error) {
	lAddrs := []lAddr{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil,err
	}

	aList, err := getAdapterList()
	if err != nil {
		return nil,err
	}


	for _, ifi := range ifaces {
		for ai := aList; ai != nil; ai = ai.Next {
			index := ai.Index
			if ifi.Index == int(index) {
				ipl := &ai.IpAddressList
				gwl := &ai.GatewayList
				for ; ipl != nil; ipl = ipl.Next  {
					itemAddr := lAddr{}
					itemAddr.Name=ifi.Name
					itemAddr.IpAddress=fmt.Sprintf("%s",ipl.IpAddress.String)
					itemAddr.IpMask=fmt.Sprintf("%s",ipl.IpMask.String)
					itemAddr.GateWay=fmt.Sprintf("%s",gwl.IpAddress.String)
					lAddrs=append(lAddrs,itemAddr)
				}
			}
		}
	}
	return lAddrs,err
}




//dns

const (
	DnsConfigDnsServerList int32 = 6
)

type char byte
type IpAddressString struct {
	DNS [4 * 10]char
}

type Ip4Array struct {
	AddrCount  uint32
	Ip4Address [1]IpAddressString
}

func GetDnsServer() []string {
	dns := []string{}
	dnsapi := windows.NewLazyDLL("Dnsapi.dll")
	dnsQuery := dnsapi.NewProc("DnsQueryConfig")
	bufferBytes := make([]byte, 60)
loop:
	buffer := (*Ip4Array)(unsafe.Pointer(&bufferBytes[0]))
	blen := len(bufferBytes)
	r1, _, _ := dnsQuery.Call(uintptr(DnsConfigDnsServerList), uintptr(0), uintptr(0), uintptr(0), uintptr(unsafe.Pointer(&bufferBytes[0])), uintptr(unsafe.Pointer(&blen)))
	if r1 == 234 {
		bufferBytes = make([]byte, blen)
		goto loop
	} else if r1 == 0 {

	} else {
		return dns
	}
	for i := uint32(1); i <= buffer.AddrCount; i++ {
		right := i * 4
		left := right - 4
		tmpChars := buffer.Ip4Address[0].DNS[left:right]
		tmpStr := []string{}
		for j := 0; j < len(tmpChars); j++ {
			tmpStr = append(tmpStr, fmt.Sprint(tmpChars[j]))
		}
		tmpDNS := strings.Join(tmpStr, ".")
		pDns := net.ParseIP(tmpDNS)
		if pDns == nil {
			continue
		}
		if !pDns.IsGlobalUnicast() {
			continue
		}
		dns = append(dns, tmpDNS)
	}
	return dns
}

func SetDNSServer(gwIp string,ip string){
	lAdds,err:=GetLocalAddresses();
	if err==nil {
		for _, v := range lAdds {
			if strings.Index(v.GateWay,gwIp)!=-1 {
				exec.Command("netsh", "interface","ip","set","dnsservers",v.Name,"static",ip).Output()
				break;
			}
		}
	}
}



func getDnsServer(gwIp string)string{
	//DNSServerSearchOrder
	adapters,err:=getNetworkAdapter()
	if(err!=nil){
		return "";
	}
	for _,v:=range adapters{
		if(v.DefaultIPGateway[0]==gwIp){
			return v.DNSServerSearchOrder[0];
		}
	}
	return "";
}

type Network struct {
	Name       string
	IP         string
	MACAddress string
}

type intfInfo struct {
	Name       string
	MacAddress string
	Ipv4       []string
}

func GetNetworkInfo() error {
	intf, err := net.Interfaces()
	if err != nil {
		log.Fatal("get network info failed: %v", err)
		return err
	}
	var is = make([]intfInfo, len(intf))
	for i, v := range intf {
		ips, err := v.Addrs()
		if err != nil {
			log.Fatal("get network addr failed: %v", err)
			return err
		}
		//此处过滤loopback（本地回环）和isatap（isatap隧道）
		if !strings.Contains(v.Name, "Loopback") && !strings.Contains(v.Name, "isatap") {
			var network Network
			is[i].Name = v.Name
			is[i].MacAddress = v.HardwareAddr.String()
			for _, ip := range ips {
				if strings.Contains(ip.String(), ".") {
					is[i].Ipv4 = append(is[i].Ipv4, ip.String())
				}
			}
			network.Name = is[i].Name
			network.MACAddress = is[i].MacAddress
			if len(is[i].Ipv4) > 0 {
				network.IP = is[i].Ipv4[0]
			}

			fmt.Printf("network:=", network)
		}

	}

	return nil
}
//BIOS信息
func GetBiosInfo() string {
	var s = []struct {
		Name string
	}{}
	err := wmi.Query("SELECT Name FROM Win32_BIOS WHERE (Name IS NOT NULL)", &s) // WHERE (BIOSVersion IS NOT NULL)
	if err != nil {
		return ""
	}
	return s[0].Name
}
type NetworkAdapter struct {
	DNSServerSearchOrder   []string
	DefaultIPGateway []string
	IPAddress []string
	Caption    string
	ServiceName  string
	IPSubnet   []string
	SettingID string
}


func getNetworkAdapter() ([]NetworkAdapter,error){
	var s = []NetworkAdapter{}
	err := wmi.Query("SELECT Caption,SettingID,DNSServerSearchOrder,DefaultIPGateway,ServiceName,IPAddress,IPSubnet    FROM Win32_NetworkAdapterConfiguration WHERE IPEnabled=True", &s) // WHERE (BIOSVersion IS NOT NULL)
	if err != nil {
		log.Printf("err:%v\r\n",err)
		return nil,err
	}
	return s,nil;
}

