// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh_test

// TODO(sadovsky): Add more tests:
// - effects of Shell.Cleanup
// - Shell.{PropagateChildOutput,ChildOutputDir,Vars,Args}
// - Cmd.{IgnoreParentExit,ExitAfter,PropagateOutput}
// - Cmd.Clone

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"v.io/x/lib/gosh"
	lib "v.io/x/lib/gosh/internal/gosh_example_lib"
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

func setsErr(t *testing.T, sh *gosh.Shell, f func()) {
	continueOnError := sh.ContinueOnError
	sh.ContinueOnError = true
	f()
	nok(t, sh.Err)
	sh.Err = nil
	sh.ContinueOnError = continueOnError
}

////////////////////////////////////////////////////////////////////////////////
// Simple registered functions

// Simplified versions of various Unix commands.
var (
	catFunc = gosh.RegisterFunc("catFunc", func() error {
		_, err := io.Copy(os.Stdout, os.Stdin)
		return err
	})
	echoFunc = gosh.RegisterFunc("echoFunc", func() error {
		_, err := fmt.Println(os.Args[1])
		return err
	})
	readFunc = gosh.RegisterFunc("readFunc", func() {
		bufio.NewReader(os.Stdin).ReadString('\n')
	})
)

// Functions with parameters.
var (
	exitFunc = gosh.RegisterFunc("exitFunc", func(code int) {
		os.Exit(code)
	})
	sleepFunc = gosh.RegisterFunc("sleepFunc", func(d time.Duration, code int) {
		// For TestSignal and TestTerminate.
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		go func() {
			<-ch
			os.Exit(0)
		}()
		// The parent waits for this "ready" notification to avoid the race where a
		// signal is sent before the handler is installed.
		gosh.SendVars(map[string]string{"ready": ""})
		time.Sleep(d)
		os.Exit(code)
	})
	printFunc = gosh.RegisterFunc("printFunc", func(v ...interface{}) {
		fmt.Print(v...)
	})
	printfFunc = gosh.RegisterFunc("printfFunc", func(format string, v ...interface{}) {
		fmt.Printf(format, v...)
	})
)

////////////////////////////////////////////////////////////////////////////////
// Shell tests

type customTB struct {
	t             *testing.T
	calledFailNow bool
	buf           *bytes.Buffer
}

func (tb *customTB) Reset() {
	tb.calledFailNow = false
	if tb.buf != nil {
		tb.buf.Reset()
	}
}

func (tb *customTB) FailNow() {
	tb.calledFailNow = true
}

func (tb *customTB) Logf(format string, args ...interface{}) {
	if tb.buf == nil {
		tb.t.Logf(format, args...)
	} else {
		fmt.Fprintf(tb.buf, format, args...)
	}
}

func TestCustomTB(t *testing.T) {
	tb := &customTB{t: t}
	sh := gosh.NewShell(tb)
	defer sh.Cleanup()

	sh.HandleError(fakeError)
	// Note, our deferred sh.Cleanup() should succeed despite this error.
	nok(t, sh.Err)
	eq(t, tb.calledFailNow, true)
}

func TestPushdPopd(t *testing.T) {
	sh := gosh.NewShell(t)
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
	setsErr(t, sh, func() { sh.Popd() })
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
	sh := gosh.NewShell(t)
	tmpDir := sh.MakeTempDir()
	sh.Pushd(tmpDir)
	eq(t, getwdEvalSymlinks(t), evalSymlinks(t, tmpDir))
	// There is no matching popd; the cwd is tmpDir, which is deleted by Cleanup.
	// Cleanup needs to put us back in startDir, otherwise all subsequent Pushd
	// calls will fail.
	sh.Cleanup()
	eq(t, getwdEvalSymlinks(t), startDir)
}

func TestMakeTempDir(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	name := sh.MakeTempDir()
	fi, err := os.Stat(name)
	ok(t, err)
	eq(t, fi.Mode().IsDir(), true)
}

func TestMakeTempFile(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	file := sh.MakeTempFile()
	fi, err := file.Stat()
	ok(t, err)
	eq(t, fi.Mode().IsRegular(), true)
}

