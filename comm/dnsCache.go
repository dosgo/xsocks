package comm

import (
	"sync"
	"time"
)

type IpInfo struct {
	cacheTime int64
	ip        string
}

type DnsCache struct {
	Cache map[string]IpInfo
	sync.Mutex
}

func (rd *DnsCache) Free() {
	rd.Lock()
	defer rd.Unlock()
	for k, v := range rd.Cache {
		//60ms
		if time.Now().Unix() > v.cacheTime {
			delete(rd.Cache, k)
		}
	}
}

func (rd *DnsCache) ReadDnsCache(remoteHost string) (string, uint32) {
	rd.Lock()
	defer rd.Unlock()
	if v, ok := rd.Cache[remoteHost]; ok {

		//60ms
		if time.Now().Unix() < v.cacheTime {
			return v.ip, uint32(v.cacheTime - time.Now().Unix())
		}
	}
	return "", 0
}
func (rd *DnsCache) WriteDnsCache(remoteHost string, ttl uint32, ip string) string {
	rd.Lock()
	defer rd.Unlock()
	rd.Cache[remoteHost] = IpInfo{time.Now().Unix() + int64(ttl), ip}
	return ""
}
