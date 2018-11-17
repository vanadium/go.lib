// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package netconfig implements a network configuration watcher.
package netconfig

// NOTE(p): This is also where we should put any code that changes
//          network configuration.

import (
	"net"
	"sync"

	"v.io/x/lib/netconfig/internal"
)

var (
	mu             sync.Mutex
	globalNotifier *internal.Notifier
)

// IPRoute represents a route in the kernel's routing table.
// Any route with a nil Gateway is a directly connected network.
type IPRoute struct {
	Net             net.IPNet
	Gateway         net.IP
	PreferredSource net.IP
	IfcIndex        int
}

func init() {
	globalNotifier = internal.NewNotifier(0)
}

// NotifyChange returns a channel that will be closed when the network
// configuration changes from the time this function was invoked. If
// SetOSNotifier has not been called then the channel returned will never
// be closed since no network changes will ever be detected.
func NotifyChange() (<-chan struct{}, error) {
	if globalNotifier == nil {
		panic("globalNotifier is not set")
	}
	return globalNotifier.Add()
}

// GetIPRoutes returns all kernel known routes. If defaultOnly is set, only
// default routes are returned. If SetOSNotifier has not been called then
// then an empty set of routes will be returned.
func GetIPRoutes(defaultOnly bool) []*IPRoute {
	ir := internal.GetIPRoutes(defaultOnly)
	r := make([]*IPRoute, len(ir))
	for i, c := range ir {
		n := new(IPRoute)
		n.Net = c.Net
		n.Gateway = c.Gateway
		n.PreferredSource = c.PreferredSource
		n.IfcIndex = c.IfcIndex
		r[i] = n
	}
	return r
}