func TestMove(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// TODO(sadovsky): Run all tests twice: once with oldpath and newpath on the
	// same volume, and once with oldpath and newpath on different volumes.
	src, dst := sh.MakeTempDir(), sh.MakeTempDir()
	// Foo files exist, bar files do not.
	srcFoo, dstFoo := filepath.Join(src, "srcFoo"), filepath.Join(dst, "dstFoo")
	srcBar, dstBar := filepath.Join(src, "srcBar"), filepath.Join(dst, "dstBar")
	ioutil.WriteFile(srcFoo, []byte("srcFoo"), 0600)
	ioutil.WriteFile(dstFoo, []byte("dstFoo"), 0600)

	// Move should fail if source does not exist.
	setsErr(t, sh, func() { sh.Move(srcBar, dstBar) })

	// Move should fail if source is a directory.
	setsErr(t, sh, func() { sh.Move(src, dstBar) })

	// Move should fail if destination exists, regardless of whether it is a
	// regular file or a directory.
	setsErr(t, sh, func() { sh.Move(srcFoo, dstFoo) })
	setsErr(t, sh, func() { sh.Move(srcFoo, dst) })

	// Move should fail if destination's parent does not exist.
	setsErr(t, sh, func() { sh.Move(srcFoo, filepath.Join(dst, "subdir", "a")) })

	// Move should succeed if source exists and is a file, destination does not
	// exist, and destination's parent exists.
	sh.Move(srcFoo, dstBar)
	if _, err := os.Stat(srcFoo); !os.IsNotExist(err) {
		t.Fatalf("got %v, expected IsNotExist", err)
	}
	buf, err := ioutil.ReadFile(dstBar)
	ok(t, err)
	eq(t, string(buf), "srcFoo")
}

func TestShellWait(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	d0 := time.Duration(0)
	d200 := 200 * time.Millisecond

	c0 := sh.FuncCmd(sleepFunc, d0, 0)   // not started
	c1 := sh.Cmd("/#invalid#/!binary!")  // failed to start
	c2 := sh.FuncCmd(sleepFunc, d200, 0) // running and will succeed
	c3 := sh.FuncCmd(sleepFunc, d200, 1) // running and will fail
	c4 := sh.FuncCmd(sleepFunc, d0, 0)   // succeeded
	c5 := sh.FuncCmd(sleepFunc, d0, 0)   // succeeded, called wait
	c6 := sh.FuncCmd(sleepFunc, d0, 1)   // failed
	c7 := sh.FuncCmd(sleepFunc, d0, 1)   // failed, called wait

	c3.ExitErrorIsOk = true
	c6.ExitErrorIsOk = true
	c7.ExitErrorIsOk = true

	// Make sure c1 fails to start. This indirectly tests that Shell.Cleanup works
	// even if Cmd.Start failed.
	setsErr(t, sh, c1.Start)

	// Start commands, then wait for them to exit.
	for _, c := range []*gosh.Cmd{c2, c3, c4, c5, c6, c7} {
		c.Start()
	}
	// Wait for a bit to allow the zero-sleep commands to exit.
	time.Sleep(100 * time.Millisecond)
	c5.Wait()
	c7.Wait()
	sh.Wait()

	// It should be possible to run existing unstarted commands, and to create and
	// run new commands, after calling Shell.Wait.
	c0.Run()
	sh.FuncCmd(sleepFunc, d0, 0).Run()
	sh.FuncCmd(sleepFunc, d0, 0).Start()

	// Call Shell.Wait again.
	sh.Wait()
}

// Tests that Shell.Ok panics under various conditions.
func TestOkPanics(t *testing.T) {
	func() { // errDidNotCallNewShell
		sh := gosh.Shell{}
		defer func() { neq(t, recover(), nil) }()
		sh.Ok()
	}()
	func() { // errShellErrIsNotNil
		sh := gosh.NewShell(t)
		sh.ContinueOnError = true
		defer sh.Cleanup()
		sh.Err = fakeError
		defer func() { neq(t, recover(), nil) }()
		sh.Ok()
	}()
	func() { // errAlreadyCalledCleanup
		sh := gosh.NewShell(t)
		sh.ContinueOnError = true
		sh.Cleanup()
		defer func() { neq(t, recover(), nil) }()
		sh.Ok()
	}()
}

// Tests that Shell.HandleError panics under various conditions.
func TestHandleErrorPanics(t *testing.T) {
	func() { // errDidNotCallNewShell
		sh := gosh.Shell{}
		defer func() { neq(t, recover(), nil) }()
		sh.HandleError(fakeError)
	}()
	func() { // errShellErrIsNotNil
		sh := gosh.NewShell(t)
		sh.ContinueOnError = true
		defer sh.Cleanup()
		sh.Err = fakeError
		defer func() { neq(t, recover(), nil) }()
		sh.HandleError(fakeError)
	}()
	func() { // errAlreadyCalledCleanup
		sh := gosh.NewShell(t)
		sh.ContinueOnError = true
		sh.Cleanup()
		defer func() { neq(t, recover(), nil) }()
		sh.HandleError(fakeError)
	}()
}

