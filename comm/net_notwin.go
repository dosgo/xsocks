//go:build !windows
// +build !windows

package comm

import (
	"net"
	"os/exec"
	"strings"
)

var oldDns = ""

func GetGateway() string {
	return ""
}

func AddRoute(tunAddr string, tunGw string, tunMask string) {

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

	//route add –net IP netmask MASK gw IP
	//route add -net 192.168.2.0/24 gw 192.168.3.254

	//clear old
	cmd1 := CmdHide("route", "delete", "-net", strings.Join(netNat, ".")+"/"+maskAddrs[1])
	//log.Printf("cmd.args:%s\r\n",cmd1.Args)
	cmd1.Run()
	cmd := CmdHide("route", "add", "-net", strings.Join(netNat, ".")+"/"+maskAddrs[1], "gw", tunAddr)
	//log.Printf("cmd.args:%s\r\n",cmd.Args)
	cmd.Run()
}

func CmdHide(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

func ExistStdOutput() bool {
	return true
}
