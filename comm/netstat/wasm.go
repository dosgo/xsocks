// +build wasm

package netstat

/**/
func PortGetPid(lSocks string) (int, error) {
	return 0, nil
}

func IsSocksServerAddr(pid int,addr string)bool{
	return false;
}
