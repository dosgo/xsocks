package client

import (
	"errors"
	"io"
	"log"
	"net"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dosgo/go-tun2socks/core"
	"github.com/dosgo/go-tun2socks/tun"
	"github.com/dosgo/go-tun2socks/tun2socks"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/comm/dot"
	"github.com/dosgo/xsocks/comm/netstat"
	"github.com/dosgo/xsocks/comm/socks"
	"github.com/dosgo/xsocks/comm/winDivert"
	"github.com/dosgo/xsocks/param"
	"github.com/miekg/dns"
	"github.com/vishalkuo/bimap"
	"golang.org/x/time/rate"
)

type FakeDnsTun struct {
	tunType        int // 3 or 5
	localSocks     string
	udpLimit       sync.Map
	socksServerPid int
	autoFilter     bool
	udpProxy       bool
	run            bool
	tunDns         *TunDns
	tunDev         io.ReadWriteCloser
	safeDns        SafeDns
}

type TunDns struct {
	dnsClient      *dns.Client
	dnsClientConn  *dns.Conn
	udpServer      *dns.Server
	tcpServer      *dns.Server
	run            bool
	excludeDomains map[string]uint8
	dnsAddr        string
	dnsAddrV6      string
	dnsPort        string
	ip2Domain      *bimap.BiMap
	fakeDnsCache   *comm.DnsCache
}

var tunAddr = "10.0.0.2"
var tunGW = "10.0.0.1"
var tunMask = "255.255.0.0"
var fakeUdpNat sync.Map
var ipv6To4 sync.Map

func (fakeDns *FakeDnsTun) Start(tunType int, udpProxy bool, tunDevice string, _tunAddr string, _tunMask string, _tunGW string, tunDNS string) {
	fakeDns.tunType = tunType
	fakeDns.udpProxy = udpProxy
	//start local dns  (Compatible with openvpn using port 653)
	fakeDns.tunDns = &TunDns{dnsPort: "653", dnsAddr: "127.0.0.1", dnsAddrV6: "0:0:0:0:0:0:0:1"}
	if fakeDns.tunType == 3 {
		fakeDns.localSocks = param.Args.Sock5Addr
	}
	if fakeDns.tunType == 5 {
		fakeDns.localSocks = param.Args.ServerAddr[9:]
		if runtime.GOOS == "windows" {
			fakeDns.autoFilter = true
		}
	}
	if fakeDns.tunType == 5 || fakeDns.tunType == 3 {
		fakeDns.safeDns = dot.NewDot("dns.google", "8.8.8.8:853", fakeDns.localSocks)
	}

	fakeDns.tunDns.ip2Domain = bimap.NewBiMap()
	fakeDns.tunDns.fakeDnsCache = &comm.DnsCache{Cache: make(map[string]string, 128)}
	fakeDns.tunDns.excludeDomains = make(map[string]uint8)
	if fakeDns.tunType == 3 {
		urlInfo, _ := url.Parse(param.Args.ServerAddr)
		fakeDns.tunDns.excludeDomains[urlInfo.Hostname()+"."] = 1
	}
	//生成本地udp端口避免过滤的时候变动了
	clientPort, _ := comm.GetFreeUdpPort()
	fakeDns.tunDns._startSmartDns(clientPort)
	fakeDns._startTun(tunDevice, _tunAddr, _tunMask, _tunGW, tunDNS)

	//edit DNS
	if runtime.GOOS == "windows" {
		go winDivert.RedirectDNS(fakeDns.tunDns.dnsAddr, fakeDns.tunDns.dnsPort, clientPort)
	} else {
		comm.SetNetConf(fakeDns.tunDns.dnsAddr, fakeDns.tunDns.dnsAddrV6)
	}
	//udp limit auto remove
	fakeDns.run = true
	go fakeDns.task()
}

func (fakeDns *FakeDnsTun) Shutdown() {
	if fakeDns.tunDev != nil {
		fakeDns.tunDev.Close()
	}
	if fakeDns.tunDns != nil {
		comm.ResetNetConf(fakeDns.tunDns.dnsAddr)
		fakeDns.tunDns.Shutdown()
	}
	fakeDns.run = false
	winDivert.CloseWinDivert()
}

