package comm

import (
	"net"
	"time"
)


func CheckTcp(host string, port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second*1)
	if err == nil {
		if conn != nil {
			defer conn.Close()
			return true
		}
	}
	return false
}

