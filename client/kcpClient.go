package client

import (
	"crypto/md5"
	"crypto/sha1"
	"github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
	"golang.org/x/crypto/pbkdf2"
	"net"
	"sync"
	"xSocks/comm"
	"xSocks/param"
)
var kcpDialer *KcpInfo
func init(){
	kcpDialer =&KcpInfo{}
}


type KcpInfo struct {
	sess           *smux.Session
	sync.Mutex
}

func NewKcpDialer() *KcpInfo {
	return kcpDialer
}


func (qd *KcpInfo) Dial(url string) (comm.CommConn, error) {
	qd.Lock()
	defer qd.Unlock()

	conf:=smux.DefaultConfig();
	//conf.KeepAliveInterval=59* time.Second;
	//conf.KeepAliveTimeout=60*time.Second;

	if qd.sess == nil||param.Mux!=1 {
		wsConn,err:=connectKcp(url);
		if(err!=nil){
			return nil,err;
		}
		session, err := smux.Client(wsConn, conf)
		if(err!=nil){
			return nil,err;
		}
		qd.sess=session;
	}

	// Open a new stream
	stream, err := qd.sess.OpenStream()
	if err != nil {
		qd.sess.Close()
		wsConn,err:=connectKcp(url);
		if(err!=nil){
			return nil,err;
		}
		session, err := smux.Client(wsConn, conf)
		if err != nil {
			return nil,err;
		}
		qd.sess=session;
		stream, err = qd.sess.OpenStream()
		if err != nil {
			return nil, err
		}
	}
	return stream, nil
}


func connectKcp(url string)(net.Conn, error){
	salt:=md5.Sum([]byte(param.Password))
	key := pbkdf2.Key([]byte(param.Password), salt[:], 1024, 32, sha1.New)
	block, _ := kcp.NewAESBlockCrypt(key)
	// dial to the echo server
	return kcp.DialWithOptions(url, block, 10, 3);
}