func (fakeDns *FakeDnsTun) _startTun(tunDevice string, _tunAddr string, _tunMask string, _tunGW string, tunDNS string) error {
	if len(_tunAddr) > 0 {
		tunAddr = _tunAddr
	}
	if len(_tunMask) > 0 {
		tunMask = _tunMask
	}
	if len(_tunGW) > 0 {
		tunGW = _tunGW
	}
	var err error
	//dnsServers := strings.Split(tunDNS, ",")
	if len(param.Args.UnixSockTun) > 0 {
		fakeDns.tunDev, err = SocketToTun(param.Args.UnixSockTun)
		if err != nil {
			return err
		}
	} else {
		log.Printf("tunGW:%s tunMask:%s\r\n", tunGW, tunMask)
		fakeDns.tunDev, err = tun.RegTunDev(tunDevice, tunAddr, tunMask, tunGW, tunDNS)
		if err != nil {
			fakeDns.tunDev = nil
			return err
		}
	}
	go func() {
		time.Sleep(time.Second * 1)
		comm.AddRoute(tunAddr, tunGW, tunMask)
	}()
	go tun2socks.ForwardTransportFromIo(fakeDns.tunDev, param.Args.Mtu, fakeDns.tcpForwarder, fakeDns.udpForwarder)
	return nil
}
func (fakeDns *FakeDnsTun) task() {
	for fakeDns.run {
		fakeDns.udpLimit.Range(func(k, v interface{}) bool {
			_v := v.(*comm.UdpLimit)
			if _v.Expired < time.Now().Unix() {
				fakeDns.udpLimit.Delete(k)
			}
			return true
		})
		pid, err := netstat.PortGetPid(fakeDns.localSocks)
		if err == nil && pid > 0 {
			fakeDns.socksServerPid = pid
		}
		time.Sleep(time.Second * 30)
	}
}

func (fakeDns *FakeDnsTun) tcpForwarder(conn core.CommTCPConn) error {
	var srcAddr = conn.LocalAddr().String()
	var srcAddrs = strings.Split(srcAddr, ":")
	var remoteAddr = ""
	var addrType = 0x01
	defer conn.Close()
	if fakeDns.autoFilter && netstat.IsSocksServerAddr(fakeDns.socksServerPid, srcAddrs[0]) {
		domain := fakeDns.dnsToDomain(srcAddr)
		domains := strings.Split(domain, ":")
		if domain == "" {
			log.Printf("dnsToDomain domain:%s srcAddr:%s\r\n", domain, srcAddr)
			return nil
		}
		//add exclude  domain
		fakeDns.tunDns.excludeDomains[domains[0]] = 1
		ip, _, err := fakeDns.tunDns.localResolve(domains[0], 4)
		if err != nil {
			log.Printf("domain:%s  srcAddr:%s localResolve err:%s\r\n", domains[0], srcAddr, err)
			return nil
		}
		socksConn, err := net.DialTimeout("tcp", ip.String()+":"+srcAddrs[1], time.Second*15)
		if err != nil {
			return nil
		}
		defer socksConn.Close()
		comm.TcpPipe(conn, socksConn, time.Minute*2)
	} else {
		remoteAddr = fakeDns.dnsToAddr(srcAddr)
		if remoteAddr == "" {
			log.Printf("remoteAddr:%s srcAddr:%s\r\n", remoteAddr, srcAddr)
			return nil
		}
		socksConn, err := net.DialTimeout("tcp", fakeDns.localSocks, time.Second*15)
		if err != nil {
			log.Printf("err:%v", err)
			return nil
		}
		defer socksConn.Close()
		if socks.SocksCmd(socksConn, 1, uint8(addrType), remoteAddr, true) == nil {
			comm.TcpPipe(conn, socksConn, time.Minute*2)
		}
	}
	return nil
}

func (fakeDns *FakeDnsTun) udpForwarder(conn core.CommUDPConn, ep core.CommEndpoint) error {
	var srcAddr = conn.LocalAddr().String()
	var remoteAddr = fakeDns.dnsToAddr(srcAddr)
	if remoteAddr == "" {
		conn.Close()
		return nil
	}
	if fakeDns.tunType == 3 {
		defer ep.Close()
		dstAddr, _ := net.ResolveUDPAddr("udp", remoteAddr)
		log.Printf("udp-remoteAddr:%s\r\n", remoteAddr)
		socks.SocksUdpGate(conn, "127.0.0.1:"+param.Args.Sock5UdpPort, dstAddr)
	}
	//tuntype 直连
	if fakeDns.tunType == 5 {
		defer ep.Close()
		if fakeDns.udpProxy {
			socksConn, err := net.DialTimeout("tcp", fakeDns.localSocks, time.Second*15)
			if err == nil {
				defer socksConn.Close()
				gateWay, err := socks.GetUdpGate(socksConn, remoteAddr)
				log.Printf("gateWay:%s %v\r\n", gateWay, err)
				if err == nil {
					dstAddr, _ := net.ResolveUDPAddr("udp", remoteAddr)
					log.Printf("udp-remoteAddr:%s\r\n", remoteAddr)
					return socks.SocksUdpGate(conn, gateWay, dstAddr)
				}
			}
		}
		fakeDns.UdpDirect(remoteAddr, conn, ep)
	}
	return nil
}

