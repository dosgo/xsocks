package client

import (
	"log"
	"net"
	"time"

	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/comm/dot"
	"github.com/dosgo/xsocks/param"
	"github.com/miekg/dns"
)

type LocalDns struct {
	safeDns   SafeDns
	dnsClient *dns.Client
	udpServer *dns.Server
	tcpServer *dns.Server
}

/*remote to loacal*/
func (localDns *LocalDns) StartDns() {
	localDns.udpServer = &dns.Server{
		Net:          "udp",
		Addr:         ":" + param.Args.DnsPort,
		Handler:      dns.HandlerFunc(localDns.ServeDNS),
		UDPSize:      4096,
		ReadTimeout:  time.Duration(10) * time.Second,
		WriteTimeout: time.Duration(10) * time.Second,
	}
	localDns.tcpServer = &dns.Server{
		Net:          "tcp",
		Addr:         ":" + param.Args.DnsPort,
		Handler:      dns.HandlerFunc(localDns.ServeDNS),
		UDPSize:      4096,
		ReadTimeout:  time.Duration(10) * time.Second,
		WriteTimeout: time.Duration(10) * time.Second,
	}

	localDns.safeDns = dot.NewDot("dns.google", "8.8.8.8:853", param.Args.Sock5Addr)
	localDns.dnsClient = &dns.Client{
		Net:            "udp",
		UDPSize:        4096,
		ReadTimeout:    time.Duration(3) * time.Second,
		WriteTimeout:   time.Duration(2) * time.Second,
		SingleInflight: false,
	}
	go localDns.udpServer.ListenAndServe()
	go localDns.tcpServer.ListenAndServe()
}

func (localDns *LocalDns) Shutdown() {
	if localDns.tcpServer != nil {
		localDns.tcpServer.Shutdown()
	}
	if localDns.udpServer != nil {
		localDns.udpServer.Shutdown()
	}
}

func (localDns *LocalDns) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	var msg *dns.Msg
	var err error
	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg, err = localDns.doIPv4Query(r)
		break
	case dns.TypeAAAA:
		//ipv6
		msg, _, err = localDns.dnsClient.Exchange(r, "114.114.114.114:53")
		break
	default:
		msg, _, err = localDns.dnsClient.Exchange(r, "114.114.114.114:53")
		break
	}
	if err != nil {
		dns.HandleFailed(w, r)
	} else {
		w.WriteMsg(msg)
	}
}

func (localDns *LocalDns) doIPv4Query(r *dns.Msg) (*dns.Msg, error) {
	m := &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = false
	domain := r.Question[0].Name
	var ip string
	var err error
	if param.Args.LocalDns == 1 {
		m1, _, err := localDns.dnsClient.Exchange(r, "114.114.114.114:53")
		if err == nil {
			for _, v := range m1.Answer {
				record, isType := v.(*dns.A)
				if isType {
					//中国Ip直接回复
					if comm.IsChinaMainlandIP(record.A.String()) {
						return m1, nil
					}
				}
			}
		}
	}
	ip, err = localDns.safeDns.Resolve(domain[0:len(domain)-1], 4)
	if err != nil {
		log.Printf("dns domain:%s Resolve err:%v\r\n", domain, err)
		return m, err
	}
	log.Printf("dns domain:%s Resolve ip:%v\r\n", domain, ip)
	m.Answer = append(r.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 1},
		A:   net.ParseIP(ip),
	})
	// final
	return m, nil
}
