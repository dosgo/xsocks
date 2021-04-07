package tun

import (
	"github.com/dosgo/xsocks/comm"
	"github.com/songgao/water"
	"io"
	"log"
	"errors"
	"net"
	"fmt"
	"os"
)

/*android use unix Socket */
func UsocketToTun(unixSockTun string)(io.ReadWriteCloser,error){
	if len(unixSockTun)>0 {
		os.Remove(unixSockTun)
		addr, err := net.ResolveUnixAddr("unixpacket",unixSockTun)
		if err != nil {
			return nil,err;
		}
		lis, err := net.ListenUnix("unixpacket", addr)
		if err != nil {                      //如果监听失败，一般是文件已存在，需要删除它
			log.Println("UNIX Domain Socket 创 建失败，正在尝试重新创建 -> ", err)
			os.Remove(unixSockTun)
			return nil,err;
		}
		conn, err := lis.Accept() //开始接 受数据
		if err != nil {                      //如果监听失败，一般是文件已存在，需要删除它
			return nil,err;
		}
		return conn,nil;
	}
	return nil,errors.New("unixSockTun null");
}
/*windows linux mac use tun dev*/
func RegTunDev(tunDevice string,tunAddr string,tunMask string,tunGW string,tunDNS string)(*water.Interface,error){
	if len(tunDevice)==0 {
		tunDevice="tun0";
	}
	if len(tunAddr)==0 {
		tunAddr="10.0.0.2";
	}
	if len(tunMask)==0 {
		tunMask="255.255.255.0";
	}
	if len(tunGW)==0 {
		tunGW="10.0.0.1";
	}
	if len(tunDNS)==0 {
		tunDNS="114.114.114.114";
	}

	config := comm.GetWaterConf(tunAddr,tunMask);
	ifce, err := water.New(config)
	if err != nil {
		fmt.Println("start tun err:", err)
		return nil,err;
	}
	return ifce,nil;
}


