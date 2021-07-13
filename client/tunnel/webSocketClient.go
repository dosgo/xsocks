package tunnel

import (
	"fmt"
	"github.com/dosgo/xsocks/client/tunnel/muxComm"
	"github.com/dosgo/xsocks/param"
	"golang.org/x/net/websocket"
	"io"
	"strings"
)
var wsYamuxDialer *muxComm.YamuxComm
func init(){
	wsYamuxDialer = muxComm.NewYamuxDialer(dialWs)
}

func NewWsYamuxDialer()  *muxComm.YamuxComm {
	return wsYamuxDialer;
}

func dialWs(url string)(io.ReadWriteCloser, error){
	var origin="";
	if strings.HasPrefix(url,"wss") {
		 origin = strings.Replace(url, "wss:", "https:", -1);
	}else {
		 origin = strings.Replace(url, "ws:", "http:", -1);
	}
	config, err := websocket.NewConfig(url, origin)
	if err != nil {
		fmt.Printf("webSocketUrl:%s err:%v\r\n",url,err)
		return nil,err;
	}
	config.TlsConfig= GetTlsConf();
	config.Header.Add("token",param.Args.Password)
	ws, err := websocket.DialConfig(config)
	if err != nil {
		return nil,err;
	}
	return ws,err;
}

