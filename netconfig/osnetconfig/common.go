// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package osnetconfig provides OS specific routines for detecting network
// changes and reading the route table; it uses cgo to to do so on some systems.
package osnetconfig

import (
	"net"
	"sync"
	"time"
)

// IPRoute represents a route in the kernel's routing table.
// Any route with a nil Gateway is a directly connected network.
type IPRoute struct {
	Net             net.IPNet
	Gateway         net.IP
	PreferredSource net.IP
	IfcIndex        int
}

// NewNotifier returns a new network change Notifier.
func NewNotifier(delay time.Duration) *Notifier {
	if delay == 0 {
		// See ding method below.
		// NOTE(p): I chose 3 seconds because that covers all the
		// events involved in moving from one wifi network to another.
		delay = 3 * time.Second
	}
	return &Notifier{delay: delay}
}

// Notifier represents a new network change Notifier.
type Notifier struct {
	sync.Mutex
	ch    chan struct{}
	timer *time.Timer
	delay time.Duration
	stop  bool

	initErr error
	inited  bool
}

// NotifyChange implements netconfig.Notifier.
func (n *Notifier) NotifyChange() (<-chan struct{}, error) {
	n.Lock()
	defer n.Unlock()
	if !n.inited {
		n.ch = make(chan struct{})
		n.initErr = n.initLocked()
		n.inited = true
	}
	if n.initErr != nil {
		return nil, n.initErr
	}
	return n.ch, nil
}

// Shutdown implements netconfig.Notifier.
func (n *Notifier) Shutdown() {
	n.Lock()
	defer n.Unlock()

	n.stop = true
	if n.ch != nil {
		close(n.ch)
	}
}

func (n *Notifier) stopped() bool {
	n.Lock()
	defer n.Unlock()
	return n.stop
}

// ding returns true when the nofitifer is being shutdown.
func (n *Notifier) ding() bool {
	// Changing networks usually spans many seconds and involves
	// multiple network config changes.  We add histeresis by
	// setting an alarm when the first change is detected and
	// not informing the client till the alarm goes off.
	n.Lock()
	defer n.Unlock()
	if n.stop {
		close(n.ch)
		return true
	}
	if n.timer == nil {
		n.timer = time.AfterFunc(n.delay, n.resetChan)
	}
	return false
}

func (n *Notifier) resetChan() {
	n.Lock()
	close(n.ch)
	n.ch = make(chan struct{})
	n.timer = nil
	n.Unlock()
}
