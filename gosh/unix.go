// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin

package gosh

import (
	"syscall"
	"time"
)

// TODO(sadovsky): Maybe wrap every child process with a "supervisor" process
// that calls InitChildMain.

func (c *Cmd) start() (e error) {
	defer func() {
		// Always close afterStartClosers upon return. Only close afterWaitClosers
		// if start failed; if start succeeds, they're closed in the startExitWaiter
		// goroutine. Only the first error is reported.
		if err := closeClosers(c.afterStartClosers); e == nil {
			e = err
		}
		if !c.started {
			if err := closeClosers(c.afterWaitClosers); e == nil {
				e = err
			}
		}
	}()
	if c.calledStart {
		return errAlreadyCalledStart
	}
	c.calledStart = true
	// Protect against Cmd.start() writing to c.c.Process concurrently with
	// signal-triggered Shell.cleanup() reading from it.
	c.sh.cleanupMu.Lock()
	defer c.sh.cleanupMu.Unlock()
	if c.sh.calledCleanup {
		return errAlreadyCalledCleanup
	}
	// Configure the command.
	c.c.Path = c.Path
	vars := copyMap(c.Vars)
	if c.IgnoreParentExit {
		delete(vars, envWatchParent)
	} else {
		vars[envWatchParent] = "1"
	}
	if c.ExitAfter == 0 {
		delete(vars, envExitAfter)
	} else {
		vars[envExitAfter] = c.ExitAfter.String()
	}
	c.c.Env = mapToSlice(vars)
	c.c.Args = c.Args
	var err error
	if c.c.Stdout, c.c.Stderr, err = c.makeStdoutStderr(); err != nil {
		return err
	}
	c.c.ExtraFiles = c.ExtraFiles
	// Create a new process group for the child.
	if c.c.SysProcAttr == nil {
		c.c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.c.SysProcAttr.Setpgid = true
	c.c.SysProcAttr.Pgid = 0
	// Start the command.
	if err = c.c.Start(); err != nil {
		return err
	}
	c.started = true
	c.startExitWaiter()
	return nil
}

func (c *Cmd) cleanupProcessGroup() {
	if !c.started {
		return
	}
	c.cleanupMu.Lock()
	defer c.cleanupMu.Unlock()

	if c.calledCleanup {
		return
	}
	c.calledCleanup = true

	// Send SIGINT first; then, after a grace period, send SIGKILL to any
	// process that is still running.
	if err := syscall.Kill(-c.Pid(), syscall.SIGINT); err == syscall.ESRCH {
		return
	}
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := syscall.Kill(-c.Pid(), 0); err == syscall.ESRCH {
			return
		}
	}
	syscall.Kill(-c.Pid(), syscall.SIGKILL)
}