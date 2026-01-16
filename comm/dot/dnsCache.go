package dot

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

type CachedResponse struct {
	resp      *dns.Msg
	cacheTime int64
}

type DnsCacheV1 struct {
	Cache map[string]CachedResponse
	sync.Mutex
}

func (rd *DnsCacheV1) Free(expired int64) {
	rd.Lock()
	defer rd.Unlock()
	for k, v := range rd.Cache {
		//60ms
		if time.Now().Unix() > v.cacheTime+expired {
			delete(rd.Cache, k)
		}
	}
}

func (rd *DnsCacheV1) ReadDnsCache(domain string, expired int64) *dns.Msg {
	rd.Lock()
	defer rd.Unlock()
	if v, ok := rd.Cache[domain]; ok {
		//60ms
		if time.Now().Unix() < v.cacheTime+expired {
			return v.resp
		}
	}
	return nil
}
func (rd *DnsCacheV1) WriteDnsCache(domain string, msg *dns.Msg) string {
	rd.Lock()
	defer rd.Unlock()
	rd.Cache[domain] = CachedResponse{msg, time.Now().Unix()}
	return ""
}
