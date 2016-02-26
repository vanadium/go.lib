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

	"v.io/x/lib/gosh"
	"v.io/x/lib/vlog"
)

var child = gosh.RegisterFunc("child", func() error {
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
})

func TestFlags(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()
	sh.FuncCmd(child).Run()
}

func TestMain(m *testing.M) {
	gosh.InitMain()
	os.Exit(m.Run())
}
