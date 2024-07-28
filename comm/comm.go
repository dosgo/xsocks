package comm

import (
	"crypto/md5"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

func GenPasswordHead(password string) string {
	h := md5.New()
	h.Write([]byte(password))
	md5Str := hex.EncodeToString(h.Sum(nil))
	return md5Str[:16]
}

func GetFreePort() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return "0", err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "0", err
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port), nil
}

func GetFreeUdpPort() (string, error) {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return "0", err
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.LocalAddr().(*net.UDPAddr).Port), nil
}

// 生成Guid字串
func UniqueId(_len int) string {
	b := make([]byte, 48)
	if _, err := io.ReadFull(crand.Reader, b); err != nil {
		return ""
	}
	h := md5.New()
	h.Write([]byte(b))
	return hex.EncodeToString(h.Sum(nil))[:_len]
}

type lAddr struct {
	Name       string
	IpAddress  string
	IpMask     string
	GateWay    string
	MACAddress string
}

func GetNetworkInfo() ([]lAddr, error) {
	intf, err := net.Interfaces()
	lAddrs := []lAddr{}
	if err != nil {
		log.Printf("get network info failed: %v", err)
		return nil, err
	}
	for _, v := range intf {
		ips, err := v.Addrs()
		if err != nil {
			log.Printf("get network addr failed: %v", err)
			return nil, err
		}
		//此处过滤loopback（本地回环）和isatap（isatap隧道）
		if !strings.Contains(v.Name, "Loopback") && !strings.Contains(v.Name, "isatap") {
			itemAddr := lAddr{}
			itemAddr.Name = v.Name
			itemAddr.MACAddress = v.HardwareAddr.String()
			for _, ip := range ips {
				if strings.Contains(ip.String(), ".") {
					_, ipNet, err1 := net.ParseCIDR(ip.String())
					if err1 == nil {
						itemAddr.IpAddress = ipNet.IP.String()
						itemAddr.IpMask = net.IP(ipNet.Mask).String()
					}
				}
			}
			lAddrs = append(lAddrs, itemAddr)
		}
	}
	return lAddrs, nil
}

func InitLog(_logfile string, flag int) {
	logfile := _logfile
	if _logfile == "" {
		logfile = "out.log"
	}
	logFile, err := os.OpenFile(logfile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err == nil {
		mw := io.MultiWriter(logFile)
		if ExistStdOutput() {
			mw = io.MultiWriter(logFile, os.Stdout)
		}
		log.SetOutput(mw)
	}
	log.SetFlags(flag)
}
