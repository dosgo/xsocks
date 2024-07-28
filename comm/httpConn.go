package comm

import (
	"io"
	"net/http"
	"time"
)

type HttpConn struct {
	W  io.Writer
	Wf http.Flusher
	R  io.ReadCloser
}

func (conn HttpConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (conn HttpConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (conn HttpConn) Read(buf []byte) (int, error) {
	return conn.R.Read(buf)
}

func (conn HttpConn) Write(buf []byte) (int, error) {
	n, err := conn.W.Write(buf)
	if conn.Wf != nil {
		conn.Wf.Flush()
	}
	return n, err
}
func (conn HttpConn) Close() error {
	return conn.R.Close()
}
