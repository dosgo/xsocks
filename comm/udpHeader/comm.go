package udpHeader

import (
	"bytes"
	"net"
	"syscall"
	"time"
)
import "math/rand"


func RollUint16() uint32 {
	return uint32( uint16(rand.Intn(65536)))
}


type UdpConn struct {
	conn *net.UDPConn
	buffer *bytes.Buffer
	vc *VideoChat
}

func NewUdpConn(conn *net.UDPConn) *UdpConn {
	return &UdpConn{conn, new(bytes.Buffer),&VideoChat{sn: uint32(RollUint16())}}
}


func (uh *UdpConn) ReadFrom(b []byte) ( int,  net.Addr,  error) {
	data := make([]byte,65535)
	n,addr,err:=uh.conn.ReadFrom(data);
	if err!=nil || n==0 {
		return n,addr,err
	}

	//b=data[uh.vc.Size():n];
	_n:=copy(b,data[uh.vc.Size():n]);
	return _n,addr,err;
}


// Serialize implements PacketHeader.
func (uh *UdpConn) WriteTo(p []byte, addr net.Addr) (int,  error) {
	var header []byte = make([]byte, uh.vc.Size())
	uh.vc.Serialize(header)
	uh.buffer.Reset()
	uh.buffer.Write(header)
	uh.buffer.Write(p)
	n, err := uh.conn.WriteTo(uh.buffer.Bytes(), addr)
	if err != nil {
		return n, err
	}
	return len(p),err;
}
func (c *UdpConn) Close() error {
	return c.conn.Close()
}

func (c *UdpConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *UdpConn) Write(b []byte) (int, error) {
	var header []byte = make([]byte, c.vc.Size())
	c.vc.Serialize(header)
	c.buffer.Reset()
	c.buffer.Write(header)
	c.buffer.Write(b)
	return c.conn.Write(c.buffer.Bytes())
}

func (c *UdpConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *UdpConn) SetReadBuffer(bytes int) error{
	return c.conn.SetReadBuffer(bytes)
}

func (c *UdpConn) SyscallConn() (syscall.RawConn, error) {
	return c.conn.SyscallConn()
}

func (c *UdpConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *UdpConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

