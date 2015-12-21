// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh_test

import (
	"testing"

	"v.io/x/lib/gosh"
)

func TestReadAfterClose(t *testing.T) {
	p := gosh.NewBufferedPipe()
	p.Write([]byte("foo"))
	p.Close()
	eq(t, toString(t, p), "foo")
}
