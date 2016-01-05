// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textutil

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPrefixWriter(t *testing.T) {
	tests := []struct {
		Prefix string
		Writes []string
		Want   string
	}{
		{"", nil, ""},
		{"", []string{""}, ""},
		{"", []string{"a"}, "a"},
		{"", []string{"a", ""}, "a"},
		{"", []string{"", "a"}, "a"},
		{"", []string{"a", "b"}, "ab"},
		{"", []string{"ab"}, "ab"},
		{"", []string{"\n"}, "\n"},
		{"", []string{"\n", ""}, "\n"},
		{"", []string{"", "\n"}, "\n"},
		{"", []string{"a", "\n"}, "a\n"},
		{"", []string{"a\n"}, "a\n"},
		{"", []string{"\n", "a"}, "\na"},
		{"", []string{"\na"}, "\na"},
		{"", []string{"a\nb\nc"}, "a\nb\nc"},
		{"PRE", nil, ""},
		{"PRE", []string{""}, ""},
		{"PRE", []string{"a"}, "PREa"},
		{"PRE", []string{"a", ""}, "PREa"},
		{"PRE", []string{"", "a"}, "PREa"},
		{"PRE", []string{"a", "b"}, "PREab"},
		{"PRE", []string{"ab"}, "PREab"},
		{"PRE", []string{"\n"}, "PRE\n"},
		{"PRE", []string{"\n", ""}, "PRE\n"},
		{"PRE", []string{"", "\n"}, "PRE\n"},
		{"PRE", []string{"a", "\n"}, "PREa\n"},
		{"PRE", []string{"a\n"}, "PREa\n"},
		{"PRE", []string{"\n", "a"}, "PRE\na"},
		{"PRE", []string{"\na"}, "PRE\na"},
		{"PRE", []string{"a", "\n", "b", "\n", "c"}, "PREa\nb\nc"},
		{"PRE", []string{"a\nb\nc"}, "PREa\nb\nc"},
		{"PRE", []string{"a\nb\nc\n"}, "PREa\nb\nc\n"},
	}
	for _, test := range tests {
		var buf bytes.Buffer
		w := PrefixWriter(&buf, test.Prefix)
		name := fmt.Sprintf("(%q, %q)", test.Want, test.Writes)
		for _, write := range test.Writes {
			name := name + fmt.Sprintf("(%q)", write)
			n, err := w.Write([]byte(write))
			if got, want := n, len(write); got != want {
				t.Errorf("%s got len %d, want %d", name, got, want)
			}
			if err != nil {
				t.Errorf("%s got error: %v", name, err)
			}
		}
		if got, want := buf.String(), test.Want; got != want {
			t.Errorf("%s got %q, want %q", name, got, want)
		}
	}
}