// Tests that Shell.Cleanup panics under various conditions.
func TestCleanupPanics(t *testing.T) {
	func() { // errDidNotCallNewShell
		sh := gosh.Shell{}
		defer func() { neq(t, recover(), nil) }()
		sh.Cleanup()
	}()
}

// Tests that Shell.Cleanup succeeds even if sh.Err is not nil.
func TestCleanupAfterError(t *testing.T) {
	sh := gosh.NewShell(t)
	sh.Err = fakeError
	sh.Cleanup()
}

// Tests that Shell.Cleanup can be called multiple times.
func TestMultipleCleanup(t *testing.T) {
	sh := gosh.NewShell(t)
	sh.Cleanup()
	sh.Cleanup()
}

// Tests that Shell.HandleError logs errors using an appropriate runtime.Caller
// skip value.
func TestHandleErrorLogging(t *testing.T) {
	tb := &customTB{t: t, buf: &bytes.Buffer{}}
	sh := gosh.NewShell(tb)
	defer sh.Cleanup()

	// Call HandleError, then check that the stack trace and error got logged.
	tb.Reset()
	sh.HandleError(fakeError)
	_, file, line, _ := runtime.Caller(0)
	got, wantSuffix := tb.buf.String(), fmt.Sprintf("%s:%d: %v\n", filepath.Base(file), line-1, fakeError)
	if !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("got %v, want suffix %v", got, wantSuffix)
	}
	if got == wantSuffix {
		t.Fatalf("missing stack trace: %v", got)
	}
	sh.Err = nil

	// Same as above, but with ContinueOnError set to true. Only the error should
	// get logged.
	sh.ContinueOnError = true
	tb.Reset()
	sh.HandleError(fakeError)
	_, file, line, _ = runtime.Caller(0)
	got, want := tb.buf.String(), fmt.Sprintf("%s:%d: %v\n", filepath.Base(file), line-1, fakeError)
	eq(t, got, want)
	sh.Err = nil

	// Same as above, but calling HandleErrorWithSkip, with skip set to 1.
	tb.Reset()
	sh.HandleErrorWithSkip(fakeError, 1)
	_, file, line, _ = runtime.Caller(0)
	got, want = tb.buf.String(), fmt.Sprintf("%s:%d: %v\n", filepath.Base(file), line-1, fakeError)
	eq(t, got, want)
	sh.Err = nil

	// Same as above, but with skip set to 2.
	tb.Reset()
	sh.HandleErrorWithSkip(fakeError, 2)
	_, file, line, _ = runtime.Caller(1)
	got, want = tb.buf.String(), fmt.Sprintf("%s:%d: %v\n", filepath.Base(file), line, fakeError)
	eq(t, got, want)
	sh.Err = nil
}

////////////////////////////////////////////////////////////////////////////////
// Cmd tests

const (
	helloWorldPkg = "v.io/x/lib/gosh/internal/hello_world"
	helloWorldStr = "Hello, world!\n"
)

// Mirrors ExampleCmd in internal/gosh_example/main.go.
func TestCmd(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Start server.
	binDir := sh.MakeTempDir()
	binPath := gosh.BuildGoPkg(sh, binDir, "v.io/x/lib/gosh/internal/gosh_example_server")
	c := sh.Cmd(binPath)
	c.Start()
	addr := c.AwaitVars("addr")["addr"]
	neq(t, addr, "")

	// Run client.
	binPath = gosh.BuildGoPkg(sh, binDir, "v.io/x/lib/gosh/internal/gosh_example_client")
	c = sh.Cmd(binPath, "-addr="+addr)
	eq(t, c.Stdout(), helloWorldStr)
}

var (
	getFunc   = gosh.RegisterFunc("getFunc", lib.Get)
	serveFunc = gosh.RegisterFunc("serveFunc", lib.Serve)
)

// Mirrors ExampleFuncCmd in internal/gosh_example/main.go.
func TestFuncCmd(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Start server.
	c := sh.FuncCmd(serveFunc)
	c.Start()
	addr := c.AwaitVars("addr")["addr"]
	neq(t, addr, "")

	// Run client.
	c = sh.FuncCmd(getFunc, addr)
	eq(t, c.Stdout(), helloWorldStr)
}

