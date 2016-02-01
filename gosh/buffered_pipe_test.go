// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

import (
	"io/ioutil"
	"testing"
)

func TestReadAfterClose(t *testing.T) {
	p := newBufferedPipe()
	if _, err := p.Write([]byte("foo")); err != nil {
		t.Errorf("write failed: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}
	b, err := ioutil.ReadAll(p)
	if err != nil {
		t.Errorf("read failed: %v", err)
	}
	if got, want := string(b), "foo"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
