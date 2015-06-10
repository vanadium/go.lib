// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vlog_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"v.io/x/lib/vlog"

	"v.io/x/ref/test/modules"
)

//go:generate v23 test generate

var child = modules.Register(func(env *modules.Env, args ...string) error {
	tmp := filepath.Join(os.TempDir(), "foo")
	flag.Set("log_dir", tmp)
	flag.Set("vmodule", "foo=2")
	flags := vlog.Log.ExplicitlySetFlags()
	if v, ok := flags["log_dir"]; !ok || v != tmp {
		return fmt.Errorf("log_dir was supposed to be %v", tmp)
	}
	if v, ok := flags["vmodule"]; !ok || v != "foo=2" {
		return fmt.Errorf("vmodule was supposed to be foo=2")
	}
	if f := flag.Lookup("max_stack_buf_size"); f == nil {
		return fmt.Errorf("max_stack_buf_size is not a flag")
	}
	maxStackBufSizeSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "max_stack_buf_size" {
			maxStackBufSizeSet = true
		}
	})
	if v, ok := flags["max_stack_buf_size"]; ok && !maxStackBufSizeSet {
		return fmt.Errorf("max_stack_buf_size unexpectedly set to %v", v)
	}
	return nil
}, "child")

func TestFlags(t *testing.T) {
	sh, err := modules.NewShell(nil, nil, testing.Verbose(), t)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer sh.Cleanup(nil, nil)
	h, err := sh.Start(nil, child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err = h.Shutdown(nil, os.Stderr); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
