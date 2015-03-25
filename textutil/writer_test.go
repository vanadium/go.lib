// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textutil

import (
	"bytes"
	"fmt"
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
		{"PRE", nil, ""},
		{"PRE", []string{""}, ""},
		{"PRE", []string{"a"}, "PREa"},
		{"PRE", []string{"a", ""}, "PREa"},
		{"PRE", []string{"", "a"}, "PREa"},
		{"PRE", []string{"a", "b"}, "PREab"},
	}
	for _, test := range tests {
		var buf bytes.Buffer
		w := PrefixWriter(&buf, test.Prefix)
		for _, write := range test.Writes {
			name := fmt.Sprintf("(%v, %v)", test.Want, write)
			n, err := w.Write([]byte(write))
			if got, want := n, len(write); got != want {
				t.Errorf("%s got len %d, want %d", name, got, want)
			}
			if err != nil {
				t.Errorf("%s got error: %v", name, err)
			}
		}
		if got, want := buf.String(), test.Want; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
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
		for _, write := range test.Writes {
			name := fmt.Sprintf("(%v, %v, %v, %v)", test.Old, test.New, test.Want, write)
			n, err := w.Write([]byte(write))
			if got, want := n, len(write); got != want {
				t.Errorf("%s got len %d, want %d", name, got, want)
			}
			if err != nil {
				t.Errorf("%s got error: %v", name, err)
			}
		}
		if got, want := buf.String(), test.Want; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}
