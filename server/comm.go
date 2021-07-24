package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/hashicorp/yamux"
	"github.com/xtaci/smux"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"sync"
	"time"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
)



/*共享内存避免GC*/
var poolAuthHeadBuf = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 16)
	},
}
/*save uniqueId Tun */
var uniqueIdTun sync.Map

func Proxy(conn comm.CommConn){
	defer conn.Close()
	//read auth Head
	var authHead = poolAuthHeadBuf.Get().([]byte)
	defer poolAuthHeadBuf.Put(authHead)
	_, err := io.ReadFull(conn, authHead[:16])
	if err != nil {
		log.Printf("err:%v\r\n",err);
		return
	}
	//autherr;
	if string(authHead)!= comm.GenPasswordHead(param.Args.Password) {
		fmt.Printf("password err\r\n");
		return ;
	}
	//read cmd
	cmd := make([]byte,1)
	_, err = io.ReadFull(conn, cmd)
	if err != nil {
		fmt.Printf("err:%v\r\n",err)
		return
	}
	switch cmd[0] {
		//dns
		case 0x01:
			dnsResolve(conn);
			break
		//to socks5
		case 0x02:
			//连接socks5
			sConn, err := net.DialTimeout("tcp", "127.0.0.1:"+param.Args.Sock5Port,param.Args.ConnectTime)
			if err!=nil {
				log.Printf("err:%v\r\n",param.Args.Sock5Port)
				return ;
			}
			defer sConn.Close();
			//交换数据
			comm.TcpPipe(sConn,conn,time.Minute*10)
			break;
		//to tun
		case 0x03:
			tcpToTun(conn)
			break;
			//to udp socket
		case 0x04:
			tcpToUdpProxy(conn);
			break;
	}
}

/*转发到本地的udp网关*/
func tcpToUdpProxy(conn comm.CommConn){
	var packLenByte []byte = make([]byte, 2)
	var bufByte []byte = make([]byte,1024*10)
	remoteConn, err := net.DialTimeout("udp", "127.0.0.1:"+param.Args.UdpGatePort,time.Second*15);
	if err!=nil {
		log.Printf("err:%v\r\n",err);
		return
	}
	defer remoteConn.Close()

	go func() {
		var bufByte1 []byte = make([]byte,1024*10)
		var buffer bytes.Buffer
		var packLenByte []byte = make([]byte, 2)
		for {
			remoteConn.SetDeadline(time.Now().Add(5*time.Minute))
			n, err := remoteConn.Read(bufByte1)
			if err != nil {
				log.Printf("err:%v\r\n",err);
				break;
			}
			buffer.Reset()
			binary.LittleEndian.PutUint16(packLenByte, uint16(n))
			buffer.Write(packLenByte)
			buffer.Write(bufByte1[:n])
			//remote to client
			conn.Write(buffer.Bytes())
		}
	}();

	for {
		//remoteConn.SetDeadline();
		conn.SetDeadline(time.Now().Add(5*time.Minute))
		_, err := io.ReadFull(conn, packLenByte)
		packLen := binary.LittleEndian.Uint16(packLenByte)
		if err != nil||int(packLen)>len(bufByte) {
			log.Printf("err:%v\r\n",err);
			break;
		}
		conn.SetDeadline(time.Now().Add(5*time.Minute))
		_, err = io.ReadFull(conn, bufByte[:int(packLen)])
		if err != nil {
			log.Printf("err:%v\r\n",err);
			break;
		}else {
			_, err = remoteConn.Write(bufByte[:int(packLen)])
			if err != nil {
				log.Printf("err:%v\r\n",err);
			}
		}
	}
}

