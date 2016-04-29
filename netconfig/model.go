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
	"time"
)

var globalNotifier notifier

// NotifyChange returns a channel that will be closed when the network
// configuration changes from the time this function was invoked.
//
// This may provide false positivies, i.e., a network change
// will cause the channel to be closed but a channel closure
// may not imply a network change.
func NotifyChange() (<-chan struct{}, error) {
	return globalNotifier.add()
}

// IPRoute represents a route in the kernel's routing table.
// Any route with a nil Gateway is a directly connected network.
type IPRoute struct {
	Net             net.IPNet
	Gateway         net.IP
	PreferredSource net.IP
	IfcIndex        int
}

type notifier struct {
	sync.Mutex
	ch    chan struct{}
	timer *time.Timer

	initErr error
	inited  bool
}

func (n *notifier) add() (<-chan struct{}, error) {
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

func (n *notifier) ding() {
	// Changing networks usually spans many seconds and involves
	// multiple network config changes.  We add histeresis by
	// setting an alarm when the first change is detected and
	// not informing the client till the alarm goes off.
	// NOTE(p): I chose 3 seconds because that covers all the
	// events involved in moving from one wifi network to another.
	n.Lock()
	if n.timer == nil {
		n.timer = time.AfterFunc(3*time.Second, n.resetChan)
	}
	n.Unlock()
}

func (n *notifier) resetChan() {
	n.Lock()
	close(n.ch)
	n.ch = make(chan struct{})
	n.timer = nil
	n.Unlock()
}
