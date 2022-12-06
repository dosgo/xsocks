package comm

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dosgo/xsocks/param"
	"github.com/yl2chen/cidranger"
)

var (
	gChinaMainlandRange cidranger.Ranger
)

func Init() {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	name := "iptable.txt"
	if param.Args.IpFile != "" {
		name = param.Args.IpFile
	}
	if err == nil && runtime.GOOS != "windows" {
		name = dir + "/" + name
	}
	if !exists(name) {
		downloadIPTable(name)
	}
	gChinaMainlandRange = loadLookupTable(name)
}

func loadLookupTable(name string) cidranger.Ranger {
	ranger := cidranger.NewPCTrieRanger()
	fi, err := os.Open(name)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return nil
	}
	defer fi.Close()

	br := bufio.NewReader(fi)
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		_, network, _ := net.ParseCIDR(string(a))
		entry := cidranger.NewBasicRangerEntry(*network)
		ranger.Insert(entry)
	}
	return ranger
}

func IsChinaMainlandIP(IP string) bool {
	//anti-fraud ip
	if IP == "182.43.124.6" {
		return false
	}
	ipp := net.ParseIP(IP)
	if gChinaMainlandRange != nil {
		contains, err := gChinaMainlandRange.Contains(ipp)
		if err != nil {
			fmt.Printf("to query ip is  %s failed %v", IP, err)
			return false
		}
		return contains
	}
	return false
}

func downloadIPTable(name string) error {
	uri := "https://raw.githubusercontent.com/17mon/china_ip_list/master/china_ip_list.txt"
	resp, err := http.Get(uri)
	if err != nil {
		fmt.Printf("sending Rio server request failed %s\r\n", err.Error())
		return nil
	}
	defer resp.Body.Close()
	nowResp, _ := io.ReadAll(resp.Body)
	if len(nowResp) > 0 {
		os.WriteFile(name, nowResp, 0644)
	}
	return nil
}

// Exists file exist
func exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}
