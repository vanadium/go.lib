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

func (n *notifier) initLocked() error {
	go func() {
		ticker := time.Tick(2 * time.Minute)
		for range ticker {
			n.ding()
		}
	}()
	return nil
}

func GetIPRoutes(defaultOnly bool) []*IPRoute {
	// TODO(nlacasse,bprosnitz): Consider implementing? For now return
	// empty array, since that seems to keep things working.
	return []*IPRoute{}
}
