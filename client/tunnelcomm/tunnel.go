package tunnelcomm

import (
	"errors"
	"strings"

	"github.com/dosgo/xsocks/client/tunnel"
	"github.com/dosgo/xsocks/comm"
)

var callerTunnel CallerTunnel
var _tunnelUrl string
var _password string
var _isProxy bool //socks5协议

type CallerTunnel interface {
	Dial(url string, remoteAddr string) (comm.CommConn, error)
}

func SetTunnel(_tunnel CallerTunnel, url string, password string, isProxy bool) {
	callerTunnel = _tunnel
	_tunnelUrl = url
	_password = password
	_isProxy = isProxy
}

func NewTunnel(remoteAddr string) (comm.CommConn, bool, error) {
	if callerTunnel == nil {
		return nil, false, errors.New("Tunnel init error")
	}
	//解析
	stream, err := callerTunnel.Dial(_tunnelUrl, remoteAddr)
	if err != nil {
		return nil, false, err
	}
	if stream == nil {
		return nil, false, errors.New("stream null")
	}
	//write password
	passwordBuf := comm.GenPasswordHead(_password)
	_, err = stream.Write([]byte(passwordBuf))
	if err != nil {
		return nil, false, err
	}
	return stream, _isProxy, nil
}

func ResetTunnel() {
	if strings.HasPrefix(_tunnelUrl, "wss") {

	}
	if strings.HasPrefix(_tunnelUrl, "quic") {
		tunnel.ClearQuicDialer()
	}
	if strings.HasPrefix(_tunnelUrl, "kcp") {

	}
}
