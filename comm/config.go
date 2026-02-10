package comm

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/dosgo/xsocks/param"
)

func ReadConf(configFile string) ([]byte, bool, error) {
	paramParam := param.Args
	paramParam.Sock5Addr = "127.0.0.1:6000"
	paramParam.ServerAddr = "wss://127.0.0.1:5003"
	paramParam.Password = "password"
	paramParam.SkipVerify = false
	paramParam.TunType = 3
	paramParam.MuxNum = 4
	paramParam.LocalDns = 0
	paramParam.SmartDns = 1
	paramParam.UdpProxy = 1
	paramParam.Mtu = 4500
	paramParam.TunSmartProxy = false
	_, err := os.Stat(configFile)
	msgStr, err1 := os.ReadFile(configFile)
	isConf := false
	if err == nil && err1 == nil {
		confParam := &param.ArgsParam{}
		err = json.NewDecoder(bytes.NewReader(msgStr)).Decode(&confParam)
		if err == nil {
			isConf = true
			paramParam.Sock5Addr = confParam.Sock5Addr
			paramParam.Password = confParam.Password
			paramParam.ServerAddr = confParam.ServerAddr
			paramParam.SkipVerify = confParam.SkipVerify
			paramParam.TunType = confParam.TunType
			paramParam.UdpProxy = confParam.UdpProxy
			paramParam.ExcludeDomain = confParam.ExcludeDomain
		}
	} else {
		fp, err := os.OpenFile(configFile, os.O_CREATE|os.O_RDWR, os.ModePerm)
		if err == nil {
			json.NewEncoder(fp).Encode(paramParam)
		}
	}
	data, err := json.Marshal(&paramParam)
	return data, isConf, err
}
