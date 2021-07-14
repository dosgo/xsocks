package tunnelcomm

import (
	"errors"
	"github.com/dosgo/xsocks/client/tunnel"
	"github.com/dosgo/xsocks/comm"
	"strings"
)

var callerTunnel CallerTunnel
var _tunnelUrl string
var _password string

type CallerTunnel interface{
	Dial(url string)(comm.CommConn, error)
}

func SetTunnel(_tunnel CallerTunnel,url string,password string){
	callerTunnel=_tunnel;
	_tunnelUrl=url;
	_password=password;
}

func  NewTunnel () (comm.CommConn,error){
	if callerTunnel==nil {
		return nil,errors.New("Tunnel init error")
	}
	//解析
	stream, err := callerTunnel.Dial(_tunnelUrl)
	if err != nil  {
		return nil,err
	}
	if  stream == nil {
		return nil,errors.New("stream null")
	}
	//write password
	passwordBuf := comm.GenPasswordHead(_password);
	_,err=stream.Write([]byte(passwordBuf))
	if err != nil  {
		return nil,err
	}
	return stream,nil;
}


func  ResetTunnel () {
	if strings.HasPrefix(_tunnelUrl,"wss") {

	}
	if strings.HasPrefix(_tunnelUrl,"quic") {
		tunnel.ClearQuicDialer();
	}
	if strings.HasPrefix(_tunnelUrl,"kcp") {

	}
}