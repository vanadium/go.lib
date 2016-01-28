// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

import (
	"bytes"
	"io"
	"sync"
)

type bufferedPipe struct {
	cond   *sync.Cond
	buf    bytes.Buffer
	closed bool
}

// newBufferedPipe returns a new thread-safe pipe backed by an unbounded
// in-memory buffer. Writes on the pipe never block; reads on the pipe block
// until data is available.
func newBufferedPipe() io.ReadWriteCloser {
	return &bufferedPipe{cond: sync.NewCond(&sync.Mutex{})}
}

// Read reads from the pipe.
func (p *bufferedPipe) Read(d []byte) (n int, err error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	for {
		// Read any remaining data before checking whether the pipe is closed.
		if p.buf.Len() > 0 {
			return p.buf.Read(d)
		}
		if p.closed {
			return 0, io.EOF
		}
		p.cond.Wait()
	}
}

// Write writes to the pipe.
func (p *bufferedPipe) Write(d []byte) (n int, err error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	if p.closed {
		return 0, io.ErrClosedPipe
	}
	defer p.cond.Signal()
	return p.buf.Write(d)
}

// Close closes the pipe.
func (p *bufferedPipe) Close() error {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	if !p.closed {
		defer p.cond.Signal()
		p.closed = true
	}
	return nil
}
