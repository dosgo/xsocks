package client

import (
	"github.com/dosgo/xsocks/client/tunnel"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
	"errors"
	"strings"
)

func  NewTunnel () (comm.CommConn,error){
	var err error;
	//解析
	var stream comm.CommConn;
	if strings.HasPrefix(param.Args.ServerAddr,"wss") {
		stream, err = tunnel.NewWsYamuxDialer().Dial(param.Args.ServerAddr)
	}else if strings.HasPrefix(param.Args.ServerAddr,"http2") {
		stream, err = tunnel.NewHttp2Dialer().Dial("https"+param.Args.ServerAddr[5:])
	}else if strings.HasPrefix(param.Args.ServerAddr,"http") {
		stream, err = tunnel.NewHttpDialer().Dial("https"+param.Args.ServerAddr[4:])
	}else if strings.HasPrefix(param.Args.ServerAddr,"quic") {
		stream, err = tunnel.NewQuicDialer().Dial(param.Args.ServerAddr[7:])
	}else if strings.HasPrefix(param.Args.ServerAddr,"kcp") {
		stream, err = tunnel.NewKcpDialer().Dial(param.Args.ServerAddr[6:])
	}


	if err != nil  {
		return nil,err
	}
	if  stream == nil {
		return nil,errors.New("stream null")
	}
	//write password
	passwordBuf := comm.GenPasswordHead(param.Args.Password);
	_,err=stream.Write([]byte(passwordBuf))
	if err != nil  {
		return nil,err
	}
	return stream,nil;
}


func  ResetTunnel () {
	if strings.HasPrefix(param.Args.ServerAddr,"wss") {

	}
	if strings.HasPrefix(param.Args.ServerAddr,"quic") {
		tunnel.ClearQuicDialer();
	}
	if strings.HasPrefix(param.Args.ServerAddr,"kcp") {

	}
}