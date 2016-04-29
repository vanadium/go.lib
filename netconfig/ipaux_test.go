// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netconfig

import (
	"testing"
)

func TestNotifyChange(t *testing.T) {
	ch, err := NotifyChange()
	if err != nil {
		t.Fatal(err)
	}
	if ch == nil {
		t.Fatalf("Expected non-nil channel")
	}
}
