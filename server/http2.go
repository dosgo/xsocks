package server

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"xSocks/comm"
	"xSocks/param"
)



func StartHttp2(addr string) error {
	if param.KeyFile==""||param.CertFile=="" {
		param.KeyFile="localhost_server.key"
		param.CertFile="localhost_server.pem"
		addrs:=strings.Split(addr,":")
		var ip="127.0.0.1";
		if(addrs[0]!="0.0.0.0"||addrs[0]!=""){
			 ip=addrs[0];
		}
		_,err:=os.Stat(param.KeyFile)
		if err!=nil {
			genCERT("improvement","localhost",ip);
		}
	}
	srv := &http.Server{Addr: addr, Handler: http.HandlerFunc(http2Handler)}
	err :=srv.ListenAndServeTLS(param.CertFile,param.KeyFile)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
	return nil;
}



func http2Handler(w http.ResponseWriter, r *http.Request) {
	//http2.0 check
	if r.ProtoMajor != 2 {
		log.Println("Not a HTTP/2 request, rejected!")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if r.Header.Get("token")!=param.Password {
		msg:="Current server time:"+time.Now().Format("2006-01-02 15:04:05");
		w.Header().Add("Connection","Close")
		w.Header().Add("Content-Type","text/html")
		w.Write([]byte(msg))
		return
	}else {
		w.WriteHeader(200)
	}
	// First flash response headers
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
		proxy(comm.HttpConn{w, f, r.Body})
	}
}