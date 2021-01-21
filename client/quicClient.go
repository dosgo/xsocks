package client

import (
	"crypto/tls"
	"github.com/lucas-clemente/quic-go"
	"xSocks/comm"
	"context"
	"xSocks/param"
	"sync"
)
var quicDialer *QuicDialer

func init(){
	quicDialer= &QuicDialer{}
}



type QuicDialer struct {
	sess           quic.Session
	sync.Mutex
}

func NewQuicDialer() *QuicDialer {
	return quicDialer
}


func (qd *QuicDialer) Dial(quicAddr string) (comm.CommConn, error) {
	qd.Lock()
	defer qd.Unlock()

	var quicConfig = &quic.Config{
		MaxIncomingStreams:                    1000,
		MaxIncomingUniStreams:                 1000,              // disable unidirectional streams
		KeepAlive: true,
	}

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}

	if qd.sess == nil || param.Mux!=1{
		sess, err := quic.DialAddr(quicAddr, tlsConf, quicConfig)
		if err != nil {
			return nil, err
		}
		qd.sess = sess
	}

	stream, err := qd.sess.OpenStreamSync(context.Background())
	if err != nil {
		qd.sess.CloseWithError(2021, "OpenStreamSync error")
		sess, err := quic.DialAddr(quicAddr, tlsConf, nil)
		if err != nil {
			return nil, err
		}
		qd.sess = sess

		stream, err = qd.sess.OpenStreamSync(context.Background())
		if err != nil {
			return nil, err
		}
	}
	return stream, nil
}

