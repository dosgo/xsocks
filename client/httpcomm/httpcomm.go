package httpcomm

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"xSocks/param"
)

func GetTlsConf()*tls.Config{
	tlsconf:=&tls.Config{InsecureSkipVerify:false,ClientSessionCache:  tls.NewLRUClientSessionCache(32)};
	if param.CaFile!="" {
		_, err := os.Stat(param.CaFile)
		if err == nil {
			pool := x509.NewCertPool()
			caCrt, err := os.ReadFile(param.CaFile)
			if err != nil {
				return &tls.Config{};
			}
			pool.AppendCertsFromPEM(caCrt)
			tlsconf.RootCAs=pool;
			return tlsconf;
		}
	}
	if param.SkipVerify {
		tlsconf.InsecureSkipVerify=true;
		return tlsconf;
	}
	return tlsconf;
}