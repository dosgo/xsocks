package muxComm

import (
	"github.com/hashicorp/yamux"
	"io"
	"math/rand"
	"time"
	"xSocks/comm"
	"sync"
	"log"
	"errors"
	"xSocks/param"
)
type YamuxComm struct {
	sess           []*yamux.Session
	dialFc  DialConn
	sync.Mutex
}

func NewYamuxDialer(conn DialConn) *YamuxComm {
	_yamux:= &YamuxComm{}
	_yamux.dialFc=conn;
	return  _yamux
}

func (qd *YamuxComm) Dial(url string) (comm.CommConn, error) {
	qd.Lock()
	defer qd.Unlock()
	if qd.sess==nil {
		qd.sess=make([]*yamux.Session,0)
	}
	if param.MuxNum==0 {
		wsConn,err:=qd.dialFc(url)
		if err!=nil {
			log.Printf("err:%v\r\n",err);
			return nil,err;
		}
		session,err:=newYamuxSession(wsConn);
		if err!=nil {
			log.Printf("err:%v\r\n",err);
			return nil,err;
		}
		return session.Open()
	}else{
		if len(qd.sess) < param.MuxNum {
			wsConn,err:=qd.dialFc(url)
			if err!=nil {
				log.Printf("err:%v\r\n",err);
				return nil,err;
			}
			session,err:=newYamuxSession(wsConn);
			if err==nil {
				qd.sess=append(qd.sess,session)
			}else{
				log.Printf("err:%v\r\n",err);
			}
		}
		if len(qd.sess)<1 {
			return nil,errors.New("sess null");
		}
		var sessIndex=0;
		var retryNum=0;
		for {
			if retryNum>3 {
				break;
			}
			sessIndex=rand.Intn(len(qd.sess))
			sess:=qd.sess[sessIndex]
			if sess.IsClosed()  {
				qd.sess=append(qd.sess[:sessIndex], qd.sess[sessIndex+1:]...)
				sess.Close()
				wsConn,err:=qd.dialFc(url)
				if err!=nil {
					log.Printf("err:%v\r\n",err);
					return nil,err;
				}
				session,err:=newYamuxSession(wsConn);
				if err==nil {
					qd.sess=append(qd.sess,session)
				}else{
					log.Printf("err:%v\r\n",err);
				}
				retryNum++;
				continue;
			}
			stream, err :=sess.Open()
			if err!=nil  {
				qd.sess=append(qd.sess[:sessIndex], qd.sess[sessIndex+1:]...)
				sess.Close()
				wsConn,err:=qd.dialFc(url)
				if err!=nil {
					log.Printf("err:%v\r\n",err);
					return nil,err;
				}
				session,err:=newYamuxSession(wsConn);
				if err==nil {
					qd.sess=append(qd.sess,session)
				}else{
					log.Printf("err:%v\r\n",err);
				}
				retryNum++;
				continue;
			}
			return stream, nil
		}
		return nil,errors.New("retryNum>3");
	}
}

func newYamuxSession(conn io.ReadWriteCloser)(*yamux.Session, error){
	conf:=yamux.DefaultConfig();
	conf.AcceptBacklog=512;
	conf.KeepAliveInterval=52* time.Second;
	conf.MaxStreamWindowSize=1024*1024;
	conf.ConnectionWriteTimeout=20* time.Second;
	return yamux.Client(conn, conf)
}
