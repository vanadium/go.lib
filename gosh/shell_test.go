// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh_test

// TODO(sadovsky): Add more tests:
// - variadic function registration and invocation
// - effects of Shell.Cleanup
// - Cmd.{Wait,Run}
// - Shell.{Args,Wait,Rename,MakeTempFile,MakeTempDir}
// - Opts (including defaulting behavior)
// - {,Maybe}WatchParent

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"testing"
	"time"

	"v.io/x/lib/gosh"
	"v.io/x/lib/gosh/internal/gosh_example_lib"
)

var fakeError = errors.New("fake error")

func fatal(t *testing.T, v ...interface{}) {
	debug.PrintStack()
	t.Fatal(v...)
}

func fatalf(t *testing.T, format string, v ...interface{}) {
	debug.PrintStack()
	t.Fatalf(format, v...)
}

func ok(t *testing.T, err error) {
	if err != nil {
		fatal(t, err)
	}
}

func nok(t *testing.T, err error) {
	if err == nil {
		fatal(t, "nil err")
	}
}

func eq(t *testing.T, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		fatalf(t, "got %v, want %v", got, want)
	}
}

func neq(t *testing.T, got, notWant interface{}) {
	if reflect.DeepEqual(got, notWant) {
		fatalf(t, "got %v", got)
	}
}

func makeErrorf(t *testing.T) func(string, ...interface{}) {
	return func(format string, v ...interface{}) {
		debug.PrintStack()
		t.Fatalf(format, v...)
	}
}

func TestPushdPopd(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Errorf: makeErrorf(t), Logf: t.Logf})
	defer sh.Cleanup()
	startDir, err := os.Getwd()
	ok(t, err)
	parentDir := filepath.Dir(startDir)
	neq(t, startDir, parentDir)
	sh.Pushd(parentDir)
	cwd, err := os.Getwd()
	ok(t, err)
	eq(t, cwd, parentDir)
	sh.Pushd(startDir)
	cwd, err = os.Getwd()
	ok(t, err)
	eq(t, cwd, startDir)
	sh.Popd()
	cwd, err = os.Getwd()
	ok(t, err)
	eq(t, cwd, parentDir)
	sh.Popd()
	cwd, err = os.Getwd()
	ok(t, err)
	eq(t, cwd, startDir)
	// The next sh.Popd() will fail.
	var calledErrorf bool
	sh.Opts.Errorf = func(string, ...interface{}) { calledErrorf = true }
	sh.Popd()
	// Note, our deferred sh.Cleanup() should succeed despite this error.
	nok(t, sh.Err)
	eq(t, calledErrorf, true)
}

func TestCmds(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Errorf: makeErrorf(t), Logf: t.Logf})
	defer sh.Cleanup()

	// Start server.
	binPath := sh.BuildGoPkg("v.io/x/lib/gosh/internal/gosh_example_server")
	c := sh.Cmd(binPath)
	c.Start()
	c.AwaitReady()
	addr := c.AwaitVars("Addr")["Addr"]
	neq(t, addr, "")

	// Run client.
	binPath = sh.BuildGoPkg("v.io/x/lib/gosh/internal/gosh_example_client")
	c = sh.Cmd(binPath, "-addr="+addr)
	stdout, _ := c.Output()
	eq(t, stdout, "Hello, world!\n")
}

var (
	get   = gosh.Register("get", lib.Get)
	serve = gosh.Register("serve", lib.Serve)
)

func TestFns(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Errorf: makeErrorf(t), Logf: t.Logf})
	defer sh.Cleanup()

	// Start server.
	c := sh.Fn(serve)
	c.Start()
	c.AwaitReady()
	addr := c.AwaitVars("Addr")["Addr"]
	neq(t, addr, "")

	// Run client.
	c = sh.Fn(get, addr)
	stdout, _ := c.Output()
	eq(t, stdout, "Hello, world!\n")
}

func TestShellMain(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Errorf: makeErrorf(t), Logf: t.Logf})
	defer sh.Cleanup()
	stdout, _ := sh.Main(lib.HelloWorldMain).Output()
	eq(t, stdout, "Hello, world!\n")
}

var write = gosh.Register("write", func(stdout, stderr bool) error {
	tenMs := 10 * time.Millisecond
	if stdout {
		time.Sleep(tenMs)
		if _, err := os.Stdout.Write([]byte("A")); err != nil {
			return err
		}
	}
	if stderr {
		time.Sleep(tenMs)
		if _, err := os.Stderr.Write([]byte("B")); err != nil {
			return err
		}
	}
	if stdout {
		time.Sleep(tenMs)
		if _, err := os.Stdout.Write([]byte("A")); err != nil {
			return err
		}
	}
	if stderr {
		time.Sleep(tenMs)
		if _, err := os.Stderr.Write([]byte("B")); err != nil {
			return err
		}
	}
	return nil
})

func toString(r io.Reader) string {
	if b, err := ioutil.ReadAll(r); err != nil {
		panic(err)
	} else {
		return string(b)
	}
}

