package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	quic "github.com/lucas-clemente/quic-go"
	"log"
	"math/big"
	"net"
	"time"
	"github.com/dosgo/xsocks/comm/udpHeader"
	"github.com/dosgo/xsocks/param"
)

func StartQuic(_addr string) error {
	var quicConfig = &quic.Config{
		MaxIncomingStreams:                    72,
		MaxIncomingUniStreams:                 -1,              // disable unidirectional streams
		KeepAlive: true,
		MaxReceiveStreamFlowControlWindow:5*1024*1024,
		MaxIdleTimeout:time.Minute*5,
		MaxReceiveConnectionFlowControlWindow:5*1024*1024,
	}
	addr, err := net.ResolveUDPAddr("udp", _addr)
	if err != nil {
		log.Println(err)
		return err;
	}
	_conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err;
	}
	//udp fob
	conn:=udpHeader.NewUdpConn(_conn);
	defer conn.Close()
	listener, err := quic.ListenEarly(conn,  generateTLSConfig(), quicConfig)
	if err != nil {
		log.Printf("err:%v\r\n",err)
		return err
	}
	for {
		sess, err := listener.Accept(context.Background())
		if err != nil {
			log.Printf("err:%v\r\n",err)
			return err
		}
		go quicToSocks5(sess)
	}
	return err
}
/* to socks server*/
func quicToSocks5(sess quic.Session) {
	for {
		stream, err := sess.AcceptStream(context.Background())
		if err != nil {
			sess.CloseWithError(2020, "AcceptStream error")
			log.Printf("err:%v\r\n",err)
			return ;
		}
		go proxy(stream)
	}
}





func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{param.Args.Password,"quic-echo-example"},
	}
}