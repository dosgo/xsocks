package client

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"xSocks/comm"
	"xSocks/param"
)

var poolDnsBuf = &sync.Pool{
	New: func() interface{} {
		log.Println("new 1")
		return make([]byte, 4096)
	},
}

func  NewTunnel () (comm.CommConn,error){
	var err error;
	//解析
	var stream comm.CommConn;
	if (strings.HasPrefix(param.ServerAddr,"wss")) {
		stream, err = NewWsYamuxDialer().Dial(param.ServerAddr)
	}
	if (strings.HasPrefix(param.ServerAddr,"quic")) {
		stream, err = NewQuicDialer().Dial(param.ServerAddr[7:])
	}
	if (strings.HasPrefix(param.ServerAddr,"kcp")) {
		stream, err = NewKcpDialer().Dial(param.ServerAddr[6:])
	}


	if err != nil || stream == nil {
		return nil,err
	}
	//write password
	passwordBuf := comm.GenPasswordHead(param.Password);
	_,err=stream.Write([]byte(passwordBuf))
	if err != nil  {
		return nil,err
	}
	stream.SetDeadline(time.Now().Add(time.Second*50))
	return stream,nil;
}


func  ResetTunnel () {
	if (strings.HasPrefix(param.ServerAddr,"wss")) {

	}
	if (strings.HasPrefix(param.ServerAddr,"quic")) {
		ClearQuicDialer();
	}
	if (strings.HasPrefix(param.ServerAddr,"kcp")) {

	}
}

func regRoute(tunAddr string,remoteAddr string,dnsServers []string,oldGw string){
	//delete old
	exec.Command("route", "delete","0.0.0.0").Output()
	// add socks5 add
	exec.Command("route", "add",remoteAddr,oldGw,"metric","6").Output()
	for _, v := range dnsServers {
		// add dns add
		exec.Command("route", "add",v,oldGw,"metric","6").Output()
	}
	//route add 0.0.0.0 mask 0.0.0.0 192.168.8.1 metric 6
	exec.Command("route", "add","0.0.0.0","mask","0.0.0.0",tunAddr,"metric","6").Output();
}

func unRegRoute(tunAddr string,remoteAddr string,dnsServers []string,oldGw string){
	exec.Command("route", "delete","0.0.0.0","mask","0.0.0.0",tunAddr,"metric","6").Output()
	//route add old
	exec.Command("route", "add","0.0.0.0","mask","0.0.0.0",oldGw,"metric","6").Output()
	//delete remoteAddr
	exec.Command("route", "delete",remoteAddr).Output()
	//delete dns
	for _, v := range dnsServers {
		// delete dns
		exec.Command("route", "delete",v).Output()
	}
}

func routeEdit(tunGW string,remoteAddr string, dnsServers []string,oldGw string){
	if(oldGw==""){
		oldGw="192.168.1.1";
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-ch
		switch s {
		default:
			if(runtime.GOOS=="windows") {
				unRegRoute(tunGW,remoteAddr,dnsServers,oldGw);
			}
			os.Exit(0);
		}
	}()

	//windows
	if(runtime.GOOS=="windows"){
		regRoute(tunGW,remoteAddr,dnsServers,oldGw);
	}
}