func TestStdoutStderr(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Errorf: makeErrorf(t), Logf: t.Logf})
	defer sh.Cleanup()

	// Write to stdout only.
	c := sh.Fn(write, true, false)
	stdoutPipe, stderrPipe := c.StdoutPipe(), c.StderrPipe()
	eq(t, c.CombinedOutput(), "AA")
	eq(t, toString(stdoutPipe), "AA")
	eq(t, toString(stderrPipe), "")
	stdout, stderr := sh.Fn(write, true, false).Output()
	eq(t, stdout, "AA")
	eq(t, stderr, "")

	// Write to stderr only.
	c = sh.Fn(write, false, true)
	stdoutPipe, stderrPipe = c.StdoutPipe(), c.StderrPipe()
	eq(t, c.CombinedOutput(), "BB")
	eq(t, toString(stdoutPipe), "")
	eq(t, toString(stderrPipe), "BB")
	stdout, stderr = sh.Fn(write, false, true).Output()
	eq(t, stdout, "")
	eq(t, stderr, "BB")

	// Write to both stdout and stderr.
	c = sh.Fn(write, true, true)
	stdoutPipe, stderrPipe = c.StdoutPipe(), c.StderrPipe()
	eq(t, c.CombinedOutput(), "ABAB")
	eq(t, toString(stdoutPipe), "AA")
	eq(t, toString(stderrPipe), "BB")
	stdout, stderr = sh.Fn(write, true, true).Output()
	eq(t, stdout, "AA")
	eq(t, stderr, "BB")
}

var sleep = gosh.Register("sleep", func(d time.Duration) {
	time.Sleep(d)
})

func TestShutdown(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Errorf: makeErrorf(t), Logf: t.Logf})
	defer sh.Cleanup()

	for _, d := range []time.Duration{0, time.Second} {
		for _, s := range []os.Signal{os.Interrupt, os.Kill} {
			fmt.Println(d, s)
			c := sh.Fn(sleep, d)
			c.Start()
			time.Sleep(10 * time.Millisecond)
			c.Shutdown(s)
		}
	}
}

var exit = gosh.Register("exit", func(code int) {
	os.Exit(code)
})

func TestExitErrorIsOk(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Errorf: makeErrorf(t), Logf: t.Logf})
	defer sh.Cleanup()

	// Exit code 0 is not an error.
	c := sh.Fn(exit, 0)
	c.Run()
	ok(t, c.Err)
	ok(t, sh.Err)

	// Exit code 1 is an error.
	c = sh.Fn(exit, 1)
	c.ExitErrorIsOk = true
	c.Run()
	nok(t, c.Err)
	ok(t, sh.Err)

	// If ExitErrorIsOk is false, exit code 1 triggers sh.HandleError.
	sh.Opts.Errorf = func(string, ...interface{}) {}
	c = sh.Fn(exit, 1)
	c.Run()
	nok(t, c.Err)
	nok(t, sh.Err)
}

// Tests that sh.Ok panics under various conditions.
func TestOkPanics(t *testing.T) {
	func() { // errDidNotCallNewShell
		sh := gosh.Shell{}
		defer func() { neq(t, recover(), nil) }()
		sh.Ok()
	}()
	func() { // errShellErrIsNotNil
		sh := gosh.NewShell(gosh.Opts{Errorf: t.Logf})
		defer sh.Cleanup()
		sh.Err = fakeError
		defer func() { neq(t, recover(), nil) }()
		sh.Ok()
	}()
	func() { // errAlreadyCalledCleanup
		sh := gosh.NewShell(gosh.Opts{Errorf: t.Logf})
		sh.Cleanup()
		defer func() { neq(t, recover(), nil) }()
		sh.Ok()
	}()
}

// Tests that sh.HandleError panics under various conditions.
func TestHandleErrorPanics(t *testing.T) {
	func() { // errDidNotCallNewShell
		sh := gosh.Shell{}
		defer func() { neq(t, recover(), nil) }()
		sh.HandleError(fakeError)
	}()
	func() { // errShellErrIsNotNil
		sh := gosh.NewShell(gosh.Opts{Errorf: t.Logf})
		defer sh.Cleanup()
		sh.Err = fakeError
		defer func() { neq(t, recover(), nil) }()
		sh.HandleError(fakeError)
	}()
	func() { // errAlreadyCalledCleanup
		sh := gosh.NewShell(gosh.Opts{Errorf: t.Logf})
		sh.Cleanup()
		defer func() { neq(t, recover(), nil) }()
		sh.HandleError(fakeError)
	}()
}

// Tests that sh.Cleanup panics under various conditions.
func TestCleanupPanics(t *testing.T) {
	func() { // errDidNotCallNewShell
		sh := gosh.Shell{}
		defer func() { neq(t, recover(), nil) }()
		sh.Cleanup()
	}()
}

// Tests that sh.Cleanup succeeds even if sh.Err is not nil.
func TestCleanupAfterError(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Errorf: makeErrorf(t), Logf: t.Logf})
	sh.Err = fakeError
	sh.Cleanup()
}

// Tests that sh.Cleanup can be called multiple times.
func TestMultipleCleanup(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Errorf: makeErrorf(t), Logf: t.Logf})
	sh.Cleanup()
	sh.Cleanup()
}

func TestMain(m *testing.M) {
	os.Exit(gosh.Run(m.Run))
}
