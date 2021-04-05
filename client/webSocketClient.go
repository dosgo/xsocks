package client

import (
	"fmt"
	"golang.org/x/net/websocket"
	"io"
	"github.com/dosgo/xSocks/client/httpcomm"
	"github.com/dosgo/xSocks/client/muxComm"
	"github.com/dosgo/xSocks/param"
)
var wsYamuxDialer *muxComm.YamuxComm
func init(){
	wsYamuxDialer= muxComm.NewYamuxDialer(dialWs)
}

func NewWsYamuxDialer()  *muxComm.YamuxComm{
	return wsYamuxDialer;
}

func dialWs(url string)(io.ReadWriteCloser, error){
	config, err := websocket.NewConfig(url, url)
	if err != nil {
		fmt.Printf("webSocketUrl:%s err:%v\r\n",url,err)
		return nil,err;
	}
	config.TlsConfig=httpcomm.GetTlsConf();
	config.Header.Add("token",param.Args.Password)
	ws, err := websocket.DialConfig(config)
	if err != nil {
		return nil,err;
	}
	return ws,err;
}

