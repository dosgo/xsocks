package server

import (
	"crypto/md5"
	"crypto/sha1"
	kcp"github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
	"golang.org/x/crypto/pbkdf2"
	"net"
	"time"
	"xSocks/param"
	_ "xSocks/param"
)



func StartKcp(addr string) error {
	salt:=md5.Sum([]byte(param.Password))
	key := pbkdf2.Key([]byte(param.Password), salt[:], 1024, 32, sha1.New)
	block, _ := kcp.NewAESBlockCrypt(key)
	if listener, err := kcp.ListenWithOptions(addr, block, 10, 3); err == nil {
		for {
			s, err := listener.AcceptKCP()
			if err != nil {
				return err;
			}
			go kcpToSocks5(s)
		}
	} else {
		return err;
	}
}

func kcpToSocks5(conn net.Conn){
	conf:=smux.DefaultConfig();
	conf.KeepAliveInterval=59* time.Second;
	conf.KeepAliveTimeout=60 * time.Second;
	// Setup server side of yamux
	session, err := smux.Server(conn, conf)
	if err != nil {
		return;
	}
	for {
		// Accept a stream
		stream, err := session.AcceptStream()
		if err != nil {
			return ;
		}
		go proxy(stream)
	}
}