/*to tun 处理*/
func tcpToTun(conn comm.CommConn){
	uniqueIdByte := make([]byte,8)
	_, err := io.ReadFull(conn, uniqueIdByte)
	if err!=nil {
		log.Printf("err:%v\r\n",err)
		return ;
	}
	uniqueId:=string(uniqueIdByte)
	fmt.Printf("uniqueId:%s\r\n",uniqueId)
	var mtuByte []byte = make([]byte, 2)
	//read Mtu
	_, err = io.ReadFull(conn, mtuByte)
	if err!=nil {
		log.Printf("err:%v\r\n")
		return ;
	}
	mtu := binary.LittleEndian.Uint16(mtuByte)
	if mtu<1 {
		mtu=1024;
	}
	_stack,channelLinkID,err:=StartTunStack(mtu);
	if err!=nil {
		return;
	}
	defer _stack.Close();
	var buffer =new(bytes.Buffer)
	defer fmt.Printf("channelLinkID recv exit \r\n");
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		var sendBuffer =new(bytes.Buffer)
		var packLenByte []byte = make([]byte, 2)
		for {
			pkt,res :=channelLinkID.ReadContext(ctx)
			if !res {
				break;
			}
			buffer.Reset()
			buffer.Write(pkt.Pkt.NetworkHeader().View())
			buffer.Write(pkt.Pkt.TransportHeader().View())
			buffer.Write(pkt.Pkt.Data.ToView())
			if(buffer.Len()>0) {
				binary.LittleEndian.PutUint16(packLenByte,uint16(buffer.Len()))
				sendBuffer.Reset()
				sendBuffer.Write(packLenByte)
				sendBuffer.Write(buffer.Bytes())
				_,err=conn.Write(sendBuffer.Bytes())
				if err!=nil {
					return ;
				}
			}
		}

	}()
	var buflen=mtu+80;
	var buf=make([]byte,buflen)
	var packLenByte []byte = make([]byte, 2)
	for {
		conn.SetDeadline(time.Now().Add(time.Minute*5))
		_, err := io.ReadFull(conn, packLenByte)
		if err != nil {
			log.Printf("err:%v\r\n", err)
			return;
		}
		packLen := binary.LittleEndian.Uint16(packLenByte)
		//null
		if packLen < 1 || packLen > buflen {
			continue;
		}
		conn.SetDeadline(time.Now().Add(time.Minute*5))
		n, err := io.ReadFull(conn, buf[:int(packLen)])
		if err != nil {
			log.Printf("err:%v\r\n", err)
			return;
		}
		InjectInbound(channelLinkID,buf[:n])
	}
}



/*dns解析*/
func dnsResolve(conn comm.CommConn) {
	hostLenBuf := make([]byte,1)
	var hostBuf =  make([]byte,1024)
	var hostLen int;
	var err error
	for{
		_, err = io.ReadFull(conn, hostLenBuf)
		if err != nil {
			return
		}
		hostLen=int(hostLenBuf[0])
		_, err = io.ReadFull(conn, hostBuf[:hostLen])
		if err != nil {
			fmt.Printf("hostLen:%d\r\n",hostLen)
			return
		}
		addr, err := net.ResolveIPAddr("ip4", string(hostBuf[:hostLen]))
		if(err!=nil){
			fmt.Printf("host:%s hostLen:%d\r\n",string(hostBuf[:hostLen]),hostLen)
			//err
			conn.Write([]byte{0x01, 0x04}) //0x01==error  0x04==ipv4
			continue;//解析失败跳过不关闭连接
		}
		_, err =conn.Write([]byte{0x00, 0x04}) //响应客户端
		_, err =conn.Write(addr.IP.To4()) //响应客户端
		if(err!=nil){
			return ;
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
			if(ipNet.IP.To4() != nil){
				if(comm.IsPublicIP(ipNet.IP)){
					ip = ipNet.IP.String()
					return ;
				}
			}
		}
	}
	err = errors.New("no public ip")
	return
}

/* to socks5 server*/
func streamToSocks5Yamux(conn io.ReadWriteCloser) {
	conf:=yamux.DefaultConfig();
	conf.AcceptBacklog=1024;
	conf.KeepAliveInterval=52* time.Second;
	conf.MaxStreamWindowSize=1024*1024;
	conf.ConnectionWriteTimeout=50* time.Second;
	// Setup server side of yamux
	session, err := yamux.Server(conn, conf)
	if err != nil {
		log.Printf("err:%v\r\n",err);
		return;
	}
	defer  session.Close();
	for {
		// Accept a stream
		stream, err := session.Accept()
		if err != nil {
			log.Printf("err:%v\r\n",err);
			return ;
		}
		go Proxy(stream)
	}
}

/* to socks5 server*/
func streamToSocks5Smux(conn io.ReadWriteCloser) {
	conf:=smux.DefaultConfig();
	conf.KeepAliveInterval=59* time.Second;
	conf.KeepAliveTimeout=60 * time.Second;
	// Setup server side of yamux
	session, err := smux.Server(conn, conf)
	if err != nil {
		log.Printf("err:%v\r\n",err);
		return;
	}
	defer  session.Close();
	for {
		// Accept a stream
		stream, err := session.AcceptStream()
		if err != nil {
			log.Printf("err:%v\r\n",err);
			return ;
		}
		go Proxy(stream)
	}
}




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