/*直连*/
func (fakeDns *FakeDnsTun) UdpDirect(remoteAddr string, conn core.CommUDPConn, ep core.CommEndpoint) {
	//tuntype 直连
	var limit *comm.UdpLimit
	_limit, ok := fakeDns.udpLimit.Load(remoteAddr)
	if !ok {
		limit = &comm.UdpLimit{Limit: rate.NewLimiter(rate.Every(1*time.Second), 50), Expired: time.Now().Unix() + 5}
	} else {
		limit = _limit.(*comm.UdpLimit)
	}
	//限流
	if limit.Limit.Allow() {
		limit.Expired = time.Now().Unix() + 5
		//本地直连交换
		comm.TunNatSawp(&fakeUdpNat, conn, ep, remoteAddr, 65*time.Second)
		fakeDns.udpLimit.Store(remoteAddr, limit)
	}
}

/*dns addr swap*/
func (fakeDns *FakeDnsTun) dnsToAddr(remoteAddr string) string {
	remoteAddr = fakeDns.dnsToDomain(remoteAddr)
	if remoteAddr == "" {
		return ""
	}
	remoteAddrs := strings.Split(remoteAddr, ":")
	domain := remoteAddrs[0]
	ip, err := fakeDns.safeDns.Resolve(domain[0:len(domain)-1], 4)
	if err != nil {
		return ""
	}
	return ip + ":" + remoteAddrs[1]
}

/*dns addr swap*/
func (fakeDns *FakeDnsTun) dnsToDomain(remoteAddr string) string {
	if fakeDns.tunDns == nil {
		return ""
	}
	remoteAddrs := strings.Split(remoteAddr, ":")
	_domain, ok := fakeDns.tunDns.ip2Domain.Get(remoteAddrs[0])
	if !ok {
		return ""
	}
	return _domain.(string) + ":" + remoteAddrs[1]
}

func (tunDns *TunDns) _startSmartDns(clientPort string) {
	tunDns.run = true
	tunDns.udpServer = &dns.Server{
		Net:          "udp",
		Addr:         ":" + tunDns.dnsPort,
		Handler:      dns.HandlerFunc(tunDns.ServeDNS),
		UDPSize:      4096,
		ReadTimeout:  time.Duration(10) * time.Second,
		WriteTimeout: time.Duration(10) * time.Second,
	}
	tunDns.tcpServer = &dns.Server{
		Net:          "tcp",
		Addr:         ":" + tunDns.dnsPort,
		Handler:      dns.HandlerFunc(tunDns.ServeDNS),
		UDPSize:      4096,
		ReadTimeout:  time.Duration(10) * time.Second,
		WriteTimeout: time.Duration(10) * time.Second,
	}

	localPort, _ := strconv.Atoi(clientPort)
	_dialer := &net.Dialer{Timeout: 10 * time.Second, LocalAddr: &net.UDPAddr{Port: localPort}}
	tunDns.dnsClient = &dns.Client{
		Net:            "udp",
		UDPSize:        4096,
		Dialer:         _dialer,
		SingleInflight: false,
		ReadTimeout:    time.Duration(10) * time.Second,
		WriteTimeout:   time.Duration(10) * time.Second,
	}
	tunDns.dnsClientConn, _ = tunDns.dnsClient.Dial(comm.GetOldDns(tunDns.dnsAddr, tunGW, "") + ":53")
	go tunDns.udpServer.ListenAndServe()
	go tunDns.tcpServer.ListenAndServe()
	go tunDns.checkDnsChange()
	go tunDns.clearDnsCache()
}

func (tunDns *TunDns) Shutdown() {
	tunDns.run = false
	if tunDns.tcpServer != nil {
		tunDns.tcpServer.Shutdown()
	}
	if tunDns.udpServer != nil {
		tunDns.udpServer.Shutdown()
	}
}

