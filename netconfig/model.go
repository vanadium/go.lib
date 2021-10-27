// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package netconfig implements a network configuration watcher, or more
// accurately an interface to a network configuration watcher. The OS specific
// implementation implements the Notifier interface below and must be
// set via the SetNotifier method. No auto-registration mechanism is provided
// since the OS specific code may use CGO and some applications may prefer
// to avoid the use of cgo in order to allow for simple cross compilation.
package netconfig

// NOTE(p): This is also where we should put any code that changes
//          network configuration.

import (
	"sync"

	"v.io/x/lib/netconfig/route"
)

var (
	mu             sync.Mutex
	globalNotifier Notifier
)

func init() {
	globalNotifier = &NullNotifier{}
}

// Notifier represents a notifier of network configuration changes and for
// obtaining current network state.
type Notifier interface {
	// NotifyChange returns a channel that will be closed when the network
	// configuration changes from the time this function was invoked.
	//
	// This may provide false positivies, i.e., a network change
	// will cause the channel to be closed but a channel closure
	// may not imply a network change.
	NotifyChange() (<-chan struct{}, error)

	// GetIPRoutes returns all kernel known routes. If defaultOnly is set,
	// only default routes are returned.
	GetIPRoutes(defaultOnly bool) []route.IPRoute

	// Shutdown will shutdown the notifier and close the channel returned
	// by NotifyChange.
	Shutdown()
}

// NullNotifier represents a null implementation of Notifier that will
// never return any notifications or routes. It is provided as a default.
type NullNotifier struct {
	mu          sync.Mutex
	initialized bool
	ch          chan struct{}
}

// NotifyChange implements Notifier.
func (n *NullNotifier) NotifyChange() (<-chan struct{}, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.initialized {
		return n.ch, nil
	}
	n.ch = make(chan struct{})
	return n.ch, nil
}

// GetIPRoutes implements Notifier.
func (n *NullNotifier) GetIPRoutes(defaultOnly bool) []route.IPRoute {
	return nil
}

// Shutdown implements Notifier.
func (n *NullNotifier) Shutdown() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.ch != nil {
		close(n.ch)
	}
	n.ch = nil
}

// NotifyChange returns a channel that will be closed when the network
// configuration changes from the time this function was invoked. If
// SetOSNotifier has not been called then the channel returned will never
// be closed since no network changes will ever be detected.
func NotifyChange() (<-chan struct{}, error) {
	return globalNotifier.NotifyChange()
}

// Shutdown shutdowns the current notifier.
func Shutdown() {
	globalNotifier.Shutdown()
}

// GetIPRoutes returns all kernel known routes. If defaultOnly is set, only
// default routes are returned. If SetOSNotifier has not been called then
// then an empty set of routes will be returned.
func GetIPRoutes(defaultOnly bool) []route.IPRoute {
	return globalNotifier.GetIPRoutes(defaultOnly)
}

// SetOSNotifier sets a the internal notifier to the one supplied. An
// existing Notifier will be shutdown.
func SetOSNotifier(n Notifier) {
	mu.Lock()
	defer mu.Unlock()
	if globalNotifier != nil {
		globalNotifier.Shutdown()
	}
	globalNotifier = n
}
