package tunnelcomm

import (
	"errors"
	"strings"

	"github.com/dosgo/xsocks/comm"
)

type TunelComm struct {
	TunnelUrl    string
	Password     string
	CallerTunnel CallerTunnel
}

type CallerTunnel interface {
	Dial(url string) (comm.CommConn, error)
}

func (tunnel *TunelComm) NewTunnel() (comm.CommConn, error) {
	if tunnel.CallerTunnel == nil {
		return nil, errors.New("Tunnel init error")
	}
	//解析
	stream, err := tunnel.CallerTunnel.Dial(tunnel.TunnelUrl)
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return nil, errors.New("stream null")
	}
	//write password
	passwordBuf := comm.GenPasswordHead(tunnel.Password)
	_, err = stream.Write([]byte(passwordBuf))
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func (tunnel *TunelComm) ResetTunnel() {
	if strings.HasPrefix(tunnel.TunnelUrl, "wss") {

	}
	if strings.HasPrefix(tunnel.TunnelUrl, "quic") {
		//tunnel.CallerTunnel.ClearQuicDialer()
	}
	if strings.HasPrefix(tunnel.TunnelUrl, "kcp") {

	}
}
