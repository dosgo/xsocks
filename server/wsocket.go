package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/xtaci/smux"
	"golang.org/x/net/websocket"
	"github.com/hashicorp/yamux"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"qproxy/param"
	"strings"
	"time"
)



func StartWebSocket(addr string) error {
	http.HandleFunc("/",webHandler)
	if(param.KeyFile==""||param.CertFile==""){
		param.KeyFile="localhost_server.key"
		param.CertFile="localhost_server.pem"
		addrs:=strings.Split(addr,":")
		var ip="127.0.0.1";
		if(addrs[0]!="0.0.0.0"||addrs[0]!=""){
			 ip=addrs[0];
		}
		_,err:=os.Stat(param.KeyFile)
		if(err!=nil){
			genCERT("improvement","localhost",ip);
		}
	}

	err :=http.ListenAndServeTLS(addr,param.CertFile,param.KeyFile,nil)

	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
	return nil;
}


func webHandler(w http.ResponseWriter, req *http.Request){
	if(req.Header.Get("token")!=param.Password){
		msg:="Current server time:"+time.Now().Format("2006-01-02 15:04:05");
		w.Header().Add("Connection","Close")
		w.Header().Add("Content-Type","text/html")
		w.Write([]byte(msg))
		return
	}
	websocket:=websocket.Handler(webToSocks5Yamux);
	websocket.ServeHTTP(w,req);
}


/* to socks5 server*/
func webToSocks5Yamux(ws *websocket.Conn) {
	conf:=yamux.DefaultConfig();
	conf.AcceptBacklog=1024;
	conf.KeepAliveInterval=59* time.Second;
	conf.MaxStreamWindowSize=512*1024;
	// Setup server side of yamux
	session, err := yamux.Server(ws, conf)
	if err != nil {
		return;
	}
	for {
		// Accept a stream
		stream, err := session.Accept()
		if err != nil {
			return ;
		}
		go proxy(stream)
	}
}

/* to socks5 server*/
func webToSocks5Smux(ws *websocket.Conn) {
	conf:=smux.DefaultConfig();
	conf.KeepAliveInterval=59* time.Second;
	conf.KeepAliveTimeout=60 * time.Second;
	// Setup server side of yamux
	session, err := smux.Server(ws, conf)
	if err != nil {
		return;
	}
	for {
		// Accept a stream
		stream, err := session.AcceptStream()
		if err != nil {
			return ;
		}
		go proxy(stream)
	}
}


/*
func startHTTPSServer(certFile, keyFile string, server *http.Server) {
	var l net.Listener
	base := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/")
	key := [32]byte{}
	rand.Seed(time.Now().Unix())
	for i := 0; i < 32; i++ {
		key[i] = base[rand.Int()%len(base)]
	}
	keys := [][32]byte{key}
	crt, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Println(err)
		return
	}
	tr := &tls.Config{NextProtos: []string{"h2"}, Certificates: []tls.Certificate{crt}, SessionTicketKey: key}
	tr.SetSessionTicketKeys(keys)
	log.Println(string(key[:]))
	sm := http.NewServeMux()
	sm.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	})
	server.Handler = sm
	log.Println("Listen at:", server.Addr)
	l, err = tls.Listen("tcp", server.Addr, tr)
	if err != nil {
		log.Panic(err)
	}
	err = server.Serve(l)
	if err != nil {
		log.Panic(err)
	}
}*/

/*生成证书,
*/
func genCERT(organization string,host string,ip string) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			Organization: []string{organization},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		SubjectKeyId:          []byte{1, 2, 3, 4, 5},
		BasicConstraintsValid: true,
		IsCA:        true,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
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
	ioutil.WriteFile(ca_f, ca_b64, 0777)

	priv_f := name + ".key"
	priv_b := x509.MarshalPKCS1PrivateKey(priv)
	ioutil.WriteFile(priv_f, priv_b, 0777)
	var privateKey = &pem.Block{Type: "PRIVATE KEY",
		Headers: map[string]string{},
		Bytes:   priv_b}
	priv_b64 := pem.EncodeToMemory(privateKey)
	ioutil.WriteFile(priv_f, priv_b64, 0777)
}



