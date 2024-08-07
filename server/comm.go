package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"sync"
	"time"

	socksTapComm "github.com/dosgo/goSocksTap/comm"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
	"github.com/hashicorp/yamux"
)

/*共享内存避免GC*/
var poolAuthHeadBuf = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 16)
	},
}

func Proxy(conn socksTapComm.CommConn) {
	defer conn.Close()
	//read auth Head
	var authHead = poolAuthHeadBuf.Get().([]byte)
	defer poolAuthHeadBuf.Put(authHead)
	_, err := io.ReadFull(conn, authHead[:16])
	if err != nil {
		log.Printf("Proxy err:%v\r\n", err)
		return
	}
	//autherr;
	if string(authHead) != comm.GenPasswordHead(param.Args.Password) {
		log.Printf("password err\r\n")
		return
	}
	//read cmd
	cmd := make([]byte, 1)
	_, err = io.ReadFull(conn, cmd)
	if err != nil {
		log.Printf("Proxy 1 err:%v\r\n", err)
		return
	}
	switch cmd[0] {
	//dns
	case 0x01:
		dnsResolve(conn)
		break
	//to tcp req
	case 0x02:
		var addrLen uint16
		if err := binary.Read(conn, binary.BigEndian, &addrLen); err != nil {
			log.Printf("Proxy 2 addrLen:%d err:%v\r\n", addrLen, err)
			return
		}
		addr := make([]byte, addrLen)
		if _, err := io.ReadFull(conn, addr); err != nil {
			log.Printf("Proxy 3 addr:%s err:%v\r\n", string(addr), err)
			return
		}
		sConn, err := net.DialTimeout("tcp", string(addr), 5*time.Second)
		if err != nil {
			log.Printf("Proxy 4 addr:%s err:%v\r\n", string(addr), err)
			return
		}
		defer sConn.Close()
		socksTapComm.ConnPipe(sConn, conn, time.Minute*2)
		break
	//to tun
	case 0x03:
		//tcpToTun(conn)
		break
		//to udp proxy
	case 0x04:
		var addrLen uint16
		if err := binary.Read(conn, binary.BigEndian, &addrLen); err != nil {
			log.Printf("Proxy 5 addrLen:%d err:%v\r\n", addrLen, err)
			return
		}
		addr := make([]byte, addrLen)
		if err := binary.Read(conn, binary.BigEndian, addr); err != nil {
			log.Printf("Proxy 6 addr:%s err:%v\r\n", string(addr), err)
			return
		}
		sConn, err := net.DialTimeout("udp", string(addr), 5*time.Second)
		if err != nil {
			log.Printf("Proxy 7 addr:%s err:%v\r\n", string(addr), err)
			return
		}
		defer sConn.Close()
		socksTapComm.ConnPipe(sConn, conn, time.Minute*2)
		break
	}
}

/*dns解析*/
func dnsResolve(conn socksTapComm.CommConn) {
	hostLenBuf := make([]byte, 1)
	var hostBuf = make([]byte, 1024)
	var hostLen int
	var err error
	for {
		_, err = io.ReadFull(conn, hostLenBuf)
		if err != nil {
			return
		}
		hostLen = int(hostLenBuf[0])
		_, err = io.ReadFull(conn, hostBuf[:hostLen])
		if err != nil {
			log.Printf("hostLen:%d\r\n", hostLen)
			return
		}
		addr, err := net.ResolveIPAddr("ip4", string(hostBuf[:hostLen]))
		if err != nil {
			log.Printf("host:%s hostLen:%d\r\n", string(hostBuf[:hostLen]), hostLen)
			//err
			conn.Write([]byte{0x01, 0x04}) //0x01==error  0x04==ipv4
			continue                       //解析失败跳过不关闭连接
		}
		_, err = conn.Write([]byte{0x00, 0x04}) //响应客户端
		_, err = conn.Write(addr.IP.To4())      //响应客户端
		if err != nil {
			return
		}
	}
}
func GetPublicIP() (ip string, err error) {
	var (
		addrs   []net.Addr
		addr    net.Addr
		ipNet   *net.IPNet // IP地址
		isIpNet bool
	)
	// 获取所有网卡
	if addrs, err = net.InterfaceAddrs(); err != nil {
		return
	}
	//取IP
	for _, addr = range addrs {
		// 这个网络地址是IP地址: ipv4, ipv6
		if ipNet, isIpNet = addr.(*net.IPNet); isIpNet && !ipNet.IP.IsLoopback() {
			//
			if ipNet.IP.To4() != nil {
				if socksTapComm.IsPublicIP(ipNet.IP) {
					ip = ipNet.IP.String()
					return
				}
			}
		}
	}
	err = errors.New("no public ip")
	return
}

/* to socks5 server*/
func streamToSocks5Yamux(conn io.ReadWriteCloser) {
	conf := yamux.DefaultConfig()
	conf.AcceptBacklog = 1024
	conf.KeepAliveInterval = 52 * time.Second
	conf.MaxStreamWindowSize = 1024 * 1024
	conf.ConnectionWriteTimeout = 50 * time.Second
	// Setup server side of yamux
	session, err := yamux.Server(conn, conf)
	if err != nil {
		log.Printf("err:%v\r\n", err)
		return
	}
	defer session.Close()
	for {
		// Accept a stream
		stream, err := session.Accept()
		if err != nil {
			log.Printf("streamToSocks5Yamux err:%v\r\n", err)
			return
		}
		go Proxy(stream)
	}
}

/*生成证书,
 */
func genCERT(organization string, host string, ip string) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			Organization: []string{organization},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		SubjectKeyId:          []byte{1, 2, 3, 4, 5},
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	privCa, _ := rsa.GenerateKey(rand.Reader, 1024)
	CreateCertificateFile(host+"_ca", ca, privCa, ca, nil)
	server := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization: []string{organization},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	hosts := []string{host, ip}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			server.IPAddresses = append(server.IPAddresses, ip)
		} else {
			server.DNSNames = append(server.DNSNames, h)
		}
	}
	privSer, _ := rsa.GenerateKey(rand.Reader, 1024)
	CreateCertificateFile(host+"_server", server, privSer, ca, privCa)
}

func CreateCertificateFile(name string, cert *x509.Certificate, key *rsa.PrivateKey, caCert *x509.Certificate, caKey *rsa.PrivateKey) {
	priv := key
	pub := &priv.PublicKey
	privPm := priv
	if caKey != nil {
		privPm = caKey
	}
	ca_b, err := x509.CreateCertificate(rand.Reader, cert, caCert, pub, privPm)
	if err != nil {
		log.Println("create failed", err)
		return
	}
	ca_f := name + ".pem"
	var certificate = &pem.Block{Type: "CERTIFICATE",
		Headers: map[string]string{},
		Bytes:   ca_b}
	ca_b64 := pem.EncodeToMemory(certificate)
	os.WriteFile(ca_f, ca_b64, 0777)

	priv_f := name + ".key"
	priv_b := x509.MarshalPKCS1PrivateKey(priv)
	os.WriteFile(priv_f, priv_b, 0777)
	var privateKey = &pem.Block{Type: "PRIVATE KEY",
		Headers: map[string]string{},
		Bytes:   priv_b}
	priv_b64 := pem.EncodeToMemory(privateKey)
	os.WriteFile(priv_f, priv_b64, 0777)
}
