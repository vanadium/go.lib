// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package netconfig implements a network configuration watcher.
package netconfig

// NOTE(p): This is also where we should put any code that changes
//          network configuration.

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