// Tests that Shell.Cmd uses Shell.Vars["PATH"] to locate executables with
// relative names.
func TestLookPath(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	binDir := sh.MakeTempDir()
	sh.Vars["PATH"] = binDir + ":" + sh.Vars["PATH"]
	relName := "hw"
	absName := filepath.Join(binDir, relName)
	gosh.BuildGoPkg(sh, "", helloWorldPkg, "-o", absName)
	c := sh.Cmd(relName)
	eq(t, c.Stdout(), helloWorldStr)

	// Test the case where we cannot find the executable.
	sh.Vars["PATH"] = ""
	setsErr(t, sh, func() { sh.Cmd("yes") })
}

var (
	sendVarsFunc = gosh.RegisterFunc("sendVarsFunc", func(vars map[string]string) {
		gosh.SendVars(vars)
		time.Sleep(time.Hour)
	})
	stderrFunc = gosh.RegisterFunc("stderrFunc", func(s string) {
		fmt.Fprintf(os.Stderr, s)
		time.Sleep(time.Hour)
	})
)

// Tests that AwaitVars works under various conditions.
func TestAwaitVars(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	c := sh.FuncCmd(sendVarsFunc, map[string]string{"a": "1"})
	c.Start()
	eq(t, c.AwaitVars("a")["a"], "1")

	c = sh.FuncCmd(stderrFunc, `<goshVars{"a":"1","b":"2"}goshVars>`)
	c.Start()
	vars := c.AwaitVars("a", "b")
	eq(t, vars["a"], "1")
	eq(t, vars["b"], "2")

	c = sh.FuncCmd(stderrFunc, `<goshVars{"a":"1"}goshVars><gosh`)
	c.Start()
	eq(t, c.AwaitVars("a")["a"], "1")

	c = sh.FuncCmd(stderrFunc, `<goshVars{"a":"1"}goshVars><goshVars{"b":"2"}goshVars>`)
	c.Start()
	vars = c.AwaitVars("a", "b")
	eq(t, vars["a"], "1")
	eq(t, vars["b"], "2")

	c = sh.FuncCmd(stderrFunc, `<goshVars{"a":"1","b":"2"}goshVars>`)
	c.Start()
	vars = c.AwaitVars("a")
	eq(t, vars["a"], "1")
	eq(t, vars["b"], "")
	vars = c.AwaitVars("b")
	eq(t, vars["a"], "")
	eq(t, vars["b"], "2")

	c = sh.FuncCmd(stderrFunc, `<g<goshVars{"a":"goshVars"}goshVars>s><goshVars`)
	c.Start()
	eq(t, c.AwaitVars("a")["a"], "goshVars")

	c = sh.FuncCmd(stderrFunc, `<<goshVars{"a":"1"}goshVars>><<goshVars{"b":"<goshVars"}goshVars>>`)
	c.Start()
	vars = c.AwaitVars("a", "b")
	eq(t, vars["a"], "1")
	eq(t, vars["b"], "<goshVars")
}

// Tests that AwaitVars returns immediately when the process exits.
func TestAwaitVarsProcessExit(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	c := sh.FuncCmd(exitFunc, 0)
	c.Start()
	setsErr(t, sh, func() { c.AwaitVars("foo") })
}

// Functions designed for TestRegistry.
var (
	printIntsFunc = gosh.RegisterFunc("printIntsFunc", func(v ...int) {
		var vi []interface{}
		for _, x := range v {
			vi = append(vi, x)
		}
		fmt.Print(vi...)
	})
	printfIntsFunc = gosh.RegisterFunc("printfIntsFunc", func(format string, v ...int) {
		var vi []interface{}
		for _, x := range v {
			vi = append(vi, x)
		}
		fmt.Printf(format, vi...)
	})
)

