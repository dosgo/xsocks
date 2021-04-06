package client

import (
	"fmt"
	"github.com/emicklei/go-restful/log"
	"github.com/miekg/dns"
	"net"
	"time"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/param"
)

type LocalDns struct {
	remoteDns RemoteDns
	dnsClient *dns.Client
}
var localdns=LocalDns{}


/*remote to loacal*/
func StartDns() error {
	udpServer := &dns.Server{
		Net:          "udp",
		Addr:         ":"+param.Args.DnsPort,
		Handler:      dns.HandlerFunc(localdns.ServeDNS),
		UDPSize:      4096,
		ReadTimeout:  time.Duration(10) * time.Second,
		WriteTimeout: time.Duration(10) * time.Second,
	}
	tcpServer:= &dns.Server{
		Net:          "tcp",
		Addr:         ":"+param.Args.DnsPort,
		Handler:      dns.HandlerFunc(localdns.ServeDNS),
		UDPSize:      4096,
		ReadTimeout:  time.Duration(10) * time.Second,
		WriteTimeout: time.Duration(10) * time.Second,
	}

	localdns.remoteDns = RemoteDns{}
	localdns.dnsClient = &dns.Client{
		Net:          "udp",
		UDPSize:      4096,
		ReadTimeout:  time.Duration(1) * time.Second,
		WriteTimeout: time.Duration(1) * time.Second,
	}
	go udpServer.ListenAndServe();
	tcpServer.ListenAndServe();
	return nil;
}




func  (localdns *LocalDns)ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	var msg *dns.Msg
	var err error
	switch r.Question[0].Qtype {
		case  dns.TypeA:
			msg, err = localdns.doIPv4Query(r)
		break;
		case  dns.TypeAAAA:
			//ipv6
			msg, err = localdns.resolve(r)
		break;
	default:
		msg,_,err = localdns.dnsClient.Exchange(r,"114.114.114.114:53")
		break;
	}
	if err != nil {
		dns.HandleFailed(w, r)
	} else {
		w.WriteMsg(msg)
	}
}

func (localdns *LocalDns) doIPv4Query(r *dns.Msg) (*dns.Msg, error) {
	m := &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = false
	domain := r.Question[0].Name
	var ip string;
	var err error;
	if param.Args.LocalDns==1 {
		m1,_,err := localdns.dnsClient.Exchange(r,"114.114.114.114:53")
		if err == nil {
			for _, v := range m1.Answer {
				record, isType := v.(*dns.A)
				if isType {
					//中国Ip直接回复
					if comm.IsChinaMainlandIP(record.A.String()) {
						return m1,nil;
					}
				}
			}
		}
	}
	ip, err = localdns.remoteDns.Resolve(domain[0 : len(domain)-1])
	if err!=nil {
		fmt.Printf("dns domain:%s Resolve err:%v\r\n",domain,err)
		return m, err;
	}
	fmt.Printf("dns domain:%s Resolve ip:%v\r\n",domain,ip)
	m.Answer = append(r.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   net.ParseIP(ip),
	})
	// final
	return m, nil
}



func  (localdns *LocalDns) resolve(r *dns.Msg) (*dns.Msg, error) {
	m :=  &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = false
	domain := r.Question[0].Name
	fmt.Printf("dns ipv6 :%s Qtype:%d\r\n",domain,r.Question[0].Qtype)

	m1,_,err := localdns.dnsClient.Exchange(r,"114.114.114.114:53")
	if err == nil {
		for _, v := range m1.Answer {
			_, isType := v.(*dns.AAAA)
			if isType {
				log.Printf("ipv6dns ok\r\n");
				return m1,nil;
			}
		}
	}
	/*
	m.Answer = append(r.Answer, &dns.AAAA{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
		AAAA:   net.ParseIP("fd3e:4f5a:5b81::1"),
	})*/
	return m, nil
}
