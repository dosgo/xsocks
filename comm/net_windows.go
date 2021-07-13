// +build windows

package comm

import (
	"errors"
	"fmt"
	"github.com/StackExchange/wmi"
	"github.com/songgao/water"
	routetable "github.com/yijunjun/route-table"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"
)


var oldDns="114.114.114.114";
var defaultDns="114.114.114.114";

/*set system proxy*/
/*set system proxy*/
func SetSystenProxy(proxyServer string,whiteList string,open bool) bool{
	key,  err := registry.OpenKey(registry.CURRENT_USER, "Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings", registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("err:%s",err.Error());
		return false;
	}
	defer key.Close()
	if open {
		err=key.SetDWordValue("ProxyEnable", 0x01)
		if err != nil {
			fmt.Printf("err:%s",err.Error());
			return false;
		}
	}else{
		err=key.SetDWordValue("ProxyEnable", 0x00);
		if err != nil {
			fmt.Printf("err:%s",err.Error());
			return false;
		}
	}

	err=key.SetStringValue("ProxyServer",proxyServer);
	if err != nil {
		fmt.Printf("err:%s",err.Error());
		return false;
	}
	if len(whiteList)>0{
		err=key.SetStringValue("ProxyOverride",whiteList)
		if err != nil {
			fmt.Printf("err:%s",err.Error());
			return false;
		}
	}
	return true;
}

func CloseSystenProxy() bool{
	key,  err := registry.OpenKey(registry.CURRENT_USER, "Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings", registry.ALL_ACCESS)
	if err != nil {
		return false;
	}
	defer key.Close()
	key.SetDWordValue("ProxyEnable", 0x00);
	return true;
}



