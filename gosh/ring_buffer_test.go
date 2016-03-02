// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

import (
	"testing"
)

func TestRingBufferBasic(t *testing.T) {
	b := newRingBuffer(5)
	if got, want := b.String(), ""; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	b.Append([]byte("foo"))
	if got, want := b.String(), "foo"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	b.Append([]byte("bar"))
	if got, want := b.String(), "oobar"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	// Append an empty string.
	b.Append([]byte(""))
	if got, want := b.String(), "oobar"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// This time, appending a string that puts us right at the cap.
	b = newRingBuffer(3)
	b.Append([]byte("foo"))
	if got, want := b.String(), "foo"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	b.Append([]byte("bar"))
	if got, want := b.String(), "bar"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// This time, appending a string that's much bigger than the buffer.
	b = newRingBuffer(2)
	b.Append([]byte("012345678"))
	if got, want := b.String(), "78"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	b.Append([]byte("0123456789"))
	if got, want := b.String(), "89"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	b.Append([]byte("0"))
	if got, want := b.String(), "90"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// This time, with a size-1 buffer.
	b = newRingBuffer(1)
	b.Append([]byte("f"))
	if got, want := b.String(), "f"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	b.Append([]byte("o"))
	if got, want := b.String(), "o"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	b.Append([]byte("bar"))
	if got, want := b.String(), "r"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// This time, with a size-0 buffer.
	b = newRingBuffer(0)
	b.Append([]byte("f"))
	if got, want := b.String(), ""; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRingBufferCopiesBytes(t *testing.T) {
	foo, bar := []byte("foo"), []byte("bar")
	b := newRingBuffer(5)
	b.Append(foo)
	foo[2] = 'z'
	if got, want := b.String(), "foo"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	b.Append(bar)
	bar[2] = 'z'
	if got, want := b.String(), "oobar"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// This time, appending a string that puts us right at the cap.
	foo, bar = []byte("foo"), []byte("bar")
	b = newRingBuffer(3)
	b.Append(foo)
	foo[2] = 'z'
	if got, want := b.String(), "foo"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	b.Append(bar)
	bar[2] = 'z'
	if got, want := b.String(), "bar"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRingBufferStress(t *testing.T) {
	const s = "0123456789"
	for strLen := 0; strLen <= len(s); strLen++ {
		for bufCap := 0; bufCap <= 2*len(s); bufCap++ {
			b := newRingBuffer(bufCap)
			all := ""
			for i := 0; i < 2*len(s); i++ {
				b.Append([]byte(s[:strLen]))
				all += s[:strLen]
			}
			start := len(all) - bufCap
			if start < 0 {
				start = 0
			}
			if got, want := b.String(), all[start:]; got != want {
				t.Errorf("got %v, want %v", got, want)
			}
		}
	}
}
