// +build linux

package bluetooth

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

// // Explicitly link libbluetooth and other libraries as "go build" cannot
// // figure out these dependencies..
// #cgo LDFLAGS: -lbluetooth
// #include <stdlib.h>
// #include <unistd.h>
// #include "bt.h"
import "C"

// listener waits for incoming RFCOMM connections on the provided socket.
// It implements the net.Listener interface.
type listener struct {
	localAddr *addr
	socket    int
}

// Implements the net.Listener interface.
func (l *listener) Accept() (net.Conn, error) {
	var fd C.int
	var remoteMAC *C.char
	if es := C.bt_accept(C.int(l.socket), &fd, &remoteMAC); es != nil {
		defer C.free(unsafe.Pointer(es))
		return nil, fmt.Errorf("error accepting connection on %s, socket: %d, error: %s", l.localAddr, l.socket, C.GoString(es))
	}
	defer C.free(unsafe.Pointer(remoteMAC))

	// Parse remote address.
	var remote addr
	var err error
	if remote.mac, err = net.ParseMAC(C.GoString(remoteMAC)); err != nil {
		return nil, fmt.Errorf("invalid remote MAC address: %s, err: %s", C.GoString(remoteMAC), err)
	}
	// There's no way to get accurate remote channel number, so use 0.
	remote.channel = 0

	return &conn{
		fd:         int(fd),
		localAddr:  l.localAddr,
		remoteAddr: &remote,
	}, nil
}

// Implements the net.Listener interface.
func (l *listener) Close() error {
	return syscall.Close(l.socket)
}

// Implements the net.Listener interface.
func (l *listener) Addr() net.Addr {
	return l.localAddr
}