func TestPrefixLineWriter(t *testing.T) {
	tests := []struct {
		Prefix string
		Writes []string
		Want   string
	}{
		{"", nil, ""},
		{"", []string{""}, ""},
		{"", []string{"a"}, "a"},
		{"", []string{"a", ""}, "a"},
		{"", []string{"", "a"}, "a"},
		{"", []string{"a", "b"}, "ab"},
		{"", []string{"ab"}, "ab"},
		{"", []string{"\n"}, "\n"},
		{"", []string{"\n", ""}, "\n"},
		{"", []string{"", "\n"}, "\n"},
		{"", []string{"a", "\n"}, "a\n"},
		{"", []string{"a\n"}, "a\n"},
		{"", []string{"\n", "a"}, "\na"},
		{"", []string{"\na"}, "\na"},
		{"", []string{"a\nb\nc"}, "a\nb\nc"},
		{"PRE", nil, ""},
		{"PRE", []string{""}, ""},
		{"PRE", []string{"a"}, "PREa"},
		{"PRE", []string{"a", ""}, "PREa"},
		{"PRE", []string{"", "a"}, "PREa"},
		{"PRE", []string{"a", "b"}, "PREab"},
		{"PRE", []string{"ab"}, "PREab"},
		{"PRE", []string{"\n"}, "PRE\n"},
		{"PRE", []string{"\n", ""}, "PRE\n"},
		{"PRE", []string{"", "\n"}, "PRE\n"},
		{"PRE", []string{"a", "\n"}, "PREa\n"},
		{"PRE", []string{"a\n"}, "PREa\n"},
		{"PRE", []string{"\n", "a"}, "PRE\nPREa"},
		{"PRE", []string{"\na"}, "PRE\nPREa"},
		{"PRE", []string{"a", "\n", "b", "\n", "c"}, "PREa\nPREb\nPREc"},
		{"PRE", []string{"a\nb\nc"}, "PREa\nPREb\nPREc"},
		{"PRE", []string{"a\nb\nc\n"}, "PREa\nPREb\nPREc\n"},
	}
	for _, test := range tests {
		for _, eol := range eolRunesAsString {
			// Replace \n in Want and Writes with the test eol rune.
			want := strings.Replace(test.Want, "\n", string(eol), -1)
			var writes []string
			for _, write := range test.Writes {
				writes = append(writes, strings.Replace(write, "\n", string(eol), -1))
			}
			// Run the actual tests.
			var buf bytes.Buffer
			w := PrefixLineWriter(&buf, test.Prefix)
			name := fmt.Sprintf("(%q, %q)", want, writes)
			for _, write := range writes {
				name := name + fmt.Sprintf("(%q)", write)
				n, err := w.Write([]byte(write))
				if got, want := n, len(write); got != want {
					t.Errorf("%s got len %d, want %d", name, got, want)
				}
				if err != nil {
					t.Errorf("%s got error: %v", name, err)
				}
			}
			if err := w.Flush(); err != nil {
				t.Errorf("%s Flush got error: %v", name, err)
			}
			if got, want := buf.String(), want; got != want {
				t.Errorf("%s got %q, want %q", name, got, want)
			}
		}
	}
}

var (
	err1 = errors.New("error 1")
	err2 = errors.New("error 2")
)

type fakeWriteFlusher struct {
	writeErr error
	flushErr error
	flushed  bool
}

func (f *fakeWriteFlusher) Write(p []byte) (int, error) {
	return len(p), f.writeErr
}

func (f *fakeWriteFlusher) Flush() error {
	f.flushed = true
	return f.flushErr
}

func TestPrefixLineWriter_Flush(t *testing.T) {
	fake := &fakeWriteFlusher{}
	w := PrefixLineWriter(fake, "prefix")
	if err := w.Flush(); err != nil {
		t.Errorf("Flush got error %v, want nil", err)
	}
	if !fake.flushed {
		t.Errorf("Flush not propagated")
	}
}

func TestPrefixLineWriter_FlushError(t *testing.T) {
	fake := &fakeWriteFlusher{flushErr: err1}
	w := PrefixLineWriter(fake, "prefix")
	if err := w.Flush(); err != err1 {
		t.Errorf("Flush got error %v, want %v", err, err1)
	}
	if !fake.flushed {
		t.Errorf("Flush not propagated")
	}
}

func TestPrefixLineWriter_WriteFlush(t *testing.T) {
	fake := &fakeWriteFlusher{}
	w := PrefixLineWriter(fake, "prefix")
	if n, err := w.Write([]byte("abc")); n != 3 || err != nil {
		t.Errorf("Write got (%v,%v), want (3,nil)", n, err)
	}
	if err := w.Flush(); err != nil {
		t.Errorf("Flush got error %v, want nil", err)
	}
	if !fake.flushed {
		t.Errorf("Flush not propagated")
	}
}

func TestPrefixLineWriter_WriteFlushError(t *testing.T) {
	fake := &fakeWriteFlusher{flushErr: err1}
	w := PrefixLineWriter(fake, "prefix")
	if n, err := w.Write([]byte("abc")); n != 3 || err != nil {
		t.Errorf("Write got (%v,%v), want (3,nil)", n, err)
	}
	if err := w.Flush(); err != err1 {
		t.Errorf("Flush got error %v, want %v", err, err1)
	}
	if !fake.flushed {
		t.Errorf("Flush not propagated")
	}
}

