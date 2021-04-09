package muxComm

import (
	"io"
)

type DialConn func(url string) (io.ReadWriteCloser, error)