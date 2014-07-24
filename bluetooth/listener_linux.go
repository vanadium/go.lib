// +build veyronbluetooth,!android

package bluetooth

import (
	"fmt"
	"net"
	"unsafe"
)

// // Explicitly link libbluetooth and other libraries as "go build" cannot
// // figure out these dependencies..
// #cgo LDFLAGS: -lbluetooth
// #include <stdlib.h>
// #include <unistd.h>
// #include "bt_linux.h"
import "C"

// listener waits for incoming RFCOMM connections on the provided socket.
// It implements the net.Listener interface.
type listener struct {
	fd         *fd
	acceptChan chan (acceptResult)
	localAddr  net.Addr
}

type acceptResult struct {
	conn net.Conn
	err  error
}

func newListener(sockfd int, addr net.Addr) (net.Listener, error) {
	fd, err := newFD(sockfd)
	if err != nil {
		return nil, err
	}
	return &listener{fd: fd, acceptChan: make(chan acceptResult), localAddr: addr}, nil
}

// Implements the net.Listener interface.
func (l *listener) Accept() (net.Conn, error) {
	go l.fd.RunWhenReadable(l.accept)
	r := <-l.acceptChan
	return r.conn, r.err
}

func (l *listener) accept(sockfd int) {
	var fd C.int
	var remoteMAC *C.char
	var result acceptResult
	defer func() { l.acceptChan <- result }()
	if es := C.bt_accept(C.int(sockfd), &fd, &remoteMAC); es != nil {
		defer C.free(unsafe.Pointer(es))
		result.err = fmt.Errorf("error accepting connection on %s, socket: %d, error: %s", l.localAddr, sockfd, C.GoString(es))
		return
	}
	defer C.free(unsafe.Pointer(remoteMAC))

	// Parse remote address.
	var remote addr
	var err error
	if remote.mac, err = net.ParseMAC(C.GoString(remoteMAC)); err != nil {
		result.err = fmt.Errorf("invalid remote MAC address: %s, err: %s", C.GoString(remoteMAC), err)
		return
	}
	// There's no way to get accurate remote channel number, so use 0.
	remote.channel = 0
	result.conn, result.err = newConn(int(fd), l.localAddr, &remote)
}

// Implements the net.Listener interface.
func (l *listener) Close() error {
	return l.fd.Close()
}

// Implements the net.Listener interface.
func (l *listener) Addr() net.Addr {
	return l.localAddr
}
