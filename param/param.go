package param

import "time"

var version = "2.0.0-(20231222)"

var Args *ArgsParam

func init() {
	Args = &ArgsParam{}
	Args.ConnectTime = time.Second * 10
	Args.Version = version
	Args.SafeDns = "114.114.114.114"
}

type ArgsParam struct {
	CommParam
	ClientParam
	ServerParam
}

type CommParam struct {
	Version     string
	Mtu         int
	Password    string
	ConnectTime time.Duration `json:"-"` //=10*time.Second;
	UdpGatePort string
}

type ClientParam struct {
	Sock5Addr     string
	ServerAddr    string
	ExcludeDomain string
	CaFile        string
	SkipVerify    bool
	TunType       int
	UnixSockTun   string
	TunFd         int
	DnsPort       string
	UdpProxy      int
	MuxNum        int
	LocalDns      int //use local dns
	SmartDns      int //use Smart dns
	TunSmartProxy bool
	Sock5UdpPort  string
	IpFile        string
	AutoStart     bool
}

type ServerParam struct {
	Sock5Port    string
	QuicPort     string
	WebPort      string
	CertFile     string
	KeyFile      string
	LocalTunSock string
	SafeDns      string
}
