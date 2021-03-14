package client

import (
	"io"
	"net/http"
	"sync"
	"xSocks/client/httpcomm"
	"xSocks/comm"
	"xSocks/param"
)

/*
http1.1
*/

type httpConn struct {
	sync.Mutex
	client *http.Client;
}

var httpDialer *httpConn
func init(){

	httpDialer=&httpConn{}
}


func NewHttpDialer()  *httpConn{
	httpDialer.client=newHttpClient()
	return httpDialer;
}

func newHttpClient() *http.Client{
	tslClientConf:=httpcomm.GetTlsConf();
	t := &http.Transport{TLSClientConfig: tslClientConf}
	return  &http.Client{Transport: t}
}

func (qd *httpConn) Dial(url string) (comm.CommConn, error) {
	qd.Lock()
	defer qd.Unlock()



	reader, writer := io.Pipe()
	// Create a request object to send to the server
	req, err := http.NewRequest(http.MethodConnect, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Add("token",param.Password)


	// Perform the request
	resp, err := qd.client.Do(req)
	if err != nil {
		return nil, err
	}
	return  comm.HttpConn{writer,nil,resp.Body},nil;
}

