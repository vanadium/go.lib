// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

type bufferedPipe struct {
	cond *sync.Cond
	buf  bytes.Buffer
	err  error
}

// NewBufferedPipe returns a new pipe backed by an unbounded in-memory buffer.
// Writes on the pipe never block; reads on the pipe block until data is
// available.
func NewBufferedPipe() io.ReadWriteCloser {
	return &bufferedPipe{cond: sync.NewCond(&sync.Mutex{})}
}

// Read reads from the pipe.
func (p *bufferedPipe) Read(d []byte) (n int, err error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	for {
		if p.buf.Len() > 0 {
			return p.buf.Read(d)
		}
		if p.err != nil {
			return 0, p.err
		}
		p.cond.Wait()
	}
}

// Write writes to the pipe.
func (p *bufferedPipe) Write(d []byte) (n int, err error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	if p.err != nil {
		return 0, errors.New("write on closed pipe")
	}
	defer p.cond.Signal()
	return p.buf.Write(d)
}

// Close closes the pipe.
func (p *bufferedPipe) Close() error {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	if p.err == nil {
		defer p.cond.Signal()
		p.err = io.EOF
	}
	return nil
}
