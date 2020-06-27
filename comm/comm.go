package comm

import "io"

type CommConn interface {
	io.Reader
	io.Writer
	io.Closer
}

