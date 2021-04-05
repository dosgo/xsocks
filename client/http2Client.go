package client

import (
	"golang.org/x/net/http2"
	"io"
	"net/http"
	"sync"
	"github.com/dosgo/xSocks/client/httpcomm"
	"github.com/dosgo/xSocks/comm"
	"github.com/dosgo/xSocks/param"
)
/*http2*/


type http2Conn struct {
	sync.Mutex
	client *http.Client;
}

var http2Dialer *http2Conn
func init(){

	http2Dialer=&http2Conn{}
}


func NewHttp2Dialer()  *http2Conn{
	http2Dialer.client=newHttp2Client()
	return http2Dialer;
}

func newHttp2Client() *http.Client{
	tslClientConf:=httpcomm.GetTlsConf();
	t := &http2.Transport{TLSClientConfig: tslClientConf}
	return  &http.Client{Transport: t}
}

func (qd *http2Conn) Dial(url string) (comm.CommConn, error) {
	qd.Lock()
	defer qd.Unlock()
	reader, writer := io.Pipe()
	// Create a request object to send to the server
	req, err := http.NewRequest(http.MethodPost, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Add("token",param.Args.Password)

	// Perform the request
	resp, err := qd.client.Do(req)
	if err != nil {
		return nil, err
	}
	return  comm.HttpConn{writer,nil,resp.Body},nil;
}