func TestPrefixLineWriter_WriteErrorFlush(t *testing.T) {
	fake := &fakeWriteFlusher{writeErr: err1}
	w := PrefixLineWriter(fake, "prefix")
	if n, err := w.Write([]byte("abc")); n != 3 || err != nil {
		t.Errorf("Write got (%v,%v), want (3,nil)", n, err)
	}
	if err := w.Flush(); err != err1 {
		t.Errorf("Flush got error %v, want %v", err, err1)
	}
	if !fake.flushed {
		t.Errorf("Flush not propagated")
	}
}

func TestPrefixLineWriter_WriteErrorFlushError(t *testing.T) {
	fake := &fakeWriteFlusher{writeErr: err1, flushErr: err2}
	w := PrefixLineWriter(fake, "prefix")
	if n, err := w.Write([]byte("abc")); n != 3 || err != nil {
		t.Errorf("Write got (%v,%v), want (3,nil)", n, err)
	}
	if err := w.Flush(); err != err1 {
		t.Errorf("Flush got error %v, want %v", err, err1)
	}
	if !fake.flushed {
		t.Errorf("Flush not propagated")
	}
}

func TestPrefixLineWriter_EOLWriteErrorFlushError(t *testing.T) {
	fake := &fakeWriteFlusher{writeErr: err1, flushErr: err2}
	w := PrefixLineWriter(fake, "prefix")
	if n, err := w.Write([]byte("ab\n")); n != 0 || err != err1 {
		t.Errorf("Write got (%v,%v), want (0,%v)", n, err, err1)
	}
	if err := w.Flush(); err != err2 {
		t.Errorf("Flush got error %v, want %v", err, err2)
	}
	if !fake.flushed {
		t.Errorf("Flush not propagated")
	}
}

func TestByteReplaceWriter(t *testing.T) {
	tests := []struct {
		Old    byte
		New    string
		Writes []string
		Want   string
	}{
		{'a', "", nil, ""},
		{'a', "", []string{""}, ""},
		{'a', "", []string{"a"}, ""},
		{'a', "", []string{"b"}, "b"},
		{'a', "", []string{"aba"}, "b"},
		{'a', "", []string{"aba", "bab"}, "bbb"},
		{'a', "X", nil, ""},
		{'a', "X", []string{""}, ""},
		{'a', "X", []string{"a"}, "X"},
		{'a', "X", []string{"b"}, "b"},
		{'a', "X", []string{"aba"}, "XbX"},
		{'a', "X", []string{"aba", "bab"}, "XbXbXb"},
		{'a', "ZZZ", nil, ""},
		{'a', "ZZZ", []string{""}, ""},
		{'a', "ZZZ", []string{"a"}, "ZZZ"},
		{'a', "ZZZ", []string{"b"}, "b"},
		{'a', "ZZZ", []string{"aba"}, "ZZZbZZZ"},
		{'a', "ZZZ", []string{"aba", "bab"}, "ZZZbZZZbZZZb"},
	}
	for _, test := range tests {
		var buf bytes.Buffer
		w := ByteReplaceWriter(&buf, test.Old, test.New)
		name := fmt.Sprintf("(%q, %q, %q, %q)", test.Old, test.New, test.Want, test.Writes)
		for _, write := range test.Writes {
			name := name + fmt.Sprintf("(%q)", write)
			n, err := w.Write([]byte(write))
			if got, want := n, len(write); got != want {
				t.Errorf("%s got len %d, want %d", name, got, want)
			}
			if err != nil {
				t.Errorf("%s got error: %v", name, err)
			}
		}
		if got, want := buf.String(), test.Want; got != want {
			t.Errorf("%s got %q, want %q", name, got, want)
		}
	}
}