/*ipv4查询代理*/
func (tunDns *TunDns) doIPv4Query(r *dns.Msg) (*dns.Msg, error) {
	m := &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = false
	domain := r.Question[0].Name
	v, err := tunDns.ipv4Res(domain)
	if err == nil {
		m.Answer = []dns.RR{v}
	}
	// final
	return m, err
}

/*ipv6查询代理*/
func (tunDns *TunDns) doIPv6Query(r *dns.Msg) (*dns.Msg, error) {
	m := &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = false
	domain := r.Question[0].Name
	v, err := tunDns.ipv6Res(domain)
	_, isA := v.(*dns.A)
	if isA {
		m.Answer = []dns.RR{v.(*dns.A)}
	}
	_, isAAAA := v.(*dns.AAAA)
	if isAAAA {
		m.Answer = []dns.RR{v.(*dns.AAAA)}
	}
	// final
	return m, err
}

func (tunDns *TunDns) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	var msg *dns.Msg
	var err error
	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg, err = tunDns.doIPv4Query(r)
		break
	case dns.TypeAAAA:
		//ipv6
		msg, err = tunDns.doIPv6Query(r)
		break
	default:
		var rtt time.Duration
		msg, rtt, err = tunDns.dnsClient.ExchangeWithConn(r, tunDns.dnsClientConn)
		log.Printf("ServeDNS default rtt:%+v err:%+v\r\n", rtt, err)
		break
	}
	if err != nil {
		msg = &dns.Msg{}
		msg.SetRcode(r, dns.RcodeServerFailure)
	}
	w.WriteMsg(msg)
}

/*ipv4智能响应*/
func (tunDns *TunDns) ipv4Res(domain string) (*dns.A, error) {
	var ip = ""
	var _ip net.IP
	var ipTtl uint32 = 60
	var dnsErr = false
	var backErr error = nil
	ipLog, ok := tunDns.ip2Domain.GetInverse(domain)
	_, excludeFlag := tunDns.excludeDomains[domain]
	if ok && !excludeFlag && strings.HasPrefix(ipLog.(string), tunAddr[0:4]) {
		ip = ipLog.(string)
		ipTtl = 1
	} else {
		if _ip == nil && len(domain) > 0 {
			//为空的话智能dns的话先解析一遍
			var backIp net.IP
			backIp, _, err := tunDns.localResolve(domain, 4)
			if err == nil {
				_ip = backIp
			} else if err.Error() != "Not found addr" {
				log.Printf("local dns error:%v\r\n", err)
				//解析错误说明无网络,否则就算不存在也会回复的
				dnsErr = true //标记为错误
			}
			//如果只是找不到地址没有任何错误可能只有ipv6地址,标记为空
			if err != nil && err.Error() == "Not found addr" {
				//backErr = errors.New("only ipv6")
				dnsErr = true
			}
		}

		//不为空判断是不是中国ip
		if excludeFlag || (_ip != nil && (comm.IsChinaMainlandIP(_ip.String()) || !comm.IsPublicIP(_ip))) {
			//中国Ip直接回复
			if _ip != nil {
				ip = _ip.String()
			}
		} else if !excludeFlag && !dnsErr {
			//外国随机分配一个代理ip
			ip = allocIpByDomain(domain, tunDns)
			ipTtl = 1
		}
	}
	log.Printf("domain:%s ip:%s\r\n", domain, ip)
	return &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ipTtl},
		A:   net.ParseIP(ip),
	}, backErr
}

/*dns缓存自动清理*/
func (tunDns *TunDns) clearDnsCache() {
	for tunDns.run {
		tunDns.fakeDnsCache.Free()
		time.Sleep(time.Second * 60)
	}
}

/*检测旧dns改变*/
func (tunDns *TunDns) checkDnsChange() {
	for tunDns.run {
		if tunDns.dnsClientConn == nil || tunDns.dnsClientConn.RemoteAddr().String() == "" {
			time.Sleep(time.Second * 10)
			continue
		}
		conn, err := net.DialTimeout("tcp", tunDns.dnsClientConn.RemoteAddr().String(), time.Second*1)
		//可能dns变了，
		if err != nil {
			oldDns := comm.GetOldDns(tunDns.dnsAddr, tunGW, "")
			//检测网关DNS是否改变
			if strings.Index(tunDns.dnsClientConn.RemoteAddr().String(), oldDns) == -1 {
				tunDns.dnsClientConn.Close()
				dnsClientConn, err := tunDns.dnsClient.Dial(oldDns + ":53")
				if err == nil {
					tunDns.dnsClientConn = dnsClientConn
				}
			}
		} else {
			conn.Close()
		}
		time.Sleep(time.Second * 10)
	}
}