// Tests function signature-checking and execution.
func TestRegistry(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Variadic functions. Non-variadic functions are sufficiently covered in
	// other tests.
	eq(t, sh.FuncCmd(printFunc).Stdout(), "")
	eq(t, sh.FuncCmd(printFunc, 0).Stdout(), "0")
	eq(t, sh.FuncCmd(printFunc, 0, "foo").Stdout(), "0foo")
	eq(t, sh.FuncCmd(printfFunc, "").Stdout(), "")
	eq(t, sh.FuncCmd(printfFunc, "%v", 0).Stdout(), "0")
	eq(t, sh.FuncCmd(printfFunc, "%v%v", 0, "foo").Stdout(), "0foo")
	eq(t, sh.FuncCmd(printIntsFunc, 1, 2).Stdout(), "1 2")
	eq(t, sh.FuncCmd(printfIntsFunc, "%v %v", 1, 2).Stdout(), "1 2")

	// Too few arguments.
	setsErr(t, sh, func() { sh.FuncCmd(exitFunc) })
	setsErr(t, sh, func() { sh.FuncCmd(sleepFunc, time.Second) })
	setsErr(t, sh, func() { sh.FuncCmd(printfFunc) })

	// Too many arguments.
	setsErr(t, sh, func() { sh.FuncCmd(exitFunc, 0, 0) })
	setsErr(t, sh, func() { sh.FuncCmd(sleepFunc, time.Second, 0, 0) })

	// Wrong argument types.
	setsErr(t, sh, func() { sh.FuncCmd(exitFunc, "foo") })
	setsErr(t, sh, func() { sh.FuncCmd(sleepFunc, 0, 0) })
	setsErr(t, sh, func() { sh.FuncCmd(printfFunc, 0) })
	setsErr(t, sh, func() { sh.FuncCmd(printfFunc, 0, 0) })

	// Wrong variadic argument types.
	setsErr(t, sh, func() { sh.FuncCmd(printIntsFunc, 0.5) })
	setsErr(t, sh, func() { sh.FuncCmd(printIntsFunc, 0, 0.5) })
	setsErr(t, sh, func() { sh.FuncCmd(printfIntsFunc, "%v", 0.5) })
	setsErr(t, sh, func() { sh.FuncCmd(printfIntsFunc, "%v", 0, 0.5) })

	// Unsupported argument types.
	var p *int
	setsErr(t, sh, func() { sh.FuncCmd(printFunc, p) })
	setsErr(t, sh, func() { sh.FuncCmd(printfFunc, "%v", p) })
}

func TestStdin(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// The "cat" command exits after the reader returns EOF.
	c := sh.FuncCmd(catFunc)
	c.SetStdinReader(strings.NewReader("foo\n"))
	eq(t, c.Stdout(), "foo\n")

	// The "cat" command exits after the reader returns EOF, so we must explicitly
	// close the stdin pipe.
	c = sh.FuncCmd(catFunc)
	stdin := c.StdinPipe()
	stdin.Write([]byte("foo\n"))
	stdin.Close()
	eq(t, c.Stdout(), "foo\n")

	// The "read" command exits when it sees a newline, so it is not necessary to
	// explicitly close the stdin pipe.
	c = sh.FuncCmd(readFunc)
	stdin = c.StdinPipe()
	stdin.Write([]byte("foo\n"))
	c.Run()

	// No stdin, so cat should exit immediately.
	c = sh.FuncCmd(catFunc)
	eq(t, c.Stdout(), "")

	// It's an error to call both StdinPipe and SetStdinReader.
	c = sh.FuncCmd(catFunc)
	c.StdinPipe()
	setsErr(t, sh, func() { c.StdinPipe() })

	c = sh.FuncCmd(catFunc)
	c.StdinPipe()
	setsErr(t, sh, func() { c.SetStdinReader(strings.NewReader("")) })

	c = sh.FuncCmd(catFunc)
	c.SetStdinReader(strings.NewReader(""))
	setsErr(t, sh, func() { c.StdinPipe() })

	c = sh.FuncCmd(catFunc)
	c.SetStdinReader(strings.NewReader(""))
	setsErr(t, sh, func() { c.SetStdinReader(strings.NewReader("")) })
}

func TestStdinPipeWriteUntilExit(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Ensure that Write calls on stdin fail after the process exits. Note that we
	// write to the command's stdin concurrently with the command's exit waiter
	// goroutine closing stdin. Use "go test -race" catch races.
	//
	// Set a non-zero exit code, so that os.Exit exits immediately. See the
	// implementation of https://golang.org/pkg/os/#Exit for details.
	c := sh.FuncCmd(exitFunc, 1)
	c.ExitErrorIsOk = true
	stdin := c.StdinPipe()
	c.Start()
	for {
		if _, err := stdin.Write([]byte("a")); err != nil {
			return
		}
	}
}

