// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netconfig

import (
	"testing"
)

func TestNetConfigWatcherStop(t *testing.T) {
	w, err := NewNetConfigWatcher()
	if err != nil {
		t.Fatal(err)
	}
	w.Stop()
	// The channel should eventually be closed when the watcher exits.
	// If it doesn't close, then this test will run into a timeout.
	for range w.Channel() {
	}
}
