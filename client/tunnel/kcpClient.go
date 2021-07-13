package tunnel

import (
	"crypto/md5"
	"crypto/sha1"
	"github.com/dosgo/xsocks/client/tunnel/muxComm"
	"github.com/dosgo/xsocks/param"
	"github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
	"golang.org/x/crypto/pbkdf2"
	"io"
	"sync"
)
var kcpDialer *muxComm.SmuxComm
func init(){
	kcpDialer = muxComm.NewWsSmuxDialer(connectKcp)
}


type KcpInfo struct {
	sess           *smux.Session
	sync.Mutex
}

func NewKcpDialer() *muxComm.SmuxComm {
	return kcpDialer
}

func connectKcp(url string)(io.ReadWriteCloser, error){
	salt:=md5.Sum([]byte(param.Args.Password))
	key := pbkdf2.Key([]byte(param.Args.Password), salt[:], 1024, 32, sha1.New)
	block, _ := kcp.NewAESBlockCrypt(key)
	// dial to the echo server
	return kcp.DialWithOptions(url, block, 10, 3);
}

