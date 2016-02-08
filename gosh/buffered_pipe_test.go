// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"testing"
)

func TestReadWriteAfterClose(t *testing.T) {
	p := newBufferedPipe()
	if n, err := p.Write([]byte("foo")); n != 3 || err != nil {
		t.Errorf("write got (%v,%v) want (3,nil)", n, err)
	}
	if n, err := p.Write([]byte("barbaz")); n != 6 || err != nil {
		t.Errorf("write got (%v,%v) want (6,nil)", n, err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}
	// Read after close returns all data terminated by EOF.
	if b, err := ioutil.ReadAll(p); string(b) != "foobarbaz" || err != nil {
		t.Errorf("read got (%s,%v) want (foobarbaz,nil)", b, err)
	}
	// Write after close fails.
	n, err := p.Write([]byte("already closed"))
	if got, want := n, 0; got != want {
		t.Errorf("write after close got n %v, want %v", got, want)
	}
	if got, want := err, io.ErrClosedPipe; got != want {
		t.Errorf("write after close got error %v, want %v", got, want)
	}
}

func TestReadFromWriteTo(t *testing.T) {
	p, buf := newBufferedPipe(), new(bytes.Buffer)
	if n, err := p.(io.ReaderFrom).ReadFrom(strings.NewReader("foobarbaz")); n != 9 || err != nil {
		t.Errorf("readfrom got (%v,%v) want (9,nil)", n, err)
	}
	if n, err := p.(io.WriterTo).WriteTo(buf); n != 9 || err != nil {
		t.Errorf("writeto got (%v,%v) want (9,nil)", n, err)
	}
	if got, want := buf.String(), "foobarbaz"; got != want {
		t.Errorf("writeto got %v want %v", got, want)
	}
	buf.Reset()
	if n, err := p.(io.ReaderFrom).ReadFrom(strings.NewReader("foobarbaz")); n != 9 || err != nil {
		t.Errorf("readfrom got (%v,%v) want (9,nil)", n, err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}
	if n, err := p.(io.WriterTo).WriteTo(buf); n != 9 || err != nil {
		t.Errorf("writeto got (%v,%v) want (9,nil)", n, err)
	}
}
