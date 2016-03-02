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

func TestBufferedPipeReadWriteAfterClose(t *testing.T) {
	p := newBufferedPipe()
	if n, err := p.Write([]byte("foo")); n != 3 || err != nil {
		t.Errorf("write got (%v, %v), want (3, <nil>)", n, err)
	}
	if n, err := p.Write([]byte("barbaz")); n != 6 || err != nil {
		t.Errorf("write got (%v, %v), want (6, <nil>)", n, err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}
	// Read after close returns all data terminated by EOF.
	if b, err := ioutil.ReadAll(p); string(b) != "foobarbaz" || err != nil {
		t.Errorf("read got (%s, %v), want (foobarbaz, <nil>)", b, err)
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

func TestBufferedPipeReadFromWriteTo(t *testing.T) {
	p, buf := newBufferedPipe(), new(bytes.Buffer)
	if n, err := p.(io.ReaderFrom).ReadFrom(strings.NewReader("foobarbaz")); n != 9 || err != nil {
		t.Errorf("ReadFrom got (%v, %v), want (9, <nil>)", n, err)
	}
	nCh, errCh := make(chan int64, 1), make(chan error, 1)
	go func() {
		n, err := p.(io.WriterTo).WriteTo(buf)
		nCh <- n
		errCh <- err
	}()
	if n, err := p.(io.ReaderFrom).ReadFrom(strings.NewReader("foobarbaz")); n != 9 || err != nil {
		t.Errorf("ReadFrom got (%v, %v), want (9, <nil>)", n, err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
	if n, err := <-nCh, <-errCh; n != 18 || err != nil {
		t.Errorf("WriteTo got (%v, %v), want (18, <nil>)", n, err)
	}
	if got, want := buf.String(), "foobarbazfoobarbaz"; got != want {
		t.Errorf("WriteTo got %v, want %v", got, want)
	}
}

func TestBufferedPipeWriteToMany(t *testing.T) {
	p := newBufferedPipe()
	pR, pW := io.Pipe()
	nCh, errCh := make(chan int64, 1), make(chan error, 1)
	go func() {
		n, err := p.(io.WriterTo).WriteTo(pW)
		nCh <- n
		errCh <- err
	}()
	var nTotal int64
	for _, m := range []string{
		"mary had",
		"a little lamb",
		"three helpings of corn",
		"two baked potatoes",
		"and extra bread",
	} {
		if n, err := p.Write([]byte(m)); n != len(m) || err != nil {
			t.Errorf("Write(%v) got (%v, %v), want (%v, <nil>)", m, n, err, len(m))
		}

		nTotal += int64(len(m))
		for i := 0; i < len(m); i++ {
			b := make([]byte, 1)
			if n, err := pR.Read(b); n != 1 || err != nil || b[0] != m[i] {
				t.Errorf("Read got (%v, %v, %v), want (1, <nil>, %v)", n, err, b[0], m[i])
			}
		}
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
	if n, err := <-nCh, <-errCh; n != nTotal || err != nil {
		t.Errorf("WriteTo got (%v, %v), want (%v, <nil>)", n, err, nTotal)
	}
}
