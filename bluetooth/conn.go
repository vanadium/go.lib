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
	fd                    int
	localAddr, remoteAddr *addr
	readDeadline          time.Time
	writeDeadline         time.Time
}

func (c *conn) String() string {
	return fmt.Sprintf("Bluetooth (%s) <--> (%s)", c.localAddr, c.remoteAddr)
}

// Implements the net.Conn interface.
func (c *conn) Read(p []byte) (n int, err error) {
	return syscall.Read(c.fd, p)
}

// Implements the net.Conn interface.
func (c *conn) Write(p []byte) (n int, err error) {
	return syscall.Write(c.fd, p)
}

// Implements the net.Conn interface.
func (c *conn) Close() error {
	return syscall.Close(c.fd)
}

// Implements the net.Conn interface.
func (c *conn) LocalAddr() net.Addr {
	return c.localAddr
}

// Implements the net.Conn interface.
func (c *conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// Implements the net.Conn interface.
func (c *conn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

// Implements the net.Conn interface.
func (c *conn) SetReadDeadline(t time.Time) error {
	if timeout := getTimeout(t); timeout != nil {
		return syscall.SetsockoptTimeval(c.fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, timeout)
	}
	return nil
}

// Implements the net.Conn interface.
func (c *conn) SetWriteDeadline(t time.Time) error {
	if timeout := getTimeout(t); timeout != nil {
		return syscall.SetsockoptTimeval(c.fd, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, timeout)
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
