// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netconfig_test

import (
	"sync"
	"testing"
	"time"

	"v.io/x/lib/netconfig"
	"v.io/x/lib/netconfig/osnetconfig"
)

func TestNotifyChange(t *testing.T) {
	ch, err := netconfig.NotifyChange()
	if err != nil {
		t.Fatal(err)
	}
	if ch == nil {
		t.Fatalf("Expected non-nil channel")
	}

	// NullNotifier is used by default.
	routes := netconfig.GetIPRoutes(true)
	if got, want := len(routes), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		netconfig.Shutdown()
		wg.Done()
	}()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Errorf("timeout")
	}
	wg.Wait()

	netconfig.SetOSNotifier(osnetconfig.NewNotifier(0))
	// Expect at least one route
	routes = netconfig.GetIPRoutes(true)
	if got, want := len(routes), 1; got < want {
		t.Errorf("got %v not less than %v", got, want)
	}
	netconfig.Shutdown()
}
