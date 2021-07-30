package muxComm

import (
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
	"github.com/xtaci/smux"
	"math/rand"
	"sync"
	"time"
)


type SmuxComm struct {
	sess   [] *smux.Session
	dialFc DialConn
	sync.Mutex
}

func NewWsSmuxDialer(conn DialConn) *SmuxComm {
	_smux:= &SmuxComm{}
	_smux.dialFc=conn;
	return  _smux
}



func (qd *SmuxComm) Dial(url string,remoteAddr string) (comm.CommConn, error) {
	qd.Lock()
	defer qd.Unlock()

	conf:=smux.DefaultConfig();
	conf.KeepAliveInterval=59* time.Second;
	conf.KeepAliveTimeout=60*time.Second;

	if param.Args.MuxNum==0 {
		wsConn,err:=qd.dialFc(url);
		if err!=nil {
			return nil,err;
		}
		session, err := smux.Client(wsConn, conf)
		if err!=nil {
			return nil,err;
		}
		return session.OpenStream()
	}



	if len(qd.sess) < param.Args.MuxNum {
		wsConn,err:=qd.dialFc(url);
		if err!=nil {
			return nil,err;
		}
		session, err := smux.Client(wsConn, conf)
		if err!=nil {
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
		wsConn,err:=qd.dialFc(url);
		if err!=nil {
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
