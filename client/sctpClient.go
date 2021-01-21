package client

import (
	"github.com/pion/dtls"
	"github.com/pion/logging"
	"github.com/pion/sctp"
	"net"
	"sync"
	"time"
	"xSocks/comm"
	"xSocks/param"
)
var sctpDialer *SctpInfo
func init(){
	sctpDialer =&SctpInfo{streamIdentifier:0}
}


type SctpInfo struct {
	dtlsConn           *dtls.Conn
	sctpClient    *sctp.Association
	streamIdentifier uint16
	sync.Mutex
}

func NewSctpDialer() *SctpInfo {
	return sctpDialer
}


func (qd *SctpInfo) Dial(url string) (comm.CommConn, error) {
	qd.Lock()
	defer qd.Unlock()
	var err error;
	if(qd.dtlsConn==nil) {
		dtlsConn,sctpClient,err:= initSctp(url)
		if err != nil {
			return nil, err;
		}
		qd.dtlsConn=dtlsConn;
		qd.sctpClient=sctpClient;
	}
	//归零
	if(qd.streamIdentifier>65534){
		qd.streamIdentifier=0;
	}
	stream, err := qd.sctpClient.OpenStream(qd.streamIdentifier,sctp.PayloadTypeWebRTCBinary)
	qd.streamIdentifier++;//累加
	if err != nil {
		if(qd.dtlsConn!=nil) {
			qd.dtlsConn.Close();
		}
		if(qd.sctpClient!=nil) {
			qd.sctpClient.Close();
		}
		dtlsConn,sctpClient,err:= initSctp(url)
		if err != nil {
			return nil, err;
		}
		qd.dtlsConn=dtlsConn;
		qd.sctpClient=sctpClient;
		stream, err = qd.sctpClient.OpenStream(qd.streamIdentifier,sctp.PayloadTypeWebRTCBinary)
		qd.streamIdentifier++;//累加
		if err != nil {
			if(qd.dtlsConn!=nil) {
				qd.dtlsConn.Close()
			}
			if(qd.sctpClient!=nil) {
				qd.sctpClient.Close()
			}
			qd.dtlsConn=nil;
			qd.sctpClient=nil;
			return nil,err;
		}
		return nil, err
	}
	stream.SetReliabilityParams(false, sctp.ReliabilityTypeReliable, 0)
	return stream, nil
}

func initSctp(url string) (*dtls.Conn, *sctp.Association ,error){
	dtlsConn,err:= connectDtls(url)
	if err != nil {
		return nil,nil, err;
	}
	config := sctp.Config{
		NetConn:   dtlsConn,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	}
	sctpClient, err := sctp.Client(config)
	if err != nil {
		return nil,nil,err;
	}
	return dtlsConn,sctpClient,nil;
}

func connectDtls(url string)(*dtls.Conn, error){
	dTlsAddr,err :=net.ResolveUDPAddr("udp",url);
	if err != nil {
		return nil,err;
	}
	// Prepare the configuration of the DTLS connection
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			return []byte(param.Password), nil
			//return []byte{0xAB, 0xC1, 0x23}, nil
		},
		PSKIdentityHint:    []byte("Pion DTLS Server"),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
		ConnectTimeout:       dtls.ConnectTimeoutOption(30 * time.Second),
	}
	// Connect to a DTLS server
	return dtls.Dial("udp", dTlsAddr, config)
}


