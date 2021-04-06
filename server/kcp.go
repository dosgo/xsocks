package server

import (
	"crypto/md5"
	"crypto/sha1"
	kcp "github.com/xtaci/kcp-go/v5"
	"golang.org/x/crypto/pbkdf2"
	"github.com/dosgo/xsocks/param"
	_ "github.com/dosgo/xsocks/param"
)



func StartKcp(addr string) error {
	salt:=md5.Sum([]byte(param.Args.Password))
	key := pbkdf2.Key([]byte(param.Args.Password), salt[:], 1024, 32, sha1.New)
	block, _ := kcp.NewAESBlockCrypt(key)
	if listener, err := kcp.ListenWithOptions(addr, block, 10, 3); err == nil {
		for {
			s, err := listener.AcceptKCP()
			if err != nil {
				return err;
			}
			go streamToSocks5Smux(s)
		}
	} else {
		return err;
	}
}


