package dot

import (
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/comm/socks"
	"github.com/miekg/dns"
	"golang.org/x/sync/singleflight"
)

var dnsCache *comm.DnsCache

type DoT struct {
	Addr          string
	LSocks        string
	ServerName    string
	dnsClient     *dns.Client
	dnsClientConn *dns.Conn
	Singleflight  *singleflight.Group
	connect       bool
}

func init() {
	dnsCache = &comm.DnsCache{Cache: make(map[string]string, 128)}
}

func NewDot(serverName string, addr string, lSocks string) *DoT {
	return &DoT{ServerName: serverName, Addr: addr, LSocks: lSocks, Singleflight: &singleflight.Group{}}
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
	srcConn, err := net.DialTimeout("tcp", rd.LSocks, time.Second*15)
	if err != nil {
		return err
	}
	if rd.LSocks != "" {
		if socks.SocksCmd(srcConn, 1, uint8(0x01), rd.Addr, true) != nil {
			return errors.New("local socks error")
		}
	}
	srcConn.(*net.TCPConn).SetKeepAlive(true)
	srcConn.(*net.TCPConn).SetKeepAlivePeriod(3 * time.Minute)

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
	cache, _ := dnsCache.ReadDnsCache(remoteHost + ":" + strconv.Itoa(ipType))
	if cache != "" {
		return cache, nil
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
			for _, v := range response.Answer {
				if ipType == 4 {
					record, isType := v.(*dns.A)
					if isType {
						ip = record.A.String()
						dnsCache.WriteDnsCache(remoteHost+":"+strconv.Itoa(ipType), record.Hdr.Ttl, ip)
						return ip, nil
					}
				}
				if ipType == 6 {
					record, isType := v.(*dns.AAAA)
					if isType {
						ip = record.AAAA.String()
						dnsCache.WriteDnsCache(remoteHost+":"+strconv.Itoa(ipType), record.Hdr.Ttl, ip)
						return ip, nil
					}
				}
			}
		} else {
			rd.connect = false
		}
	}
	return ip, err
}
