package dot

import (
	"crypto/tls"
	"errors"
	"github.com/miekg/dns"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/comm/socks"
	"net"
	"time"
)

var dnsCache *comm.DnsCache

type DoT struct {
	Addr string;
	LSocks string;
	ServerName string;
	dnsClient *dns.Client
	dnsClientConn *dns.Conn
	connect bool;
}

func init(){
	dnsCache = &comm.DnsCache{Cache: make(map[string]string, 128)}
}


func (rd *DoT)Connect() error {
	rd.dnsClient = &dns.Client{
		Net:          "tcp",
		UDPSize:      4096,
		SingleInflight:true,
		ReadTimeout:  time.Duration(3) * time.Second,
		WriteTimeout: time.Duration(2) * time.Second,
	}
	if rd.ServerName == "" {
		return errors.New("dot: server name cannot be empty")
	}
	if rd.Addr == "" {
		return errors.New("dot: addrs cannot be empty")
	}
	cfg := &tls.Config{
		ServerName: rd.ServerName,
	}
	srcConn, err := net.DialTimeout("tcp", rd.LSocks, time.Second*15)
	if err != nil {
		return  err;
	}
	if rd.LSocks!="" {
		if socks.SocksCmd(srcConn, 1, uint8(0x01), rd.Addr,true) != nil {
			return errors.New("local socks error")
		}
	}
	srcConn.(*net.TCPConn).SetKeepAlive(true)
	srcConn.(*net.TCPConn).SetKeepAlivePeriod(3 * time.Minute)


	rd.dnsClientConn = new(dns.Conn)
	rd.dnsClientConn.Conn= tls.Client(srcConn, cfg)
	rd.dnsClientConn.UDPSize = 4094;
	rd.connect=true;
	return nil;
}



func (rd *DoT)Resolve(remoteHost string) (string,error){
	query := &dns.Msg{}
	query.SetQuestion(remoteHost+".", dns.TypeA)
	var ip="";
	var err error
	cache,_:= dnsCache.ReadDnsCache(remoteHost)
	if cache!="" {
		return  cache,nil;
	}

	for i:=0;i<2;i++{
		if !rd.connect {
			err= rd.Connect();
			if err != nil {
				continue;
			}
		}
		response, _, err := rd.dnsClient.ExchangeWithConn(query, rd.dnsClientConn)
		if err == nil {
			for _, v := range response.Answer {
				record, isType := v.(*dns.A)
				if isType {
					ip = record.A.String();
					dnsCache.WriteDnsCache(remoteHost, record.Hdr.Ttl, ip);
					return ip,nil;
				}
			}
		}else{
			rd.connect=false;
		}
	}
	return ip,err;
}

