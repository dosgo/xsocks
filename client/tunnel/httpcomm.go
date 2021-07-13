package tunnel

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"github.com/dosgo/xsocks/param"
)

func GetTlsConf()*tls.Config{
	tlsconf:=&tls.Config{InsecureSkipVerify:false,ClientSessionCache:  tls.NewLRUClientSessionCache(32)};
	if param.Args.CaFile!="" {
		_, err := os.Stat(param.Args.CaFile)
		if err == nil {
			pool := x509.NewCertPool()
			caCrt, err := os.ReadFile(param.Args.CaFile)
			if err != nil {
				return &tls.Config{};
			}
			pool.AppendCertsFromPEM(caCrt)
			tlsconf.RootCAs=pool;
			return tlsconf;
		}
	}
	if param.Args.SkipVerify {
		tlsconf.InsecureSkipVerify=true;
		return tlsconf;
	}
	return tlsconf;
}