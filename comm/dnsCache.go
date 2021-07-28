package comm

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

type DnsCache struct {
	Cache        map[string]string;
	sync.Mutex
}
func (rd *DnsCache)ReadDnsCache(remoteHost string)(string,uint32) {
	rd.Lock();
	defer rd.Unlock();
	if v, ok := rd.Cache[remoteHost]; ok {
		cache:=strings.Split(v,"_")
		cacheTime, _ := strconv.ParseInt(cache[1], 10, 64)
		//60ms
		if time.Now().Unix()<cacheTime {
			return cache[0],uint32(cacheTime-time.Now().Unix());
		}
	}
	return "",0;
}
func (rd *DnsCache)WriteDnsCache(remoteHost string,ttl uint32,ip string)string{
	rd.Lock();
	defer rd.Unlock();
	rd.Cache[remoteHost]=ip+"_"+strconv.FormatInt(time.Now().Unix()+int64(ttl),10)
	return "";
}

