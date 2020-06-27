package tunnel

import (
	"qproxy/client"
	"qproxy/comm"
	"qproxy/param"
	"strings"
)


func  NewTunnel () (comm.CommConn,error){
	var err error;
	//解析
	var stream comm.CommConn;
	if (strings.HasPrefix(param.ServerAddr,"wss")) {
		stream, err = client.NewWsYamuxDialer().Dial(param.ServerAddr)
	}
	if (strings.HasPrefix(param.ServerAddr,"quic")) {
		stream, err = client.NewQuicDialer().Dial(param.ServerAddr[7:])
	}
	if err != nil || stream == nil {
		return nil,err
	}
	//write password
	passwordBuf := client.GenPasswordHead(param.Password);
	stream.Write([]byte(passwordBuf))
	return stream,nil;
}