/*ipv6智能判断*/
func (tunDns *TunDns) ipv6Res(domain string) (interface{}, error) {
	ipLog, ok := tunDns.ip2Domain.GetInverse(domain)
	_, ok1 := ipv6To4.Load(domain)
	_, excludeFlag := tunDns.excludeDomains[domain]
	if ok && ok1 && !excludeFlag && strings.HasPrefix(ipLog.(string), tunAddr[0:4]) {
		//ipv6返回错误迫使使用ipv4地址
		return nil, errors.New("use ipv4")
	}

	//ipv6
	ipStr, rtt, err := tunDns.localResolve(domain, 6)
	if err == nil {
		if ipStr.String() == "" {
			//返回ipv6地址
			return &dns.AAAA{
				Hdr:  dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 1},
				AAAA: net.ParseIP(""),
			}, nil
		}
		ipv6Addr := net.ParseIP(ipStr.String())
		//私有地址或者环路地址或者Teredo地址说明被污染了...返回ipv4的代理ip
		if ipv6Addr.IsPrivate() || ipv6Addr.IsLoopback() || isTeredo(ipv6Addr) {
			ipv6To4.Store(domain, 1)
			//ipv6返回错误迫使使用ipv4地址
			return nil, errors.New("use ipv4")
		} else {
			//返回ipv6地址
			return &dns.AAAA{
				Hdr:  dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
				AAAA: net.ParseIP(ipStr.String()),
			}, nil
		}
	} else {
		log.Printf("ipv6:%s  rtt:%+v err:%+v\r\n", domain, rtt, err)
	}
	return nil, err
}

/*ipv6 teredo addr (4to6)*/
func isTeredo(addr net.IP) bool {
	if len(addr) != 16 {
		return false
	}
	return addr[0] == 0x20 && addr[1] == 0x01 && addr[2] == 0x00 && addr[3] == 0x00
}

/*
本地dns解析有缓存
domain 域名有最后一个"."
*/
func (tunDns *TunDns) localResolve(domain string, ipType int) (net.IP, uint32, error) {
	query := &dns.Msg{}
	if ipType == 4 {
		query.SetQuestion(domain, dns.TypeA)
	}
	if ipType == 6 {
		query.SetQuestion(domain, dns.TypeAAAA)
	}
	cache, ttl := tunDns.fakeDnsCache.ReadDnsCache(domain + ":" + strconv.Itoa(ipType))
	if cache != "" {
		return net.ParseIP(cache), ttl, nil
	}
	m1, rtt, err := tunDns.dnsClient.ExchangeWithConn(query, tunDns.dnsClientConn)
	if err == nil {
		for _, v := range m1.Answer {
			if ipType == 4 {
				record, isType := v.(*dns.A)
				if isType {
					//有些dns会返回127.0.0.1
					if record.A.String() != "127.0.0.1" {
						tunDns.fakeDnsCache.WriteDnsCache(domain+":"+strconv.Itoa(ipType), record.Hdr.Ttl, record.A.String())
						return record.A, record.Hdr.Ttl, nil
					}
				}
			}
			if ipType == 6 {
				record, isType := v.(*dns.AAAA)
				if isType {
					tunDns.fakeDnsCache.WriteDnsCache(domain+":"+strconv.Itoa(ipType), record.Hdr.Ttl, record.AAAA.String())
					return record.AAAA, record.Hdr.Ttl, nil
				}
			}
		}
	} else {
		log.Printf("localResolve:%s  ipType:%d  rtt:%+v err:%+v\r\n", domain, ipType, rtt, err)
		return nil, 0, err
	}
	return nil, 0, errors.New("Not found addr")
}

/*给域名分配私有地址*/
func allocIpByDomain(domain string, tunDns *TunDns) string {
	var ip = ""
	for i := 0; i <= 10; i++ {
		ip = comm.GetCidrRandIpByNet(tunAddr, tunMask)
		_, ok := tunDns.ip2Domain.Get(ip)
		if !ok && ip != tunAddr {
			tunDns.ip2Domain.Insert(ip, domain)
			break
		} else {
			log.Println("ip used up")
			ip = ""
		}
	}
	return ip
}
