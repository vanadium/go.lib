// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !linux,!darwin,!dragonfly,!freebsd,!netbsd,!openbsd
// TODO(bprosnitz) Should change for nacl?

package netconfig

// Code to signal a network change every 2 minutes.   We use
// this for systems where we don't yet have a good way to
// watch for network changes.

import (
	"time"
)

type timerNetConfigWatcher struct {
	c    chan struct{} // channel to signal confg changes
	stop chan struct{} // channel to tell the watcher to stop
}

func (w *timerNetConfigWatcher) Stop() {
	w.stop <- struct{}{}
}

func (w *timerNetConfigWatcher) Channel() <-chan struct{} {
	return w.c
}

func (w *timerNetConfigWatcher) watcher() {
	for {
		select {
		case <-w.stop:
			close(w.c)
			return
		case <-time.NewTimer(2 * time.Minute).C:
			select {
			case w.c <- struct{}{}:
			default:
			}
		}
	}
}

func NewNetConfigWatcher() (NetConfigWatcher, error) {
	w := &timerNetConfigWatcher{}
	w.c = make(chan struct{})
	w.stop = make(chan struct{}, 1)
	go w.watcher()
	return w, nil
}

func GetIPRoutes(defaultOnly bool) []*IPRoute {
	// TODO(nlacasse,bprosnitz): Consider implementing? For now return
	// empty array, since that seems to keep things working.
	return []*IPRoute{}
}