var writeFunc = gosh.RegisterFunc("writeFunc", func(stdout, stderr bool) error {
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
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Write to stdout only.
	c := sh.FuncCmd(writeFunc, true, false)
	stdoutPipe, stderrPipe := c.StdoutPipe(), c.StderrPipe()
	stdout, stderr := c.StdoutStderr()
	eq(t, stdout, "AA")
	eq(t, stderr, "")
	eq(t, toString(t, stdoutPipe), "AA")
	eq(t, toString(t, stderrPipe), "")

	// Write to stderr only.
	c = sh.FuncCmd(writeFunc, false, true)
	stdoutPipe, stderrPipe = c.StdoutPipe(), c.StderrPipe()
	stdout, stderr = c.StdoutStderr()
	eq(t, stdout, "")
	eq(t, stderr, "BB")
	eq(t, toString(t, stdoutPipe), "")
	eq(t, toString(t, stderrPipe), "BB")

	// Write to both stdout and stderr.
	c = sh.FuncCmd(writeFunc, true, true)
	stdoutPipe, stderrPipe = c.StdoutPipe(), c.StderrPipe()
	stdout, stderr = c.StdoutStderr()
	eq(t, stdout, "AA")
	eq(t, stderr, "BB")
	eq(t, toString(t, stdoutPipe), "AA")
	eq(t, toString(t, stderrPipe), "BB")
}

var writeMoreFunc = gosh.RegisterFunc("writeMoreFunc", func() {
	sh := gosh.NewShell(nil)
	defer sh.Cleanup()

	c := sh.FuncCmd(writeFunc, true, true)
	c.AddStdoutWriter(os.Stdout)
	c.AddStderrWriter(os.Stderr)
	c.Run()

	fmt.Fprint(os.Stdout, " stdout done")
	fmt.Fprint(os.Stderr, " stderr done")
})

// Tests that it's safe to add os.Stdout and os.Stderr as writers.
func TestAddStdoutStderrWriter(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	stdout, stderr := sh.FuncCmd(writeMoreFunc).StdoutStderr()
	eq(t, stdout, "AA stdout done")
	eq(t, stderr, "BB stderr done")
}

func TestCombinedOutput(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	c := sh.FuncCmd(writeFunc, true, true)
	buf := &bytes.Buffer{}
	c.AddStdoutWriter(buf)
	c.AddStderrWriter(buf)
	output := c.CombinedOutput()
	// Note, we can't assume any particular ordering of stdout and stderr, so we
	// simply check the length of the combined output.
	eq(t, len(output), 4)
	// The ordering must be the same, regardless of how we captured the combined
	// output.
	eq(t, output, buf.String())
}

func TestOutputDir(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	dir := sh.MakeTempDir()
	c := sh.FuncCmd(writeFunc, true, true)
	c.OutputDir = dir
	c.Run()

	matches, err := filepath.Glob(filepath.Join(dir, "*.stdout"))
	ok(t, err)
	eq(t, len(matches), 1)
	stdout, err := ioutil.ReadFile(matches[0])
	ok(t, err)
	eq(t, string(stdout), "AA")

	matches, err = filepath.Glob(filepath.Join(dir, "*.stderr"))
	ok(t, err)
	eq(t, len(matches), 1)
	stderr, err := ioutil.ReadFile(matches[0])
	ok(t, err)
	eq(t, string(stderr), "BB")
}

var replaceFunc = gosh.RegisterFunc("replaceFunc", func(old, new byte) error {
	buf := make([]byte, 1024)
	for {
		n, err := os.Stdin.Read(buf)
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		}
		rep := bytes.Replace(buf[:n], []byte{old}, []byte{new}, -1)
		if _, err := os.Stdout.Write(rep); err != nil {
			return err
		}
	}
})

func TestSignal(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	for _, d := range []time.Duration{0, time.Hour} {
		for _, s := range []os.Signal{os.Interrupt, os.Kill} {
			fmt.Println(d, s)
			c := sh.FuncCmd(sleepFunc, d, 0)
			c.Start()
			c.AwaitVars("ready")
			// Wait for a bit to allow the zero-sleep commands to exit.
			time.Sleep(100 * time.Millisecond)
			c.Signal(s)
			switch {
			case s == os.Interrupt:
				// Wait should succeed as long as the exit code was 0, regardless of
				// whether the signal arrived or the process had already exited.
				c.Wait()
			case d != 0:
				// Note: We don't call Wait in the {d: 0, s: os.Kill} case because doing
				// so makes the test flaky on slow systems.
				setsErr(t, sh, func() { c.Wait() })
			}
		}
	}

	// Signal should fail if Wait has been called.
	c := sh.FuncCmd(sleepFunc, time.Duration(0), 0)
	c.Run()
	setsErr(t, sh, func() { c.Signal(os.Interrupt) })
}

