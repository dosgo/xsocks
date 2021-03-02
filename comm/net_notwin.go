// +build !windows

package comm


func GetGateway()string {

	return "";
}

func GetDnsServer() []string {
	dns := []string{}
	return dns;
}


func GetLocalAddresses() ([]lAddr ,error) {
	lAddrs := []lAddr{}
	return lAddrs,nil;
}

func SetDNSServer(gwIp string,ip string){

}

func AddRoute(tunNet string,tunGw string, tunMask string){

}
func GetDnsServerByGateWay(gwIp string)string{
	return "";
}