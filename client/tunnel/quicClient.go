package tunnel

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	socksTapComm "github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/comm/udpHeader"
	"github.com/quic-go/quic-go"
)

var quicDialer *QuicDialer

func init() {
	quicDialer = &QuicDialer{}
}

var num int64 = 0

type QuicDialer struct {
	sess    quic.Connection
	udpConn *udpHeader.UdpConn
	sync.Mutex
}

func NewQuicDialer() *QuicDialer {
	return quicDialer
}

func ClearQuicDialer() {
	sess := quicDialer.GetSess()
	if sess != nil {
		sess.CloseWithError(2021, "deadlocks error close")
	}
}

func (qd *QuicDialer) Connect(quicAddr string) error {
	qd.Lock()
	defer qd.Unlock()
	if qd.sess != nil {
		qd.sess.CloseWithError(2021, "OpenStreamSync error")
	}
	var maxIdleTimeout = time.Minute * 5

	var quicConfig = &quic.Config{
		//	MaxIncomingStreams:                    32,
		//	MaxIncomingUniStreams:                 -1,              // disable unidirectional streams
		Versions:       []quic.VersionNumber{quic.Version1},
		MaxIdleTimeout: maxIdleTimeout,
	}

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-xsocks", "quic-echo-example"},
	}
	udpAddr, err := net.ResolveUDPAddr("udp", quicAddr)
	if err != nil {
		log.Printf("err:%v\r\n", err)
		return err
	}
	_udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		log.Printf("err:%v\r\n", err)
		return err
	}
	//udp fob
	udpConn := udpHeader.NewUdpConn(_udpConn)

	sess, err := quic.DialEarly(context.Background(), udpConn, udpAddr, tlsConf, quicConfig)
	if err != nil {
		log.Printf("err:%v udpAddr:%v _udpConn:%v\r\n", err, udpAddr, _udpConn)
		return err
	}
	qd.sess = sess
	qd.udpConn = udpConn
	atomic.StoreInt64(&num, 0)
	return nil
}

func (qd *QuicDialer) GetSess() quic.Connection {
	qd.Lock()
	defer qd.Unlock()
	return qd.sess
}

func isActive(s quic.Connection) bool {
	select {
	case <-s.Context().Done():
		return false
	default:
		return true
	}
}

func (qd *QuicDialer) Dial(quicAddr string) (socksTapComm.CommConn, error) {
	atomic.AddInt64(&num, 1)
	var retryNum = 0
	log.Printf("num:%d\r\n", num)
	for {
		if retryNum > 3 {
			break
		}
		sess := qd.GetSess()
		if sess == nil || !isActive(sess) {
			qd.Connect(quicAddr)
			retryNum++
			continue
		}
		stream, err := sess.OpenStream()
		if err != nil {
			log.Printf("err:%v\r\n", err)
			qd.Connect(quicAddr)
			retryNum++
			continue
		}
		return stream, nil
	}
	return nil, errors.New("retryNum>3")
}