var processGroup = gosh.RegisterFunc("processGroup", func(n int) {
	pids := make([]string, n)
	for x := 0; x < n; x++ {
		c := exec.Command("sleep", "3600")
		c.Start()
		pids[x] = strconv.Itoa(c.Process.Pid)
	}
	gosh.SendVars(map[string]string{"pids": strings.Join(pids, ",")})
	time.Sleep(time.Minute)
})

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

func TestTerminate(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	for _, d := range []time.Duration{0, time.Hour} {
		for _, s := range []os.Signal{os.Interrupt, os.Kill} {
			fmt.Println(d, s)
			c := sh.FuncCmd(sleepFunc, d, 0)
			c.Start()
			c.AwaitVars("ready")
			// Wait for a bit to allow the zero-sleep commands to exit.
			time.Sleep(100 * time.Millisecond)
			// Terminate should succeed regardless of the exit code, and regardless of
			// whether the signal arrived or the process had already exited.
			c.Terminate(s)
		}
	}

	// Terminate should fail if Wait has been called.
	c := sh.FuncCmd(sleepFunc, time.Duration(0), 0)
	c.Run()
	setsErr(t, sh, func() { c.Terminate(os.Interrupt) })
}

func TestExitErrorIsOk(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Exit code 0 is not an error.
	c := sh.FuncCmd(exitFunc, 0)
	c.Run()
	ok(t, c.Err)
	ok(t, sh.Err)

	// Exit code 1 is an error.
	c = sh.FuncCmd(exitFunc, 1)
	c.ExitErrorIsOk = true
	c.Run()
	nok(t, c.Err)
	ok(t, sh.Err)

	// If ExitErrorIsOk is false, exit code 1 triggers sh.HandleError.
	c = sh.FuncCmd(exitFunc, 1)
	setsErr(t, sh, func() { c.Run() })
	nok(t, c.Err)
}

func TestIgnoreClosedPipeError(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Since writeLoopFunc will only finish if it receives a write error, it's
	// depending on the closed pipe error from closedPipeErrorWriter.
	c := sh.FuncCmd(writeLoopFunc)
	c.AddStdoutWriter(errorWriter{io.ErrClosedPipe})
	c.IgnoreClosedPipeError = true
	c.Run()
	ok(t, c.Err)
	ok(t, sh.Err)

	// Without IgnoreClosedPipeError, the command fails.
	c = sh.FuncCmd(writeLoopFunc)
	c.AddStdoutWriter(errorWriter{io.ErrClosedPipe})
	setsErr(t, sh, func() { c.Run() })
	nok(t, c.Err)
}

var writeLoopFunc = gosh.RegisterFunc("writeLoopFunc", func() error {
	for {
		if _, err := os.Stdout.Write([]byte("a\n")); err != nil {
			// Always return success; the purpose of this command is to ensure that
			// when the next command in the pipeline fails, it causes a closed pipe
			// write error here to exit the loop.
			return nil
		}
	}
})

type errorWriter struct {
	error
}

func (w errorWriter) Write(p []byte) (int, error) {
	return 0, w.error
}

var cmdFailureFunc = gosh.RegisterFunc("cmdFailureFunc", func(nStdout, nStderr int) error {
	if _, err := os.Stdout.Write([]byte(strings.Repeat("A", nStdout))); err != nil {
		return err
	}
	if _, err := os.Stderr.Write([]byte(strings.Repeat("B", nStderr))); err != nil {
		return err
	}
	time.Sleep(time.Second)
	return fakeError
})

