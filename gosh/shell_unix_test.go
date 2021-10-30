// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows
// +build !windows

package gosh_test

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"v.io/x/lib/gosh"
)

func TestCleanupProcessGroup(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	c := sh.FuncCmd(processGroup, 5)
	c.Start()
	pids := c.AwaitVars("pids")["pids"]
	c.Signal(os.Interrupt)

	// Wait for all processes in the child's process group to exit.
	for syscall.Kill(-c.Pid(), 0) != syscall.ESRCH {
		time.Sleep(100 * time.Millisecond)
	}
	for _, pid := range strings.Split(pids, ",") {
		p, _ := strconv.Atoi(pid)
		eq(t, syscall.Kill(p, 0), syscall.ESRCH)
	}
}
