// +build linux

package bluetooth

import (
	"fmt"
	"net"
	"syscall"
	"time"
)

// conn represents one RFCOMM connection between two bluetooth devices.
// It implements the net.Conn interface.
type conn struct {
	fd                    *fd
	localAddr, remoteAddr net.Addr
	readDeadline          time.Time
	writeDeadline         time.Time
}

func newConn(sockfd int, local, remote net.Addr) (net.Conn, error) {
	fd, err := newFD(sockfd)
	if err != nil {
		syscall.Close(sockfd)
		return nil, err
	}
	return &conn{fd: fd, localAddr: local, remoteAddr: remote}, nil
}

func (c *conn) String() string {
	return fmt.Sprintf("Bluetooth (%s) <--> (%s)", c.localAddr, c.remoteAddr)
}

// net.Conn interface methods
func (c *conn) Read(p []byte) (n int, err error)   { return c.fd.Read(p) }
func (c *conn) Write(p []byte) (n int, err error)  { return c.fd.Write(p) }
func (c *conn) Close() error                       { return c.fd.Close() }
func (c *conn) LocalAddr() net.Addr                { return c.localAddr }
func (c *conn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *conn) SetReadDeadline(t time.Time) error  { return c.setSockoptTimeval(t, syscall.SO_RCVTIMEO) }
func (c *conn) SetWriteDeadline(t time.Time) error { return c.setSockoptTimeval(t, syscall.SO_SNDTIMEO) }
func (c *conn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

func (c *conn) setSockoptTimeval(t time.Time, opt int) error {
	fd, err := c.fd.Reference()
	if err != nil {
		return err
	}
	defer c.fd.ReleaseReference()
	if timeout := getTimeout(t); timeout != nil {
		return syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, opt, timeout)
	}
	return nil
}

// getTimeout returns timeout for socket read/write operations, given the
// deadline specified as absolute time.  Return value nil indicates no timeout.
// Return value 0 indicates that the read/write operation should timeout
// immediately.
func getTimeout(deadline time.Time) *syscall.Timeval {
	if deadline.IsZero() {
		return nil
	}
	d := deadline.Sub(time.Now())
	if d < 0 {
		ret := syscall.NsecToTimeval(0)
		return &ret
	}
	ret := syscall.NsecToTimeval(d.Nanoseconds())
	return &ret
}
