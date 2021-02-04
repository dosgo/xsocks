package param

import "time"

//common
var Version ="1.2.6-(20210204)"


//client
var Sock5Addr string
var ServerAddr string
var CaFile string;
var SkipVerify bool;
var TunType int;
var UnixSockTun string;
var DnsPort string;
var MuxNum int;
var LocalDns int;  //use local dns
var TunSmartProxy bool;


//comm
var Mtu int;
var ConnectTime =10*time.Second;

//server
var Sock5Port string
var Sock5UdpPort string
var QuicPort string
var WebPort string
var SudpPort string;
var KcpPort string;
var Password string
var CertFile string;
var KeyFile string;
var TunPort string;
var LocalTunSock string;
var SafeDns string="114.114.114.114";
