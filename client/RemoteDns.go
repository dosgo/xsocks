package client

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"xSocks/comm"
)
var dnsCache *DnsCache
var dnsIng sync.Map
//污染的域名不用再尝试解析
var PolluteDomainName sync.Map


func init(){
	dnsCache = &DnsCache{cache: make(map[string]string, 128)}
}


type DnsCache struct {
	cache        map[string]string;
	sync.Mutex
}

type RemoteDns struct {
	Tunnel comm.CommConn
	sync.Mutex
}

func (rd *RemoteDns)Connect() (comm.CommConn,error){
	sendBuf:=[]byte{};
	//cmd
	sendBuf =append(sendBuf,0x01);//dns
	var err error;
	tunnel, err:= NewTunnel();
	if(err!=nil){
		return nil,err;
	}
	_,err=tunnel.Write(sendBuf)
	if(err!=nil){
		return nil,err;
	}
	return tunnel,nil;
}

func (rd *RemoteDns)Resolve(remoteHost string) (string,error){
	rd.Lock();
	defer  rd.Unlock()
	var err error
	cache:= readDnsCache(remoteHost)
	if(cache!=""){
		return  cache,nil;
	}
	if(rd.Tunnel==nil) {
		fmt.Printf("Resolve Tunnel null connect\r\n")
		tunnel,err := rd.Connect();
		if (err != nil) {
			fmt.Printf("Resolve1\r\n")
			return "",err
		}
		rd.Tunnel=tunnel;
	}
	sendBuf:=[]byte{};
	hostLen:=uint8( len(remoteHost))
	sendBuf =append(sendBuf,hostLen)
	//host
	sendBuf =append(sendBuf, []byte(remoteHost)...)

	_, err = rd.Tunnel.Write(sendBuf)
	//失败重新连接
	if err != nil {
		fmt.Printf("Resolve2-2\r\n")
		tunnel,err1 := rd.Connect();
		if(err1!=nil){
			fmt.Printf("Resolve2\r\n")
			return "",err1
		}
		rd.Tunnel=tunnel;
		_, err = rd.Tunnel.Write(sendBuf)
		if(err!=nil){
			fmt.Printf("Resolve3\r\n")
			return "",err
		}
	}

	backHead := make([]byte,2)
	_, err = io.ReadFull(rd.Tunnel, backHead)
	if err != nil {
		//回收
		if(rd.Tunnel!=nil) {
			rd.Tunnel.Close();
			rd.Tunnel = nil;
		}
		fmt.Printf("Resolve4 err:%v\r\n",err)
		return "",err
	}
	if(backHead[0]!=0x00){
		fmt.Printf("Resolve5 backHead:%v\r\n",backHead)
		return "",errors.New("remote dns err");
	}
	//ipv4
	if(backHead[1]==0x04){
		ipBuf := make([]byte,4)
		_, err = io.ReadFull(rd.Tunnel, ipBuf)
		if err != nil {
			fmt.Printf("Resolve6 err:%v\r\n",err)
			return "",err
		}
		ipAddr := net.IPv4(ipBuf[0], ipBuf[1], ipBuf[2], ipBuf[3])
		writeDnsCache(remoteHost,ipAddr.String())
		return ipAddr.String(),nil;
	}else{
		fmt.Printf("backHead no v4 backHead:%v\r\n",backHead)
	}
	return "",errors.New("err");
}




func readDnsCache(remoteHost string)string{
	dnsCache.Lock();
	defer dnsCache.Unlock();
	if v, ok := dnsCache.cache[remoteHost]; ok {
		cache:=strings.Split(v,"_")
		cacheTime, _ := strconv.ParseInt(cache[1], 10, 64)
		//60ms
		if time.Now().Unix()-cacheTime<3*60 {
			return cache[0];
		}
	}
	return "";
}
func writeDnsCache(remoteHost string,ip string)string{
	dnsCache.Lock();
	defer dnsCache.Unlock();
	dnsCache.cache[remoteHost]=ip+"_"+strconv.FormatInt(time.Now().Unix(),10)
	return "";
}

/*优化 通过socks5解析*/
func RemoteDnsV1(remoteHost string)(string,error){
	var fornum=0;
	for{
		_,ok := dnsIng.Load(remoteHost)
		if(!ok){
			break;
		}
		time.Sleep(time.Millisecond*5)
		fornum++;
		//15ms
		if fornum>3000 {
			break;
		}
	}
	dnsIng.Store(remoteHost,1)
	defer dnsIng.Delete(remoteHost)

	cache:= readDnsCache(remoteHost)
	if cache!="" {
		return  cache,nil;
	}

	sendBuf:=[]byte{};
	//cmd
	sendBuf =append(sendBuf,0x01);//dns
	hostLen:=uint8( len(remoteHost))
	sendBuf =append(sendBuf,hostLen)
	//host
	sendBuf =append(sendBuf, []byte(remoteHost)...)
	stream, err := NewTunnel();
	_, err = stream.Write(sendBuf)
	if err != nil {
		return "",err
	}
	//back errr
	backHead := make([]byte,2)
	_, err = io.ReadFull(stream, backHead)
	if err != nil {
		return "",err
	}
	if backHead[0]!=0x00 {
		return "",err;
	}
	//ipv4
	if backHead[1]==0x04 {
		ipBuf := make([]byte,4)
		_, err = io.ReadFull(stream, ipBuf)
		if err != nil {
			return "",err
		}
		ipAddr := net.IPv4(ipBuf[0], ipBuf[1], ipBuf[2], ipBuf[3])
		writeDnsCache(remoteHost,ipAddr.String())
		return ipAddr.String(),nil;
	}
	return  "",errors.New("err");
}

