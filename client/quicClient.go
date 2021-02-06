package client

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/lucas-clemente/quic-go"
	"log"
	"net"
	"sync"
	"xSocks/comm"
	"xSocks/comm/udpHeader"
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

func (qd *QuicDialer) Connect(quicAddr string) error{
	qd.Lock();
	defer qd.Unlock();
	var quicConfig = &quic.Config{
		MaxIncomingStreams:                    100,
		MaxIncomingUniStreams:                 100,              // disable unidirectional streams
		KeepAlive: true,
	}
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	udpAddr, err := net.ResolveUDPAddr("udp", quicAddr)
	if err != nil {
		return  err
	}
	_udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return  err
	}
	//udp fob
	udpConn := udpHeader.NewUdpConn(_udpConn);

	sess, err := quic.Dial(udpConn,udpAddr,quicAddr,tlsConf, quicConfig)
	if err != nil {
		log.Printf("err:%v\r\n",err)
		return err
	}
	qd.sess = sess
	return nil;
}

func (qd *QuicDialer) GetSess() quic.Session{
	qd.Lock();
	defer qd.Unlock();
	return qd.sess
}


func (qd *QuicDialer) Dial(quicAddr string) (comm.CommConn, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var retryNum=0;
	for{
		if retryNum>3 {
			break;
		}
		sess:=qd.GetSess();
		if sess==nil {
			qd.Connect(quicAddr);
			retryNum++;
			continue;
		}
		stream, err := qd.sess.OpenStreamSync(ctx)
		if err != nil {
			qd.sess.CloseWithError(2021, "OpenStreamSync error")
			qd.Connect(quicAddr);
			retryNum++;
			continue;
		}
		return stream, nil
	}
	return nil,errors.New("retryNum>3")
}

