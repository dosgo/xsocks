package client

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
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
	if strings.HasPrefix(param.Args.ServerAddr,"wss") {
		stream, err = NewWsYamuxDialer().Dial(param.Args.ServerAddr)
	}else if strings.HasPrefix(param.Args.ServerAddr,"http2") {
		stream, err = NewHttp2Dialer().Dial("https"+param.Args.ServerAddr[5:])
	}else if strings.HasPrefix(param.Args.ServerAddr,"http") {
		stream, err = NewHttpDialer().Dial("https"+param.Args.ServerAddr[4:])
	}else if strings.HasPrefix(param.Args.ServerAddr,"quic") {
		stream, err = NewQuicDialer().Dial(param.Args.ServerAddr[7:])
	}else if strings.HasPrefix(param.Args.ServerAddr,"kcp") {
		stream, err = NewKcpDialer().Dial(param.Args.ServerAddr[6:])
	}


	if err != nil  {
		return nil,err
	}
	if  stream == nil {
		return nil,errors.New("stream null")
	}
	//write password
	passwordBuf := comm.GenPasswordHead(param.Args.Password);
	_,err=stream.Write([]byte(passwordBuf))
	if err != nil  {
		return nil,err
	}
	return stream,nil;
}


func  ResetTunnel () {
	if strings.HasPrefix(param.Args.ServerAddr,"wss") {

	}
	if strings.HasPrefix(param.Args.ServerAddr,"quic") {
		ClearQuicDialer();
	}
	if strings.HasPrefix(param.Args.ServerAddr,"kcp") {

	}
}

func regRoute(tunAddr string,remoteAddr string,dnsServers []string,oldGw string,localNetwork string){
	//if localNetwork!="" {
	//	localRoute:=strings.Split(localNetwork,"_")
		//exec.Command("route", "add",localRoute[0],"mask",localRoute[1],oldGw,"metric","6").Output();
	//}


	//delete old
	exec.Command("route", "delete","0.0.0.0").Output()
	// add socks5 add
	exec.Command("route", "add",remoteAddr,oldGw,"metric","6").Output()
	for _, v := range dnsServers {
		// add dns add
		exec.Command("route", "add",v,oldGw,"metric","6").Output()
	}
	//route add 0.0.0.0 mask 0.0.0.0 192.168.8.1 metric 6
	exec.Command("route", "add","0.0.0.0","mask","0.0.0.0",tunAddr,"metric","8").Output();
}

func unRegRoute(tunAddr string,remoteAddr string,dnsServers []string,oldGw string,localNetwork string){

	//if localNetwork!="" {
		//localRoute:=strings.Split(localNetwork,"_")
	//	exec.Command("route", "delete",localRoute[0],"mask",localRoute[1],oldGw).Output()
	//}


	exec.Command("route", "delete","0.0.0.0","mask","0.0.0.0",tunAddr,"metric","6").Output()
	//route add old
	exec.Command("route", "add","0.0.0.0","mask","0.0.0.0",oldGw,"metric","8").Output()
	//delete remoteAddr
	exec.Command("route", "delete",remoteAddr).Output()
	//delete dns
	for _, v := range dnsServers {
		// delete dns
		exec.Command("route", "delete",v).Output()
	}
}

func routeEdit(tunGW string,remoteAddr string, dnsServers []string,oldGw string){
	if oldGw=="" {
		oldGw="192.168.1.1";
	}

	var localNetwork="";
	/*
	if runtime.GOOS=="windows" {
		lAdds,err:=comm.GetLocalAddresses();
		if err==nil {
			for _, v := range lAdds {
				if strings.Index(v.GateWay,oldGw)!=-1 {
					//route add 10.108.0.0 mask 255.255.0.0 10.10.20.1 -p
					masks:=strings.Split(v.IpMask,".")
					ips:=strings.Split(v.IpAddress,".");
					var tmpIp=make([]string,4);
					for i := 0; i <= 3; i++ {
						if masks[i]=="255" {
							tmpIp[i]=ips[i]
						}else{
							tmpIp[i]="0";
						}
					}
					localNetwork=strings.Join(tmpIp,".")+"_"+v.IpMask;
					break;
				}
			}
		}
	}*/

	ch := make(chan os.Signal, 1)
	signal.Notify(ch,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
		syscall.SIGABRT,
		syscall.SIGSEGV,
		syscall.SIGQUIT)
	go func() {
		_ = <-ch
		if runtime.GOOS=="windows" {
			unRegRoute(tunGW,remoteAddr,dnsServers,oldGw,localNetwork);
		}
		os.Exit(0);
	}()



	//windows
	if runtime.GOOS=="windows" {
		regRoute(tunGW,remoteAddr,dnsServers,oldGw,localNetwork);
	}
}



