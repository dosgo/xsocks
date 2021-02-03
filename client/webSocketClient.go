package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/hashicorp/yamux"
	"github.com/xtaci/smux"
	"golang.org/x/net/websocket"
	"os"
	"math/rand"
	"errors"
	"sync"
	"time"
	"xSocks/comm"
	"xSocks/param"
)
var wsYamuxDialer *WsYamux
var wsSmuxDialer *WsSmux
func init(){
	wsYamuxDialer= &WsYamux{}
	wsSmuxDialer= &WsSmux{}
}

type WsYamux struct {
	sess           []*yamux.Session
	sync.Mutex
}

type WsSmux struct {
	sess          [] *smux.Session
	sync.Mutex
}



func NewWsYamuxDialer() *WsYamux {
	return wsYamuxDialer
}

func NewWsSmuxDialer() *WsSmux {
	return wsSmuxDialer
}

func (qd *WsYamux) Dial(url string) (comm.CommConn, error) {
	qd.Lock()
	defer qd.Unlock()
	conf:=yamux.DefaultConfig();
	conf.AcceptBacklog=512;
	conf.KeepAliveInterval=52* time.Second;
	conf.MaxStreamWindowSize=1024*1024;
	conf.ConnectionWriteTimeout=20* time.Second;

	if(qd.sess==nil){
		qd.sess=make([]*yamux.Session, 0)
	}

	if(param.MuxNum==0){
		wsConn,err:=dialAddr(url);
		if(err!=nil){
			return nil,err;
		}
		session, err := yamux.Client(wsConn, conf)
		if err != nil {
			return nil,err;
		}
		return session.Open()
	}else{
		if len(qd.sess) < param.MuxNum {
			wsConn, err := dialAddr(url);
			if (err != nil) {
				return nil, err;
			}
			session, err := yamux.Client(wsConn, conf)
			if err != nil {
				return nil, err;
			}
			qd.sess = append(qd.sess, session)
		}
		if(len(qd.sess)<1){
			return nil,errors.New("sess null");
		}
		sessIndex:=rand.Intn(len(qd.sess))
		sess:=qd.sess[sessIndex]
		// Open a new stream
		stream, err :=sess.Open()
		if err != nil {
			qd.sess=append(qd.sess[:sessIndex], qd.sess[sessIndex+1:]...)
			sess.Close()
			wsConn,err:=dialAddr(url);
			if(err!=nil){
				return nil,err;
			}
			session, err := yamux.Client(wsConn, conf)
			if(err!=nil){
				return nil,err;
			}
			qd.sess=append(qd.sess,session)
			stream, err = session.Open()
			if err != nil {
				return nil, err
			}
		}
		return stream, nil
	}
}


func (qd *WsSmux) Dial(url string) (comm.CommConn, error) {
	qd.Lock()
	defer qd.Unlock()

	conf:=smux.DefaultConfig();
	conf.KeepAliveInterval=59* time.Second;
	conf.KeepAliveTimeout=60*time.Second;

	if param.MuxNum==0 {
		wsConn,err:=dialAddr(url);
		if(err!=nil){
			return nil,err;
		}
		session, err := smux.Client(wsConn, conf)
		if(err!=nil){
			return nil,err;
		}
		return session.OpenStream()
	}



	if len(qd.sess) < param.MuxNum {
		wsConn,err:=dialAddr(url);
		if(err!=nil){
			return nil,err;
		}
		session, err := smux.Client(wsConn, conf)
		if(err!=nil){
			return nil,err;
		}
		qd.sess=append(qd.sess,session)
	}
	sessIndex:=rand.Intn(len(qd.sess))
	sess:=qd.sess[sessIndex]

	// Open a new stream
	stream, err := sess.OpenStream()
	if err != nil {
		sess.Close()
		wsConn,err:=dialAddr(url);
		if(err!=nil){
			return nil,err;
		}
		session, err := smux.Client(wsConn, conf)
		if err != nil {
			return nil,err;
		}
		qd.sess=append(qd.sess,session)
		stream, err = session.OpenStream()
		if err != nil {
			return nil, err
		}
	}
	return stream, nil
}


func dialAddr(url string)(*websocket.Conn, error){
	config, err := websocket.NewConfig(url, url)
	if err != nil {
		fmt.Printf("webSocketUrl:%s err:%v\r\n",url,err)
		return nil,err;
	}
	config.TlsConfig=getTlsConf();
	config.Header.Add("token",param.Password)
	ws, err := websocket.DialConfig(config)
	if err != nil {
		return nil,err;
	}
	return ws,err;
}


func getTlsConf()*tls.Config{
	tlsconf:=&tls.Config{InsecureSkipVerify:false,ClientSessionCache:  tls.NewLRUClientSessionCache(32)};
	if(param.CaFile!=""){
		_, err := os.Stat(param.CaFile)
		if err == nil {
			pool := x509.NewCertPool()
			caCrt, err := os.ReadFile(param.CaFile)
			if err != nil {
				return &tls.Config{};
			}
			pool.AppendCertsFromPEM(caCrt)
			tlsconf.RootCAs=pool;
			return tlsconf;
		}
	}
	if(param.SkipVerify){
		tlsconf.InsecureSkipVerify=true;
		return tlsconf;
	}
	return tlsconf;
}