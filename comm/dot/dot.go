package dot

import (
	"crypto/tls"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/proxy"
	"golang.org/x/sync/singleflight"
)

type DoT struct {
	dnsCache      *DnsCacheV1
	Addr          string
	LSocks        string
	ServerName    string
	dnsClient     *dns.Client
	dnsClientConn *dns.Conn
	Singleflight  *singleflight.Group
	connect       bool
}

func NewDot(serverName string, addr string, lSocks string) *DoT {
	dot := &DoT{ServerName: serverName, Addr: addr, LSocks: lSocks, Singleflight: &singleflight.Group{}}
	dot.dnsCache = &DnsCacheV1{Cache: make(map[string]CachedResponse, 128)}
	return dot
}

func (rd *DoT) Connect() error {
	rd.dnsClient = &dns.Client{
		Net:            "tcp",
		UDPSize:        4096,
		SingleInflight: true,
		ReadTimeout:    time.Duration(10) * time.Second,
		WriteTimeout:   time.Duration(10) * time.Second,
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
	dialer, err := proxy.SOCKS5("tcp", rd.LSocks, nil, proxy.Direct)
	if err != nil {
		log.Printf("SOCKS5 拨号失败: %v", err)
		return err
	}
	// ... err check
	srcConn, err := dialer.Dial("tcp", rd.Addr)
	rd.dnsClientConn = new(dns.Conn)
	rd.dnsClientConn.Conn = tls.Client(srcConn, cfg)
	rd.dnsClientConn.UDPSize = 4096
	rd.connect = true
	return nil
}

func (rd *DoT) Resolve(remoteHost string, ipType int) (string, error) {
	query := &dns.Msg{}
	if ipType == 4 {
		query.SetQuestion(remoteHost+".", dns.TypeA)
	}
	if ipType == 6 {
		query.SetQuestion(remoteHost+".", dns.TypeAAAA)
	}
	var ip = ""
	var err error
	cacheRes := rd.dnsCache.ReadDnsCache(remoteHost+":"+strconv.Itoa(ipType), 120)
	if cacheRes != nil {
		ip, err = rd.getIP(cacheRes, ipType)
		if err == nil {
			return ip, err
		}
	}

	for i := 0; i < 2; i++ {
		if !rd.connect {
			_, err, _ = rd.Singleflight.Do("connect", func() (interface{}, error) {
				return nil, rd.Connect()
			})
			if err != nil {
				continue
			}
		}
		response, _, err := rd.dnsClient.ExchangeWithConn(query, rd.dnsClientConn)
		if err == nil {
			rd.dnsCache.WriteDnsCache(remoteHost+":"+strconv.Itoa(ipType), response)
			ip, err = rd.getIP(response, ipType)
			if err == nil {
				return ip, err
			}
		} else {
			rd.connect = false
		}
	}
	return ip, err
}

func (rd *DoT) getIP(response *dns.Msg, ipType int) (string, error) {
	for _, v := range response.Answer {
		if ipType == 4 {
			record, isType := v.(*dns.A)
			if isType {
				ip := record.A.String()
				return ip, nil
			}
		}
		if ipType == 6 {
			record, isType := v.(*dns.AAAA)
			if isType {
				ip := record.AAAA.String()
				return ip, nil
			}
		}
	}
	return "", errors.New("not ")
}

func (rd *DoT) AutoFree() {
	rd.dnsCache.Free(120)
}
