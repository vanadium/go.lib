package bluetooth

import (
	"net"
	"time"

	"veyron/lib/unit"
)

// Network string for net.Addr implementations used by the bluetooth
// package.
const Network = "bluetooth"

// ScanReading holds a single reading of a Low-Energy scan on the Bluetooth device.
type ScanReading struct {
	// Name represents a local name of the remote device.  It can also store
	// arbitrary application-specific data.
	Name string
	// MAC is the hardware address of the remote device.
	MAC net.HardwareAddr
	// Distance represents the (power-estimated) distance to the remote device.
	Distance unit.Distance
	// Time is the time the advertisement packed was received/scanned.
	Time time.Time
}