func GetGateway()string {
	table, err := routetable.NewRouteTable()
	if err != nil {
		return "";
	}
	defer table.Close()
	rows, err := table.Routes()
	if err != nil {
		return "";
	}
	var minMetric uint32=0;
	var gwIp="";
	for _, row := range rows {
		if routetable.Inet_ntoa(row.ForwardDest, false)=="0.0.0.0" {
			if minMetric==0 {
				minMetric=row.ForwardMetric1;
				gwIp=routetable.Inet_ntoa(row.ForwardNextHop, false)
			}else{
				if row.ForwardMetric1<minMetric {
					minMetric=row.ForwardMetric1;
					gwIp=routetable.Inet_ntoa(row.ForwardNextHop, false)
				}
			}
		}
	}
	return gwIp;
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

/*获取旧的dns,内网解析用*/
func GetOldDns(dnsAddr string,tunGW string,_tunGW string) string{
	//非默认值说明设置过
	if oldDns!=defaultDns{
		return oldDns;
	}
	gwIp:=GetGateway()
	dnsServers,_,_:=GetDnsServerByGateWay(gwIp);
	for _,v:=range dnsServers{
		if v!=dnsAddr&&v!=tunGW && v!=_tunGW  {
			oldDns=v;
			break;
		}
	}
	return oldDns;
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

var _watchIpChange *watchIpChange



/*
修改配置
*/
func SetNetConf(dnsIpv4 string,dnsIpv6 string){
	//修改dns配置
	gwIp:=GetGateway()
	oneAdapter=&adapterConf{}
	oneAdapter.gwIp=gwIp;
	oneAdapter.dHCPEnabled,oneAdapter.isIPv6=_setDNSServer(dnsIpv4,dnsIpv6,gwIp);

	//注册网络变动
	//注册变动
	_watchIpChange:=&watchIpChange{Run: true,AdapterConf: make(map[string]*adapterConf,0)}
	go _watchIpChange.Watch(func() {
		time.Sleep(time.Second*2)
		gwIp:=GetGateway()
		fmt.Printf("SetDNSServer gwip:%s\r\n",gwIp)
		_adapterConf:=&adapterConf{}
		_adapterConf.gwIp=gwIp;
		_adapterConf.dHCPEnabled,_adapterConf.isIPv6=_setDNSServer("127.0.0.1","0:0:0:0:0:0:0:1",gwIp);
		_watchIpChange.AdapterConf[gwIp]=_adapterConf;
		oldDns=defaultDns;//设置回默认值
	})
}

func _setDNSServer(dnsIpv4 string,dnsIpv6 string,gwIp string) (bool,bool){
	log.Printf("SetDNSServer-gwIp:%s\r\n",gwIp)
	_,dHCPEnabled,isIPv6:=GetDnsServerByGateWay(gwIp);
	//ipv4
	changeDns("ip",dnsIpv4,gwIp)
	//ipv6
	if isIPv6 {
		changeDns( "ipv6", dnsIpv6,gwIp)
	}
	//ipv4优先
	if isIPv6 {
		Ipv6Switch(false);
	}
	CmdHide("ipconfig", "/flushdns").Run()
	return dHCPEnabled,isIPv6;
}


type adapterConf struct {
	gwIp string;
	dHCPEnabled bool
	isIPv6 bool
}
/*第一个适配器配置*/
var oneAdapter *adapterConf;

type watchIpChange struct {
	Run bool
	AdapterConf map[string]*adapterConf
}

func (wtp *watchIpChange) Watch(callBack func())error{
	var  notifyAddrChange        *syscall.Proc
	if iphlpapi, err := syscall.LoadDLL("Iphlpapi.dll"); err == nil {
		if p, err := iphlpapi.FindProc("NotifyAddrChange"); err == nil {
			notifyAddrChange = p
		}
	}
	if notifyAddrChange==nil {
		return errors.New("NotifyAddrChange\r\n");
	}
	for wtp.Run {
		notifyAddrChange.Call(0, 0)
		callBack();
	}
	return nil;
}

func (wtp *watchIpChange) Shutdown(){
	wtp.Run=false;
	for _,v:=range wtp.AdapterConf{
		_resetDns("ip",v.dHCPEnabled,v.gwIp);
		if v.isIPv6 {
			_resetDns("ipv5",v.dHCPEnabled,v.gwIp);
		}
	}
}


func gwIpToName(gwIp string)string{
	lAdds,err:=GetLocalAddresses();
	var iName="";
	if err==nil {
		for _, v := range lAdds {
			if strings.Index(v.GateWay,gwIp)!=-1 {
				iName=v.Name;
				break;
			}
		}
	}
	return iName;
}


func changeDns(netType string,ip string,gwIp string){
	dnsServers,_,_:=GetDnsServerByGateWay(gwIp);
	var iName=gwIpToName(gwIp);
//	netsh interface ipv6 add dns
	//netsh interface ip set dnsservers xx static 127.0.0.1 192.168.9.102
	CmdHide("netsh", "interface",netType,"set","dnsservers",iName,"static",ip).Output()
	for _,v:=range dnsServers{
		CmdHide("netsh", "interface",netType,"add","dnsservers",iName,v).Output()
	}
}


func ResetNetConf(dnsAddr string){
	if _watchIpChange!=nil {
		_watchIpChange.Shutdown();
	}
	if oneAdapter!=nil {
		_resetDns("ip", oneAdapter.dHCPEnabled,oneAdapter.gwIp);
		if oneAdapter.isIPv6 {
			_resetDns("ipv6", oneAdapter.dHCPEnabled,oneAdapter.gwIp);
		}
	}
}

func _resetDns(netType string,dHCPEnabled bool,gwIp string){
	var iName=gwIpToName(gwIp);
	dnsServers,_,_:=GetDnsServerByGateWay(gwIp);
	//dhcp
	if dHCPEnabled {
		CmdHide("netsh", "interface",netType,"set","dnsservers",iName,"dhcp").Output()
	}else {
		i:=0;
		for _,v:=range dnsServers{
			if v=="127.0.0.1"{
				continue;
			}
			if i==0 {
				CmdHide("netsh", "interface", netType, "set", "dnsservers", iName, "static", v).Output()
			}else {
				CmdHide("netsh", "interface", netType, "add", "dnsservers", iName, v).Output()
			}
			i++;
		}
	}
}


func GetDnsServerByGateWay(gwIp string)([]string,bool,bool){
	//DNSServerSearchOrder
	adapters,err:=GetNetworkAdapter()
	var isIpv6=false;
	if err!=nil {
		return nil,false,isIpv6;
	}
	for _,v:=range adapters{
		if len(v.DefaultIPGateway)>0&&v.DefaultIPGateway[0]==gwIp {
			for _,v2:=range v.IPAddress{
				if len(v2)>16{
					isIpv6=true;
					break;
				}
			}
			return v.DNSServerSearchOrder,v.DHCPEnabled,isIpv6;
		}
	}
	return nil,false,isIpv6;
}


type NetworkAdapter struct {
	DNSServerSearchOrder   []string
	DefaultIPGateway []string
	IPAddress []string
	Caption    string
	DHCPEnabled  bool
	ServiceName  string
	IPSubnet   []string
	SettingID string
}

func GetWaterConf(tunAddr string,tunMask string)water.Config{
	masks:=net.ParseIP(tunMask).To4();
	maskAddr:=net.IPNet{IP: net.ParseIP(tunAddr), Mask: net.IPv4Mask(masks[0], masks[1], masks[2], masks[3] )}
	return  water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID:   "tap0901",
			//	InterfaceName: "Ethernet 3",
			Network:       maskAddr.String(),
		},
	}
}

