// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

type ringBuffer struct {
	buf   []byte
	start int
	len   int
}

// newRingBuffer returns a new fixed-size buffer that holds the last 'capacity'
// bytes written.
func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{buf: make([]byte, capacity)}
}

// Append writes to the buffer.
func (b *ringBuffer) Append(p []byte) {
	if len(b.buf) == 0 {
		return
	}
	if len(p) >= len(b.buf) {
		copy(b.buf, p[len(p)-len(b.buf):])
		b.start = 0
		b.len = len(b.buf)
		return
	}
	// Copy p into b.buf.
	end := (b.start + b.len) % len(b.buf)
	n := copy(b.buf[end:], p)
	if n < len(p) {
		copy(b.buf, p[n:])
	}
	// Update b.start and b.len.
	b.len += len(p)
	if b.len > len(b.buf) {
		b.start = (b.start + b.len) % len(b.buf)
		b.len = len(b.buf)
	}
}

// String returns the buffer as a string.
func (b *ringBuffer) String() string {
	if b.start == 0 {
		return string(b.buf[:b.len])
	}
	// INVARIANT: If b.start > 0, b.len == len(b.buf).
	return string(b.buf[b.start:]) + string(b.buf[:b.start])
}
