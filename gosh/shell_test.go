// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh_test

// TODO(sadovsky): Add more tests:
// - effects of Shell.Cleanup
// - Cmd.Clone
// - Shell.{Vars,Args,Rename,MakeTempFile,MakeTempDir}
// - Opts (including defaulting behavior)
// - {,Maybe}WatchParent

import (
	"bufio"
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

func toString(t *testing.T, r io.Reader) string {
	b, err := ioutil.ReadAll(r)
	ok(t, err)
	return string(b)
}

func makeFatalf(t *testing.T) func(string, ...interface{}) {
	return func(format string, v ...interface{}) {
		debug.PrintStack()
		t.Fatalf(format, v...)
	}
}

////////////////////////////////////////
// Simple functions

// Simplified versions of various Unix commands.
var (
	catFn = gosh.Register("cat", func() {
		io.Copy(os.Stdout, os.Stdin)
	})
	echoFn = gosh.Register("echo", func() {
		fmt.Println(os.Args[1])
	})
	readFn = gosh.Register("read", func() {
		bufio.NewReader(os.Stdin).ReadString('\n')
	})
)

// Functions with parameters.
var (
	exitFn = gosh.Register("exit", func(code int) {
		os.Exit(code)
	})
	sleepFn = gosh.Register("sleep", func(d time.Duration, code int) {
		time.Sleep(d)
		os.Exit(code)
	})
	printFn = gosh.Register("print", func(v ...interface{}) {
		fmt.Print(v...)
	})
	printfFn = gosh.Register("printf", func(format string, v ...interface{}) {
		fmt.Printf(format, v...)
	})
)

////////////////////////////////////////
// Tests

func TestCustomFatalf(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	defer sh.Cleanup()

	var calledFatalf bool
	sh.Opts.Fatalf = func(string, ...interface{}) { calledFatalf = true }
	sh.HandleError(fakeError)
	// Note, our deferred sh.Cleanup() should succeed despite this error.
	nok(t, sh.Err)
	eq(t, calledFatalf, true)
}

func TestPushdPopd(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
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
	sh.Opts.Fatalf = func(string, ...interface{}) {}
	sh.Popd()
	nok(t, sh.Err)
}

func evalSymlinks(t *testing.T, dir string) string {
	var err error
	dir, err = filepath.EvalSymlinks(dir)
	ok(t, err)
	return dir
}

func getwdEvalSymlinks(t *testing.T) string {
	dir, err := os.Getwd()
	ok(t, err)
	return evalSymlinks(t, dir)
}

func TestPushdNoPopdCleanup(t *testing.T) {
	startDir := getwdEvalSymlinks(t)
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	tmpDir := sh.MakeTempDir()
	sh.Pushd(tmpDir)
	eq(t, getwdEvalSymlinks(t), evalSymlinks(t, tmpDir))
	// There is no matching popd; the cwd is tmpDir, which is deleted by Cleanup.
	// Cleanup needs to put us back in startDir, otherwise all subsequent Pushd
	// calls will fail.
	sh.Cleanup()
	eq(t, getwdEvalSymlinks(t), startDir)
}

func TestCmds(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
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
	eq(t, c.Stdout(), "Hello, world!\n")
}

var (
	getFn   = gosh.Register("get", lib.Get)
	serveFn = gosh.Register("serve", lib.Serve)
)

func TestFns(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	defer sh.Cleanup()

	// Start server.
	c := sh.Fn(serveFn)
	c.Start()
	c.AwaitReady()
	addr := c.AwaitVars("Addr")["Addr"]
	neq(t, addr, "")

	// Run client.
	c = sh.Fn(getFn, addr)
	eq(t, c.Stdout(), "Hello, world!\n")
}

func TestShellMain(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	defer sh.Cleanup()

	c := sh.Main(lib.HelloWorldMain)
	eq(t, c.Stdout(), "Hello, world!\n")
}

// Functions designed for TestRegistry.
var (
	printIntsFn = gosh.Register("printInts", func(v ...int) {
		var vi []interface{}
		for _, x := range v {
			vi = append(vi, x)
		}
		fmt.Print(vi...)
	})
	printfIntsFn = gosh.Register("printfInts", func(format string, v ...int) {
		var vi []interface{}
		for _, x := range v {
			vi = append(vi, x)
		}
		fmt.Printf(format, vi...)
	})
)

// Tests function signature-checking and execution.
func TestRegistry(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	defer sh.Cleanup()

	// Variadic functions. Non-variadic functions are sufficiently covered in
	// other tests.
	eq(t, sh.Fn(printFn).Stdout(), "")
	eq(t, sh.Fn(printFn, 0).Stdout(), "0")
	eq(t, sh.Fn(printFn, 0, "foo").Stdout(), "0foo")
	eq(t, sh.Fn(printfFn, "").Stdout(), "")
	eq(t, sh.Fn(printfFn, "%v", 0).Stdout(), "0")
	eq(t, sh.Fn(printfFn, "%v%v", 0, "foo").Stdout(), "0foo")
	eq(t, sh.Fn(printIntsFn, 1, 2).Stdout(), "1 2")
	eq(t, sh.Fn(printfIntsFn, "%v %v", 1, 2).Stdout(), "1 2")

	// Error cases.
	sh.Opts.Fatalf = func(string, ...interface{}) {}
	reset := func() {
		nok(t, sh.Err)
		sh.Err = nil
	}

	// Too few arguments.
	sh.Fn(exitFn)
	reset()
	sh.Fn(sleepFn, time.Second)
	reset()
	sh.Fn(printfFn)
	reset()

	// Too many arguments.
	sh.Fn(exitFn, 0, 0)
	reset()
	sh.Fn(sleepFn, time.Second, 0, 0)
	reset()

	// Wrong argument types.
	sh.Fn(exitFn, "foo")
	reset()
	sh.Fn(sleepFn, 0, 0)
	reset()
	sh.Fn(printfFn, 0)
	reset()
	sh.Fn(printfFn, 0, 0)
	reset()

	// Wrong variadic argument types.
	sh.Fn(printIntsFn, 0.5)
	reset()
	sh.Fn(printIntsFn, 0, 0.5)
	reset()
	sh.Fn(printfIntsFn, "%v", 0.5)
	reset()
	sh.Fn(printfIntsFn, "%v", 0, 0.5)
	reset()

	// Unsupported argument types.
	var p *int
	sh.Fn(printFn, p)
	reset()
	sh.Fn(printfFn, "%v", p)
	reset()
}

func TestStdin(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	defer sh.Cleanup()

	c := sh.Main(catFn)
	c.Stdin = "foo\n"
	// We set c.Stdin and did not call c.StdinPipe(), so stdin should close and
	// cat should exit immediately.
	eq(t, c.Stdout(), "foo\n")

	c = sh.Main(catFn)
	c.StdinPipe().Write([]byte("foo\n"))
	// The "cat" command only exits when stdin is closed, so we must explicitly
	// close the stdin pipe. Note, it's safe to call c.StdinPipe multiple times.
	c.StdinPipe().Close()
	eq(t, c.Stdout(), "foo\n")

	c = sh.Main(readFn)
	c.StdinPipe().Write([]byte("foo\n"))
	// The "read" command exits when it sees a newline, so Cmd.Wait (and thus
	// Cmd.Run) should return immediately; it should not be necessary to close the
	// stdin pipe.
	c.Run()

	c = sh.Main(catFn)
	// No stdin, so cat should exit immediately.
	eq(t, c.Stdout(), "")

	// It's an error (detected at command start time) to both set c.Stdin and call
	// c.StdinPipe. Note, this indirectly tests that Shell.Cleanup works even if
	// some Cmd.Start failed.
	c = sh.Main(catFn)
	c.Stdin = "foo"
	c.StdinPipe().Write([]byte("bar"))
	c.StdinPipe().Close()
	sh.Opts.Fatalf = func(string, ...interface{}) {}
	c.Start()
	nok(t, sh.Err)
}

var writeFn = gosh.Register("write", func(stdout, stderr bool) error {
	if stdout {
		if _, err := os.Stdout.Write([]byte("A")); err != nil {
			return err
		}
	}
	if stderr {
		if _, err := os.Stderr.Write([]byte("B")); err != nil {
			return err
		}
	}
	if stdout {
		if _, err := os.Stdout.Write([]byte("A")); err != nil {
			return err
		}
	}
	if stderr {
		if _, err := os.Stderr.Write([]byte("B")); err != nil {
			return err
		}
	}
	return nil
})

func TestStdoutStderr(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	defer sh.Cleanup()

	// Write to stdout only.
	c := sh.Fn(writeFn, true, false)
	stdoutPipe, stderrPipe := c.StdoutPipe(), c.StderrPipe()
	stdout, stderr := c.StdoutStderr()
	eq(t, stdout, "AA")
	eq(t, stderr, "")
	eq(t, toString(t, stdoutPipe), "AA")
	eq(t, toString(t, stderrPipe), "")

	// Write to stderr only.
	c = sh.Fn(writeFn, false, true)
	stdoutPipe, stderrPipe = c.StdoutPipe(), c.StderrPipe()
	stdout, stderr = c.StdoutStderr()
	eq(t, stdout, "")
	eq(t, stderr, "BB")
	eq(t, toString(t, stdoutPipe), "")
	eq(t, toString(t, stderrPipe), "BB")

	// Write to both stdout and stderr.
	c = sh.Fn(writeFn, true, true)
	stdoutPipe, stderrPipe = c.StdoutPipe(), c.StderrPipe()
	stdout, stderr = c.StdoutStderr()
	eq(t, stdout, "AA")
	eq(t, stderr, "BB")
	eq(t, toString(t, stdoutPipe), "AA")
	eq(t, toString(t, stderrPipe), "BB")
}

var writeMoreFn = gosh.Register("writeMore", func() {
	sh := gosh.NewShell(gosh.Opts{})
	defer sh.Cleanup()

	c := sh.Fn(writeFn, true, true)
	c.AddStdoutWriter(os.Stdout)
	c.AddStderrWriter(os.Stderr)
	c.Run()

	fmt.Fprint(os.Stdout, " stdout done")
	fmt.Fprint(os.Stderr, " stderr done")
})

// Tests that it's safe to add os.Stdout and os.Stderr as writers.
func TestAddWriters(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	defer sh.Cleanup()

	stdout, stderr := sh.Fn(writeMoreFn).StdoutStderr()
	eq(t, stdout, "AA stdout done")
	eq(t, stderr, "BB stderr done")
}

// Tests piping from one Cmd's stdout/stderr to another's stdin. It should be
// possible to wait on just the last Cmd.
func TestPiping(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	defer sh.Cleanup()

	echo := sh.Main(echoFn, "foo")
	cat := sh.Main(catFn)
	echo.AddStdoutWriter(cat.StdinPipe())
	echo.Start()
	eq(t, cat.Stdout(), "foo\n")

	// This time, pipe both stdout and stderr to cat's stdin.
	c := sh.Fn(writeFn, true, true)
	cat = sh.Main(catFn)
	c.AddStdoutWriter(cat.StdinPipe())
	c.AddStderrWriter(cat.StdinPipe())
	c.Start()
	// Note, we can't assume any particular ordering of stdout and stderr, so we
	// simply check the length of the combined output.
	eq(t, len(cat.Stdout()), 4)
}

func TestShutdown(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	defer sh.Cleanup()

	for _, d := range []time.Duration{0, time.Second} {
		for _, s := range []os.Signal{os.Interrupt, os.Kill} {
			fmt.Println(d, s)
			c := sh.Fn(sleepFn, d, 0)
			c.Start()
			// Wait for a bit to allow the zero-sleep commands to exit, to test that
			// Shutdown succeeds for an exited process and to avoid the race condition
			// in Cmd.Shutdown.
			time.Sleep(10 * time.Millisecond)
			c.Shutdown(s)
		}
	}
}

func TestShellWait(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	defer sh.Cleanup()

	d0 := time.Duration(0)
	d200 := 200 * time.Millisecond

	c0 := sh.Fn(sleepFn, d0, 0)   // not started
	c1 := sh.Fn(sleepFn, d0, 0)   // failed to start
	c2 := sh.Fn(sleepFn, d200, 0) // running and will succeed
	c3 := sh.Fn(sleepFn, d200, 1) // running and will fail
	c4 := sh.Fn(sleepFn, d0, 0)   // succeeded
	c5 := sh.Fn(sleepFn, d0, 0)   // succeeded, called wait
	c6 := sh.Fn(sleepFn, d0, 1)   // failed
	c7 := sh.Fn(sleepFn, d0, 1)   // failed, called wait

	c3.ExitErrorIsOk = true
	c6.ExitErrorIsOk = true
	c7.ExitErrorIsOk = true

	// Configure the "failed to start" command.
	c1.StdinPipe()
	c1.Stdin = "foo"
	sh.Opts.Fatalf = func(string, ...interface{}) {}
	c1.Start()
	nok(t, sh.Err)
	sh.Err = nil
	sh.Opts.Fatalf = makeFatalf(t)

	// Start commands, call wait.
	for _, c := range []*gosh.Cmd{c2, c3, c4, c5, c6, c7} {
		c.Start()
	}
	c5.Wait()
	c7.Wait()
	sh.Wait()

	// It should be possible to run existing unstarted commands, and to create and
	// run new commands, after calling Shell.Wait.
	c0.Run()
	sh.Fn(sleepFn, d0, 0).Run()
	sh.Fn(sleepFn, d0, 0).Start()

	// Call Shell.Wait again.
	sh.Wait()
}

func TestExitErrorIsOk(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	defer sh.Cleanup()

	// Exit code 0 is not an error.
	c := sh.Fn(exitFn, 0)
	c.Run()
	ok(t, c.Err)
	ok(t, sh.Err)

	// Exit code 1 is an error.
	c = sh.Fn(exitFn, 1)
	c.ExitErrorIsOk = true
	c.Run()
	nok(t, c.Err)
	ok(t, sh.Err)

	// If ExitErrorIsOk is false, exit code 1 triggers sh.HandleError.
	c = sh.Fn(exitFn, 1)
	sh.Opts.Fatalf = func(string, ...interface{}) {}
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
		sh := gosh.NewShell(gosh.Opts{Fatalf: t.Logf})
		defer sh.Cleanup()
		sh.Err = fakeError
		defer func() { neq(t, recover(), nil) }()
		sh.Ok()
	}()
	func() { // errAlreadyCalledCleanup
		sh := gosh.NewShell(gosh.Opts{Fatalf: t.Logf})
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
		sh := gosh.NewShell(gosh.Opts{Fatalf: t.Logf})
		defer sh.Cleanup()
		sh.Err = fakeError
		defer func() { neq(t, recover(), nil) }()
		sh.HandleError(fakeError)
	}()
	func() { // errAlreadyCalledCleanup
		sh := gosh.NewShell(gosh.Opts{Fatalf: t.Logf})
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
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	sh.Err = fakeError
	sh.Cleanup()
}

// Tests that sh.Cleanup can be called multiple times.
func TestMultipleCleanup(t *testing.T) {
	sh := gosh.NewShell(gosh.Opts{Fatalf: makeFatalf(t), Logf: t.Logf})
	sh.Cleanup()
	sh.Cleanup()
}

func TestMain(m *testing.M) {
	os.Exit(gosh.Run(m.Run))
}
