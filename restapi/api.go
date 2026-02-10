package restapi

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/dosgo/xsocks/client"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
)

var socksX_cli *client.Client
var token = ""
var configFile = "config.json"

func apiAction(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if token != "" && r.Form.Get("token") != token {
		back := make(map[string]interface{})
		back["code"] = -1
		back["msg"] = "token error"
		jsonBack(w, back)
		return
	}
	if r.Form.Get("cmd") == "save" {
		err := SaveConf(r.Form.Get("jsonStr"))
		if err != nil {
			w.Write([]byte("dd"))
		}
	}
	if r.Form.Get("cmd") == "read" {
		conf, _, err := comm.ReadConf(configFile)
		param.Args.AutoStart = IsAutoStart(param.Args.ServerAddr)
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
	//写日志
	if r.Form.Get("cmd") != "console" {
		log.Printf(r.URL.String() + "cmd:" + r.Form.Get("cmd") + "\r\n")
	}
}

func Start(port string, _token string) {
	token = _token
	socksX_cli = &client.Client{}
	defer socksX_cli.Shutdown()
	http.HandleFunc("/api", apiAction)
	if port == "" {
		port, _ = comm.GetFreePort()
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

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

func jsonBack(w http.ResponseWriter, data map[string]interface{}) {
	buf, err := json.Marshal(data)
	if err == nil {
		w.Write(buf)
	}
}

/*是否自动启动*/
func IsAutoStart(serverAddr string) bool {
	_, err := os.Stat(configFile)
	if err == nil {
		urlInfo, err := url.Parse(serverAddr)
		if err == nil {
			if urlInfo.Scheme == "wss" || urlInfo.Scheme == "http2" || urlInfo.Scheme == "socks5" {
				if comm.CheckTcp(urlInfo.Hostname(), urlInfo.Port()) {
					return true
				}
			} else {
				return true
			}
		}
	}
	return false
}
