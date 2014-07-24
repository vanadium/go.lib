// +build !veyronbluetooth !linux android

package bluetooth

import (
	"errors"
	"net"
	"time"
)

var errNotSupported = errors.New("bluetooth is not supported on this platform")

func (d *Device) StartScan(scanInterval, scanWindow time.Duration) (<-chan ScanReading, error) {
	return nil, errNotSupported
}
func (d *Device) StopScan() error                                  { return errNotSupported }
func (d *Device) Close() error                                     { return errNotSupported }
func (d *Device) StartAdvertising(advInterval time.Duration) error { return errNotSupported }
func (d *Device) SetAdvertisingPayload(payload string) error       { return errNotSupported }
func (d *Device) StopAdvertising() error                           { return errNotSupported }

// Dial always returns an error since bluetooth is not yet supported
// on this platform.
func Dial(remoteaddr string) (net.Conn, error) {
	return nil, errNotSupported
}

// Listen always returns an error since bluetooth is not yet supported
// on this platform.
func Listen(localaddr string) (net.Listener, error) {
	return nil, errNotSupported
}

// OpenFirstAvailableDevice always returns an error since bluetooth is
// not yet supported on this platform.
func OpenFirstAvailableDevice() (*Device, error) {
	return nil, errNotSupported
}
