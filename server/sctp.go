package server

import (
	"fmt"
	"github.com/pion/logging"
	"github.com/pion/sctp"
	"github.com/pion/dtls"
	"net"
	"strconv"
	"strings"
	"time"
	"xSocks/param"
	_ "xSocks/param"
)



func StartSctp(addr string) error {
	addrs:=strings.Split(addr,":")
	_port, _ := strconv.Atoi(addrs[1])
	dTlsAddr := &net.UDPAddr{IP:net.ParseIP(addrs[0]), Port: _port}

	// Prepare the configuration of the DTLS connection
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			fmt.Printf("Client's hint: %s \n", hint)
			return []byte(param.Password), nil
			//return []byte{0xAB, 0xC1, 0x23}, nil
		},
		PSKIdentityHint:      []byte("Pion DTLS Client"),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
		ConnectTimeout:       dtls.ConnectTimeoutOption(30 * time.Second),
	}

	// Connect to a DTLS server
	listener, err := dtls.Listen("udp", dTlsAddr, config)
	if err != nil {
		return err
	}
	for {
		// Wait for a connection.
		conn, err := listener.Accept()
		if err != nil {
			return err;
		}
		go sctpToSocks5(conn)
	}
	return nil;
}

func sctpToSocks5(conn net.Conn){
	config := sctp.Config{
		NetConn:        conn,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	}
	a, err := sctp.Server(config)
	if err != nil {
		return ;
	}
	for{
		stream, err := a.AcceptStream()
		if err != nil {
			return ;
		}
		// set unordered = true and 10ms treshold for dropping packets
		stream.SetReliabilityParams(false, sctp.ReliabilityTypeReliable, 0)
		go proxy(stream);
	}
}
