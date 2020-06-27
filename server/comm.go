package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"qproxy/comm"
	"qproxy/client"
	"qproxy/param"
)



func proxy(conn comm.CommConn){
	defer conn.Close()
	//read auth Head
	authHead := make([]byte,16)
	_, err := io.ReadFull(conn, authHead)
	if err != nil {
		fmt.Printf("err:%v\r\n",err)
		return
	}
	//autherr;
	if(string(authHead)!= client.GenPasswordHead(param.Password)){
		return ;
	}
	//read cmd
	cmd := make([]byte,1)
	_, err = io.ReadFull(conn, cmd)
	if err != nil {
		return
	}
	switch cmd[0] {
	//dns
	case 0x01:
		dnsResolve(conn);
		break
	//to socks5
	case 0x02:
		//连接socks5
		sConn, err := net.Dial("tcp", "127.0.0.1:"+param.Sock5Port)
		if(err!=nil){
			log.Printf("err:%v\r\n",param.Sock5Port)
			return ;
		}
		defer sConn.Close();
		//交换数据
		go io.Copy(sConn, conn)
		io.Copy(conn, sConn)
		break;
	}
}

/*dns解析*/
func dnsResolve(conn comm.CommConn) {
	for{
		hostLen := make([]byte,1)
		_, err := io.ReadFull(conn, hostLen)
		if err != nil {
			return
		}
		hostBuf := make([]byte, hostLen[0])
		_, err = io.ReadFull(conn, hostBuf)
		if err != nil {
			return
		}
		addr, err := net.ResolveIPAddr("ip4", string(hostBuf))
		if(err!=nil){
			conn.Write([]byte{0x01, 0x04,0x00, 0x00, 0x00, 0x00}) //0x01==error  0x04==ipv4
			return ;
		}
		_, err =conn.Write([]byte{0x00, 0x04}) //响应客户端
		_, err =conn.Write(addr.IP.To4()) //响应客户端
		if(err!=nil){
			return ;
		}
	}
}


func regRoute(tunAddr string,socks5Addr string,dnsServers []string,oldGw string){
	//delete old
	exec.Command("route", "delete","0.0.0.0").Output()

	// add socks5 add
	exec.Command("route", "add",socks5Addr,oldGw,"metric","6").Output()
	//route add 0.0.0.0 mask 0.0.0.0 192.168.8.1 metric 6
	exec.Command("route", "add","0.0.0.0","mask","0.0.0.0",tunAddr,"metric","6").Output();
//	for _, dns := range dnsServers {
		//exec.Command("route", "add",dns,oldGw,"metric","6").Output();
	//}
//	exec.Command("route", "add","114.114.114.114",oldGw,"metric","6").Output()

}

func unRegRoute(tunAddr string,oldGw string){
	exec.Command("route", "delete","0.0.0.0","mask","0.0.0.0",tunAddr,"metric","6").Output()
	//route add old
	exec.Command("route", "add","0.0.0.0","mask","0.0.0.0",oldGw,"metric","6").Output()
}