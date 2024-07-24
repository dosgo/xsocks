package param

var version = "3.0.0-(20240724)"

var Args *ArgsParam

func init() {
	Args = &ArgsParam{}
	Args.Version = version
	Args.SafeDns = "114.114.114.114"
}

type ArgsParam struct {
	CommParam
	ClientParam
	ServerParam
}

type CommParam struct {
	Version  string
	Mtu      int
	Password string
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
	UdpGatePort   string
}

type ServerParam struct {
	QuicPort string
	WebPort  string
	CertFile string
	KeyFile  string
	SafeDns  string
}
