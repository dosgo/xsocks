//go:build wasm
// +build wasm

package netstat

/**/
func PortGetPid(lSocks string) (int, error) {
	return 0, nil
}

func IsUdpSocksServerAddr(pid int, addr string) bool {
	return false
}