// Tests that when a command fails, we log the head and tail of its stdout and
// stderr.
func TestCmdFailureLoggingEnabled(t *testing.T) {
	tb := &customTB{t: t, buf: &bytes.Buffer{}}
	sh := gosh.NewShell(tb)
	defer sh.Cleanup()

	const k = 1 << 15

	// Note: When a FuncCmd fails, InitMain calls log.Fatal(err), which writes err
	// to stderr. In several places below, our expected stderr must accommodate
	// this logged fakeError string.
	tests := []struct {
		nStdout    int
		nStderr    int
		wantStdout string
		wantStderr string
	}{
		{0, 0, "[ empty ]", ""},
		{1, 1, "A", "B"},
		{k, k, strings.Repeat("A", k), strings.Repeat("B", k)},
		{k + 1, k + 1, strings.Repeat("A", k+1), strings.Repeat("B", k+1)},
		// Stderr includes fakeError.
		{2 * k, 2 * k, strings.Repeat("A", 2*k), strings.Repeat("B", k) + "\n[ ... skipping "},
		// Stderr includes fakeError.
		{2*k + 1, 2*k + 1, strings.Repeat("A", k) + "\n[ ... skipping 1 bytes ... ]\n" + strings.Repeat("A", k), strings.Repeat("B", k) + "\n[ ... skipping "},
	}

	sep := strings.Repeat("-", 40)
	for _, test := range tests {
		tb.Reset()
		sh.FuncCmd(cmdFailureFunc, test.nStdout, test.nStderr).Run()
		got := tb.buf.String()
		wantStdout := fmt.Sprintf("\nSTDOUT\n%s\n%s\n", sep, test.wantStdout)
		if !strings.Contains(got, wantStdout) {
			t.Fatalf("got %v, want substring %v", got, wantStdout)
		}
		// Stderr includes fakeError.
		wantStderr := fmt.Sprintf("\nSTDERR\n%s\n%s", sep, test.wantStderr)
		if !strings.Contains(got, wantStderr) {
			t.Fatalf("got %v, want substring %v", got, wantStderr)
		}
		sh.Err = nil
	}
}

// Tests that we don't log command failures when ExitErrorIsOk or
// ContinueOnError is set.
func TestCmdFailureLoggingDisabled(t *testing.T) {
	tb := &customTB{t: t, buf: &bytes.Buffer{}}
	sh := gosh.NewShell(tb)
	defer sh.Cleanup()

	// If ExitErrorIsOk is set and the command fails, we shouldn't log anything.
	tb.Reset()
	c := sh.FuncCmd(exitFunc, 1)
	c.ExitErrorIsOk = true
	c.Run()
	eq(t, tb.calledFailNow, false)
	eq(t, tb.buf.String(), "")

	// If ContinueOnError is set and the command fails, we should log the exit
	// status but not the command stderr.
	tb.Reset()
	c = sh.FuncCmd(exitFunc, 1)
	sh.ContinueOnError = true
	c.Run()
	eq(t, tb.calledFailNow, false)
	got := tb.buf.String()
	if !strings.Contains(got, "exit status 1") {
		t.Fatalf("missing error: %s", got)
	}
	if strings.Contains(got, "STDERR") {
		t.Fatalf("should not log stderr: %s", got)
	}
}

func TestMain(m *testing.M) {
	gosh.InitMain()
	os.Exit(m.Run())
}

////////////////////////////////////////////////////////////////////////////////
// Other tests

// Tests BuildGoPkg's handling of the -o flag.
func TestBuildGoPkg(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Set -o to an absolute name.
	relName := "hw"
	absName := filepath.Join(sh.MakeTempDir(), relName)
	eq(t, gosh.BuildGoPkg(sh, "", helloWorldPkg, "-o", absName), absName)
	c := sh.Cmd(absName)
	eq(t, c.Stdout(), helloWorldStr)

	// Set -o to a relative name with no path separators.
	binDir := sh.MakeTempDir()
	absName = filepath.Join(binDir, relName)
	eq(t, gosh.BuildGoPkg(sh, binDir, helloWorldPkg, "-o", relName), absName)
	c = sh.Cmd(absName)
	eq(t, c.Stdout(), helloWorldStr)

	// Set -o to a relative name that contains a path separator.
	relNameWithSlash := filepath.Join("subdir", relName)
	absName = filepath.Join(binDir, relNameWithSlash)
	eq(t, gosh.BuildGoPkg(sh, binDir, helloWorldPkg, "-o", relNameWithSlash), absName)
	c = sh.Cmd(absName)
	eq(t, c.Stdout(), helloWorldStr)

	// Missing location after -o.
	setsErr(t, sh, func() { gosh.BuildGoPkg(sh, "", helloWorldPkg, "-o") })

	// Multiple -o.
	absName = filepath.Join(sh.MakeTempDir(), relName)
	gosh.BuildGoPkg(sh, "", helloWorldPkg, "-o", relName, "-o", absName)
	c = sh.Cmd(absName)
	eq(t, c.Stdout(), helloWorldStr)

	// Use --o instead of -o.
	absName = filepath.Join(sh.MakeTempDir(), relName)
	gosh.BuildGoPkg(sh, "", helloWorldPkg, "--o", absName)
	c = sh.Cmd(absName)
	eq(t, c.Stdout(), helloWorldStr)
}
