package tun

import (
	"errors"
	"io"
	"log"
	"net"
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
