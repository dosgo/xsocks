package param

//common
var Version ="1.0.2-(20200627)"


//client
var Sock5Addr string
var ServerAddr string
var CaFile string;
var SkipVerify bool;
var Tun2Socks bool;
var UnixSockTun string;
var DnsPort string;
var Mux int;
var LocalDns int;  //use local dns
var Mtu int;

//server
var Sock5Port string
var QuicPort string
var WebPort string
var Password string
var CertFile string;
var KeyFile string;

