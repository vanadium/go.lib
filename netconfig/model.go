// package netconfig implements a network configuration watcher.
// NOTE(p): This is also where we should put any code that changes
//          network configuration.

package netconfig

import (
	"net"
)

// NetConfigWatcher sends on channel whenever an interface or interface address
// is added or deleted.
type NetConfigWatcher interface {
	// Stop watching.
	Stop()

	// A channel that returns an item whenever the network addresses or
	// interfaces have changed. It is up to the caller to reread the
	// network configuration in such cases.
	Channel() chan struct{}
}

// IPRoute represents a route in the kernel's routing table.
// Any route with a nil Gateway is a directly connected network.
type IPRoute struct {
	Net             net.IPNet
	Gateway         net.IP
	PreferredSource net.IP
	IfcIndex        int
}