func GetNetworkAdapter() ([]NetworkAdapter,error){
	var s = []NetworkAdapter{}
	err := wmi.Query("SELECT Caption,SettingID,DNSServerSearchOrder,DefaultIPGateway,ServiceName,IPAddress,IPSubnet,DHCPEnabled       FROM Win32_NetworkAdapterConfiguration WHERE IPEnabled=True", &s) // WHERE (BIOSVersion IS NOT NULL)
	if err != nil {
		log.Printf("err:%v\r\n",err)
		return nil,err
	}
	return s,nil;
}


func AddRoute(tunAddr string, tunGw string, tunMask string) error {
	var netNat =make([]string,4);
	//masks:=strings.Split(tunMask,".")
	masks:=net.ParseIP(tunMask).To4();
	Addrs:=strings.Split(tunAddr,".")
	for i := 0; i <= 3; i++ {
		if masks[i]==255 {
			netNat[i]=Addrs[i];
		}else{
			netNat[i]="0";
		}
	}
	maskAddr:=net.IPNet{IP: net.ParseIP(tunAddr), Mask: net.IPv4Mask(masks[0], masks[1], masks[2], masks[3] )}
	maskAddrs:=strings.Split(maskAddr.String(),"/")
	lAdds,err:=GetLocalAddresses();
	var iName="";
	if err==nil {
		for _, v := range lAdds {
			if strings.Index(v.IpAddress,tunAddr)!=-1 {
				iName=v.Name;
				break;
			}
		}
	}

	//clear old
	CmdHide("route", "delete",strings.Join(netNat,".")).Output()
	cmd:=CmdHide("netsh", "interface","ipv4","add","route",strings.Join(netNat,".")+"/"+maskAddrs[1],iName,tunGw,"metric=6","store=active")
	fmt.Printf("cmd:%s\r\n",cmd.Args)
	cmd.Run();


	fmt.Printf("cmd:%s\r\n",strings.Join(cmd.Args," "))
	CmdHide("ipconfig", "/flushdns").Run()
	return nil;
}

func Ipv6Switch(open bool)error{
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, "SYSTEM\\CurrentControlSet\\Services\\TCPIP6\\Parameters", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer key.Close()
	if open {
		key.SetDWordValue("DisabledComponents", 0x00)
	}else{
		key.SetDWordValue("DisabledComponents", 0x00000020)
	}
	return nil;
}

func CmdHide(name string, arg ...string) *exec.Cmd{
	cmd:=exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd;
}