package client

import (
	"golang.org/x/net/http2"
	"io"
	"net/http"
	"xSocks/client/httpcomm"
	"xSocks/client/muxComm"
	"xSocks/comm"
	"xSocks/param"
)
var http2YamuxDialer *muxComm.YamuxComm
func init(){
	http2YamuxDialer= muxComm.NewYamuxDialer(dialHttp2)
}

func NewHttp2YamuxDialer()  *muxComm.YamuxComm{
	return http2YamuxDialer;
}

func dialHttp2(url string)(io.ReadWriteCloser, error){
	tslClientConf:=httpcomm.GetTlsConf();
	t := &http2.Transport{
		TLSClientConfig: tslClientConf,
	}
	client := &http.Client{Transport: t,}
	reader, writer := io.Pipe()
	// Create a request object to send to the server
	req, err := http.NewRequest(http.MethodPost, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Add("token",param.Password)
	// Perform the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return  comm.HttpConn{writer,nil,resp.Body},nil;
}

