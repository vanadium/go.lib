// +build !veyronbluetooth !linux android

package bluetooth

import (
	"errors"
	"net"
)

// Device is a struct representing an opened Bluetooth device.
type Device struct{}

// Dial always returns an error since bluetooth is not yet supported
// on this platform.
func Dial(remoteaddr string) (net.Conn, error) {
	return nil, errors.New("bluetooth is not supported on this platform")
}

// Listen always returns an error since bluetooth is not yet supported
// on this platform.
func Listen(localaddr string) (net.Listener, error) {
	return nil, errors.New("bluetooth is not supported on this platform")
}

// OpenFirstAvailableDevice always returns an error since bluetooth is
// not yet supported on this platform.
func OpenFirstAvailableDevice() (*Device, error) {
	return nil, errors.New("bluetooth is not supported on this platform")
}
