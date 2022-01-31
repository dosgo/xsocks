package restapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/dosgo/xsocks/client"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
)

var socksX_cli *client.Client
var logOutFun func()

func apiAction(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if r.Form.Get("cmd") == "save" {
		err := SaveConf(r.Form.Get("jsonStr"))
		if err != nil {
			w.Write([]byte("dd"))
		}
	}
	if r.Form.Get("cmd") == "read" {
		conf, err := ReadConf()
		if err == nil {
			w.Write(conf)
		}
	}
	if r.Form.Get("cmd") == "run" {
		os.WriteFile("out.log", []byte(""), os.ModePerm)
		socksX_cli.Start()
	}
	if r.Form.Get("cmd") == "stop" {
		socksX_cli.Shutdown()
	}
	if r.Form.Get("cmd") == "console" {
		line, _ := strconv.Atoi(r.Form.Get("line"))
		outTxt := comm.Tail("out.log", line)
		back := make(map[string]interface{})
		back["out"] = outTxt
		back["code"] = 0
		back["msg"] = ""
		jsonBack(w, back)
	}
	if r.Form.Get("cmd") == "clearConsole" {
		os.WriteFile("out.log", []byte(""), os.ModePerm)
	}
}
func init() {
	logOutFun = comm.LogOutput("")
}

func Start() {
	if logOutFun != nil {
		defer logOutFun()
	}
	socksX_cli = &client.Client{}
	defer socksX_cli.Shutdown()
	http.HandleFunc("/api", apiAction)
	log.Fatal(http.ListenAndServe(":10000", nil))
}

var configFile = "config.json"

func SaveConf(jsonStr string) error {
	paramParam := param.Args
	err := json.NewDecoder(strings.NewReader(jsonStr)).Decode(&paramParam)
	if err != nil {
		return err
	}
	fp, err := os.OpenFile(configFile, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err == nil {
		err = json.NewEncoder(fp).Encode(paramParam)
	}
	return err
}

func ReadConf() ([]byte, error) {
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
	if err == nil && err1 == nil {
		confParam := &param.ArgsParam{}
		err = json.NewDecoder(bytes.NewReader(msgStr)).Decode(&confParam)
		if err == nil {
			paramParam.Sock5Addr = confParam.Sock5Addr
			paramParam.Password = confParam.Password
			paramParam.ServerAddr = confParam.ServerAddr
			paramParam.SkipVerify = confParam.SkipVerify
			paramParam.TunType = confParam.TunType
			paramParam.UdpProxy = confParam.UdpProxy
		}
	} else {
		fp, err := os.OpenFile(configFile, os.O_CREATE|os.O_RDWR, os.ModePerm)
		if err == nil {
			json.NewEncoder(fp).Encode(paramParam)
		}
	}
	return json.Marshal(&paramParam)
}

func jsonBack(w http.ResponseWriter, data map[string]interface{}) {
	buf, err := json.Marshal(data)
	if err == nil {
		w.Write(buf)
	}
}
