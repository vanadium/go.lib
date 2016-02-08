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

var (
	// Make sure the signatures are right, so that io.Copy can be faster.
	_ io.WriterTo   = (*bufferedPipe)(nil)
	_ io.ReaderFrom = (*bufferedPipe)(nil)
)

// newBufferedPipe returns a new thread-safe pipe backed by an unbounded
// in-memory buffer. Writes on the pipe never block; reads on the pipe block
// until data is available.
func newBufferedPipe() io.ReadWriteCloser {
	return &bufferedPipe{cond: sync.NewCond(&sync.Mutex{})}
}

// Read reads from the pipe.
func (p *bufferedPipe) Read(d []byte) (int, error) {
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

// WriteTo implements the io.WriterTo method; it is the fast version of Read
// used by io.Copy.
// Unlike Read, which returns io.EOF to signal that all data has been read,
// WriteTo blocks until all data has been written to w, and never returns
// io.EOF.
func (p *bufferedPipe) WriteTo(w io.Writer) (int64, error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	var written int64
	for {
		// Keep writing data until the pipe is closed.
		n, err := p.buf.WriteTo(w)
		written += n
		if p.closed || err != nil {
			return written, err
		}
		p.cond.Wait()
	}
}

// Write writes to the pipe.
func (p *bufferedPipe) Write(d []byte) (int, error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	if p.closed {
		return 0, io.ErrClosedPipe
	}
	defer p.cond.Signal()
	return p.buf.Write(d)
}

// ReadFrom implements the io.ReaderFrom method; it is the fast version of Write
// used by io.Copy.
func (p *bufferedPipe) ReadFrom(r io.Reader) (int64, error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	if p.closed {
		return 0, io.ErrClosedPipe
	}
	defer p.cond.Signal()
	return p.buf.ReadFrom(r)
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
