// +build !linux android

package bluetooth

import (
	"errors"
	"net"
)

// Dial always returns an error since bluetooth RFCOMM is not implemented on this
// platform.
func Dial(remoteaddr string) (net.Conn, error) {
	return nil, errors.New("bluetooth Dialing not implemented on this platform")
}

// Listen always returns an error since bluetooth RFCOMM is not implemented on
// this platform.
func Listen(localaddr string) (net.Listener, error) {
	return nil, errors.New("bluetooth Listening not implemented on this platform")
}
