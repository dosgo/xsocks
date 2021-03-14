package server

import (
	"fmt"
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

	err :=http.ListenAndServeTLS(addr,param.CertFile,param.KeyFile,nil)

	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
	return nil;
}


func webHandler(w http.ResponseWriter, req *http.Request){
	if req.Header.Get("token")!=param.Password {
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
			fmt.Printf("MethodConnect\r\n")
			w.WriteHeader(http.StatusOK)
			hijacker, ok := w.(http.Hijacker)
			if ok {
				fmt.Printf("MethodConnect2\r\n")
				//接管连接
				client_conn, _, err := hijacker.Hijack()
				if err==nil {
					fmt.Printf("MethodConnect3\r\n")
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




