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
)

var (
	mu             sync.Mutex
	globalNotifier OSNotifier
)

// IPRoute represents a route in the kernel's routing table.
// Any route with a nil Gateway is a directly connected network.
type IPRoute struct {
	Net             net.IPNet
	Gateway         net.IP
	PreferredSource net.IP
	IfcIndex        int
}

// NotifyChange returns a channel that will be closed when the network
// configuration changes from the time this function was invoked. If
// SetOSNotifier has not been called then the channel returned will never
// be closed since no network changes will ever be detected.
func NotifyChange() (<-chan struct{}, error) {
	if globalNotifier == nil {
		// this channel will never be closed.
		return make(chan struct{}), nil
	}
	return globalNotifier.NotifyChange()
}

// GetIPRoutes returns all kernel known routes. If defaultOnly is set, only
// default routes are returned. If SetOSNotifier has not been called then
// then an empty set of routes will be returned.
func GetIPRoutes(defaultOnly bool) []*IPRoute {
	if globalNotifier == nil {
		return []*IPRoute{}
	}
	return globalNotifier.GetIPRoutes(defaultOnly)
}

// OSNotifier represents an os specific notifier of network configuration
// changes and for obtaining current network state.
type OSNotifier interface {
	// NotifyChange returns a channel that will be closed when the network
	// configuration changes from the time this function was invoked.
	//
	// This may provide false positivies, i.e., a network change
	// will cause the channel to be closed but a channel closure
	// may not imply a network change.
	NotifyChange() (<-chan struct{}, error)

	// GetIPRoutes returns all kernel known routes. If defaultOnly is set,
	// only default routes are returned.
	GetIPRoutes(defaultOnly bool) []*IPRoute
}

// SetOSNotifier sets the global OS specific notifier and route table accessor.
// This function may only be called once.
func SetOSNotifier(osn OSNotifier) {
	mu.Lock()
	defer mu.Unlock()
	if globalNotifier != nil {
		panic("The global OS notifier for network state changes has already been set")
	}
	globalNotifier = osn
}
