package server

import (
	"golang.org/x/net/websocket"
	"net/http"
	"os"
	"strings"
	"time"
	"xSocks/comm"
	"xSocks/param"
)

/*websocket + http2*/

func StartWeb(addr string) error {
	http.HandleFunc("/",webHandler)
	if param.Args.KeyFile==""||param.Args.CertFile=="" {
		param.Args.KeyFile="localhost_server.key"
		param.Args.CertFile="localhost_server.pem"
		addrs:=strings.Split(addr,":")
		var ip="127.0.0.1";
		if(addrs[0]!="0.0.0.0"||addrs[0]!=""){
			 ip=addrs[0];
		}
		_,err:=os.Stat(param.Args.KeyFile)
		if err!=nil {
			genCERT("improvement","localhost",ip);
		}
	}

	err :=http.ListenAndServeTLS(addr,param.Args.CertFile,param.Args.KeyFile,nil)

	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
	return nil;
}


func webHandler(w http.ResponseWriter, req *http.Request){
	if req.Header.Get("token")!=param.Args.Password {
		msg:="Current server time:"+time.Now().Format("2006-01-02 15:04:05");
		w.Header().Add("Connection","Close")
		w.Header().Add("Content-Type","text/html")
		w.Write([]byte(msg))
		return
	}
	//http2
	if req.ProtoMajor == 2 {
		w.WriteHeader(http.StatusOK)
		// First flash response headers
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
			proxy(comm.HttpConn{w, f, req.Body})
		}
	}else {
		//http 1.1 connect　proxy
		if req.Method==http.MethodConnect {
			w.WriteHeader(http.StatusOK)
			hijacker, ok := w.(http.Hijacker)
			if ok {
				//接管连接
				client_conn, _, err := hijacker.Hijack()
				if err==nil {
					proxy(client_conn)
				}
			}
		}else {
			//web socket
			websocket := websocket.Handler(wsToStream);
			websocket.ServeHTTP(w, req);
		}
	}
}



/* wsToStream*/
func wsToStream(ws *websocket.Conn) {
	streamToSocks5Yamux(ws)
}




