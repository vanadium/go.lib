// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package cmdline

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func writeFunc(s string) func(io.Writer) {
	return func(w io.Writer) { w.Write([]byte(s)) }
}

func TestEnvUsageErrorf(t *testing.T) {
	tests := []struct {
		format string
		args   []interface{}
		usage  func(io.Writer)
		want   string
	}{
		{"", nil, nil, "ERROR: \n\nusage error\n"},
		{"", nil, writeFunc("FooBar"), "ERROR: \n\nFooBar"},
		{"", nil, writeFunc("FooBar\n"), "ERROR: \n\nFooBar\n"},
		{"A%vB", []interface{}{"x"}, nil, "ERROR: AxB\n\nusage error\n"},
		{"A%vB", []interface{}{"x"}, writeFunc("FooBar"), "ERROR: AxB\n\nFooBar"},
		{"A%vB", []interface{}{"x"}, writeFunc("FooBar\n"), "ERROR: AxB\n\nFooBar\n"},
	}
	for _, test := range tests {
		var buf bytes.Buffer
		env := &Env{Stderr: &buf, Usage: test.usage}
		if got, want := env.UsageErrorf(test.format, test.args...), ErrUsage; got != want {
			t.Errorf("%q got error %v, want %v", test.want, got, want)
		}
		if got, want := buf.String(), test.want; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestEnvWidth(t *testing.T) {
	tests := []struct {
		env  string
		want int
	}{
		{"123", 123},
		{"-1", -1},
		{"0", defaultWidth},
		{"", defaultWidth},
		{"foobar", defaultWidth},
	}
	for _, test := range tests {
		if err := os.Setenv("CMDLINE_WIDTH", test.env); err != nil {
			t.Errorf("Setenv(%q) failed: %v", test.env, err)
			continue
		}
		if got, want := NewEnv().width(), test.want; got != want {
			t.Errorf("%q got %v, want %v", test.env, got, want)
		}
	}
}

func TestEnvStyle(t *testing.T) {
	tests := []struct {
		env  string
		want style
	}{
		{"compact", styleCompact},
		{"full", styleFull},
		{"godoc", styleGoDoc},
		{"", styleCompact},
		{"abc", styleCompact},
		{"foobar", styleCompact},
	}
	for _, test := range tests {
		if err := os.Setenv("CMDLINE_STYLE", test.env); err != nil {
			t.Errorf("Setenv(%q) failed: %v", test.env, err)
			continue
		}
		if got, want := NewEnv().style(), test.want; got != want {
			t.Errorf("%q got %v, want %v", test.env, got, want)
		}
	}
}
