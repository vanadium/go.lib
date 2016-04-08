// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gosh provides facilities for running and managing processes: start
// them, wait for them to exit, capture their output streams, pipe messages
// between them, terminate them (e.g. on SIGINT), and so on.
//
// Gosh is meant to be used in situations where one might otherwise be tempted
// to write a shell script. (Oh my gosh, no more shell scripts!)
//
// For usage examples, see shell_test.go and internal/gosh_example/main.go.
package gosh

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"
	"time"
)

const (
	envExitAfter   = "GOSH_EXIT_AFTER"
	envInvocation  = "GOSH_INVOCATION"
	envWatchParent = "GOSH_WATCH_PARENT"
)

var (
	errAlreadyCalledCleanup = errors.New("gosh: already called Shell.Cleanup")
	errDidNotCallInitMain   = errors.New("gosh: did not call gosh.InitMain")
	errDidNotCallNewShell   = errors.New("gosh: did not call gosh.NewShell")
)

// TB is a subset of the testing.TB interface, defined here to avoid depending
// on the testing package.
type TB interface {
	FailNow()
	Logf(format string, args ...interface{})
}

// Shell represents a shell. Not thread-safe.
type Shell struct {
	// Err is the most recent error from this Shell or any of its child Cmds (may
	// be nil).
	Err error
	// PropagateChildOutput specifies whether to propagate child stdout and stderr
	// up to the parent's stdout and stderr.
	PropagateChildOutput bool
	// ChildOutputDir, if non-empty, makes it so child stdout and stderr are tee'd
	// to files in the specified directory.
	ChildOutputDir string
	// ContinueOnError specifies whether to invoke TB.FailNow on error, i.e.
	// whether to panic on error. Users that set ContinueOnError to true should
	// inspect sh.Err after each Shell method invocation.
	ContinueOnError bool
	// Vars is the map of env vars for this Shell.
	Vars map[string]string
	// Args is the list of args to append to subsequent command invocations.
	Args []string
	// Internal state.
	calledNewShell  bool
	tb              TB
	cleanupDone     chan struct{}
	cleanupMu       sync.Mutex // protects the fields below; held during cleanup
	calledCleanup   bool
	cmds            []*Cmd
	tempFiles       []*os.File
	tempDirs        []string
	dirStack        []string // for pushd/popd
	cleanupHandlers []func()
}

// NewShell returns a new Shell. Tests and benchmarks should pass their
// testing.TB instance; non-tests should pass nil.
func NewShell(tb TB) *Shell {
	sh, err := newShell(tb)
	sh.handleError(err)
	return sh
}

// HandleError sets sh.Err. If err is not nil and sh.ContinueOnError is false,
// it also calls TB.FailNow.
func (sh *Shell) HandleError(err error) {
	sh.HandleErrorWithSkip(err, 2)
}

// handleError is intended for use by public Shell method implementations.
func (sh *Shell) handleError(err error) {
	sh.HandleErrorWithSkip(err, 3)
}

// HandleErrorWithSkip is like HandleError, but allows clients to specify the
// skip value to pass to runtime.Caller.
func (sh *Shell) HandleErrorWithSkip(err error, skip int) {
	sh.Ok()
	sh.Err = err
	if err == nil {
		return
	}
	_, file, line, _ := runtime.Caller(skip)
	toLog := fmt.Sprintf("%s:%d: %v\n", filepath.Base(file), line, err)
	if sh.ContinueOnError {
		sh.tb.Logf(toLog)
		return
	}
	if sh.tb != pkgLevelDefaultTB {
		sh.tb.Logf(string(debug.Stack()))
	}
	// Unfortunately, if FailNow panics, there's no way to make toLog get printed
	// beneath the stack trace.
	sh.tb.Logf(toLog)
	sh.tb.FailNow()
}

// Cmd returns a Cmd for an invocation of the named program. The given arguments
// are passed to the child as command-line arguments.
func (sh *Shell) Cmd(name string, args ...string) *Cmd {
	sh.Ok()
	res, err := sh.cmd(nil, name, args...)
	sh.handleError(err)
	return res
}

// FuncCmd returns a Cmd for an invocation of the given registered Func. The
// given arguments are gob-encoded in the parent process, then gob-decoded in
// the child and passed to the Func as parameters. To specify command-line
// arguments for the child invocation, append to the returned Cmd's Args.
func (sh *Shell) FuncCmd(f *Func, args ...interface{}) *Cmd {
	sh.Ok()
	res, err := sh.funcCmd(f, args...)
	sh.handleError(err)
	return res
}

// Wait waits for all commands started by this Shell to exit.
func (sh *Shell) Wait() {
	sh.Ok()
	sh.handleError(sh.wait())
}

// Move moves a file from 'oldpath' to 'newpath'. It first attempts os.Rename;
// if that fails, it copies 'oldpath' to 'newpath', then deletes 'oldpath'.
// Requires that 'newpath' does not exist, and that the parent directory of
// 'newpath' does exist. Currently only supports moving an individual file;
// moving a directory is not yet supported.
func (sh *Shell) Move(oldpath, newpath string) {
	sh.Ok()
	sh.handleError(sh.move(oldpath, newpath))
}

// MakeTempFile creates a new temporary file in os.TempDir, opens the file for
// reading and writing, and returns the resulting *os.File.
func (sh *Shell) MakeTempFile() *os.File {
	sh.Ok()
	res, err := sh.makeTempFile()
	sh.handleError(err)
	return res
}

// MakeTempDir creates a new temporary directory in os.TempDir and returns the
// path of the new directory.
func (sh *Shell) MakeTempDir() string {
	sh.Ok()
	res, err := sh.makeTempDir()
	sh.handleError(err)
	return res
}

// Pushd behaves like Bash pushd.
func (sh *Shell) Pushd(dir string) {
	sh.Ok()
	sh.handleError(sh.pushd(dir))
}

// Popd behaves like Bash popd.
func (sh *Shell) Popd() {
	sh.Ok()
	sh.handleError(sh.popd())
}

// AddCleanupHandler registers the given function to be called during cleanup.
// Cleanup handlers are called in LIFO order, possibly in a separate goroutine
// spawned by gosh.
func (sh *Shell) AddCleanupHandler(f func()) {
	sh.Ok()
	sh.handleError(sh.addCleanupHandler(f))
}

// Cleanup cleans up all resources (child processes, temporary files and
// directories) associated with this Shell. It is safe (and recommended) to call
// Cleanup after a Shell error. It is also safe to call Cleanup multiple times;
// calls after the first return immediately with no effect. Cleanup never calls
// HandleError.
func (sh *Shell) Cleanup() {
	if !sh.calledNewShell {
		panic(errDidNotCallNewShell)
	}
	sh.cleanupMu.Lock()
	defer sh.cleanupMu.Unlock()
	if !sh.calledCleanup {
		sh.cleanup()
	}
}

// Ok panics iff this Shell is in a state where it's invalid to call other
// methods. This method is public to facilitate Shell wrapping.
func (sh *Shell) Ok() {
	if !sh.calledNewShell {
		panic(errDidNotCallNewShell)
	}
	// Panic on incorrect usage of Shell.
	if sh.Err != nil {
		panic(fmt.Errorf("gosh: Shell.Err is not nil: %v", sh.Err))
	}
	sh.cleanupMu.Lock()
	defer sh.cleanupMu.Unlock()
	if sh.calledCleanup {
		panic(errAlreadyCalledCleanup)
	}
}

////////////////////////////////////////
// Internals

type defaultTB struct{}

func (*defaultTB) FailNow() {
	panic(nil)
}

func (*defaultTB) Logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

var pkgLevelDefaultTB *defaultTB = &defaultTB{}

func newShell(tb TB) (*Shell, error) {
	if tb == nil {
		tb = pkgLevelDefaultTB
	}
	// Filter out any gosh env vars coming from outside.
	shVars := sliceToMap(os.Environ())
	for _, key := range []string{envExitAfter, envInvocation, envWatchParent} {
		delete(shVars, key)
	}
	sh := &Shell{
		Vars:           shVars,
		calledNewShell: true,
		tb:             tb,
		cleanupDone:    make(chan struct{}),
	}
	sh.cleanupOnSignal()
	return sh, nil
}

// cleanupOnSignal starts a goroutine that calls cleanup if a termination signal
// is received.
func (sh *Shell) cleanupOnSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-ch:
			// A termination signal was received; the process will exit.
			sh.tb.Logf("Received signal: %v\n", sig)
			sh.cleanupMu.Lock()
			defer sh.cleanupMu.Unlock()
			if !sh.calledCleanup {
				sh.cleanup()
			}
			// Note: We hold cleanupMu during os.Exit(1) so that the main goroutine
			// will not call Shell.Ok() and panic before we exit.
			os.Exit(1)
		case <-sh.cleanupDone:
			// The user called sh.Cleanup; stop listening for signals and exit this
			// goroutine.
		}
		signal.Stop(ch)
	}()
}

func (sh *Shell) cmd(vars map[string]string, name string, args ...string) (*Cmd, error) {
	if vars == nil {
		vars = make(map[string]string)
	}
	c, err := newCmd(sh, mergeMaps(sh.Vars, vars), name, append(args, sh.Args...)...)
	if err != nil {
		return nil, err
	}
	c.PropagateOutput = sh.PropagateChildOutput
	c.OutputDir = sh.ChildOutputDir
	return c, nil
}

var executablePath = os.Args[0]

func init() {
	// If exec.LookPath fails, hope for the best.
	if lp, err := exec.LookPath(executablePath); err != nil {
		executablePath = lp
	}
}

func (sh *Shell) funcCmd(f *Func, args ...interface{}) (*Cmd, error) {
	// Safeguard against the developer forgetting to call InitMain, which could
	// lead to infinite recursion.
	if !calledInitMain {
		return nil, errDidNotCallInitMain
	}
	buf, err := encodeInvocation(f.handle, args...)
	if err != nil {
		return nil, err
	}
	vars := map[string]string{envInvocation: string(buf)}
	return sh.cmd(vars, executablePath)
}

func (sh *Shell) wait() error {
	// Note: It is illegal to call newCmdInternal (which mutates sh.cmds)
	// concurrently with Shell.wait, so we need not hold cleanupMu when accessing
	// sh.cmds below.
	var res error
	for _, c := range sh.cmds {
		if !c.started || c.calledWait {
			continue
		}
		if err := c.wait(); !c.errorIsOk(err) {
			sh.tb.Logf("%s (PID %d) failed: %v\n", c.Path, c.Pid(), err)
			res = err
		}
	}
	return res
}

func copyFile(to, from string) error {
	fi, err := os.Stat(from)
	if err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode().Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}

func (sh *Shell) move(oldpath, newpath string) error {
	fi, err := os.Stat(oldpath)
	if err != nil {
		return err
	}
	if fi.Mode().IsDir() {
		return errors.New("gosh: moving a directory is not yet supported")
	}
	if _, err := os.Stat(newpath); !os.IsNotExist(err) {
		return errors.New("gosh: destination file must not exist")
	}
	if _, err := os.Stat(filepath.Dir(newpath)); err != nil {
		if os.IsNotExist(err) {
			return errors.New("gosh: destination file's parent directory must exist")
		}
		return err
	}
	if err := os.Rename(oldpath, newpath); err == nil {
		return nil
	}
	// Concurrent, same-directory rename operations sometimes fail on certain
	// systems, so we retry once after a random backoff.
	time.Sleep(time.Duration(rand.Int63n(1000)) * time.Millisecond)
	if err := os.Rename(oldpath, newpath); err == nil {
		return nil
	}
	// Try copying the file over.
	if err := copyFile(newpath, oldpath); err != nil {
		return err
	}
	return os.Remove(oldpath)
}

func (sh *Shell) makeTempFile() (*os.File, error) {
	sh.cleanupMu.Lock()
	defer sh.cleanupMu.Unlock()
	if sh.calledCleanup {
		return nil, errAlreadyCalledCleanup
	}
	f, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}
	sh.tempFiles = append(sh.tempFiles, f)
	return f, nil
}

func (sh *Shell) makeTempDir() (string, error) {
	sh.cleanupMu.Lock()
	defer sh.cleanupMu.Unlock()
	if sh.calledCleanup {
		return "", errAlreadyCalledCleanup
	}
	name, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}
	sh.tempDirs = append(sh.tempDirs, name)
	return name, nil
}

func (sh *Shell) pushd(dir string) error {
	sh.cleanupMu.Lock()
	defer sh.cleanupMu.Unlock()
	if sh.calledCleanup {
		return errAlreadyCalledCleanup
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	sh.dirStack = append(sh.dirStack, cwd)
	return nil
}

func (sh *Shell) popd() error {
	sh.cleanupMu.Lock()
	defer sh.cleanupMu.Unlock()
	if sh.calledCleanup {
		return errAlreadyCalledCleanup
	}
	if len(sh.dirStack) == 0 {
		return errors.New("gosh: dir stack is empty")
	}
	dir := sh.dirStack[len(sh.dirStack)-1]
	if err := os.Chdir(dir); err != nil {
		return err
	}
	sh.dirStack = sh.dirStack[:len(sh.dirStack)-1]
	return nil
}

func (sh *Shell) addCleanupHandler(f func()) error {
	sh.cleanupMu.Lock()
	defer sh.cleanupMu.Unlock()
	if sh.calledCleanup {
		return errAlreadyCalledCleanup
	}
	sh.cleanupHandlers = append(sh.cleanupHandlers, f)
	return nil
}

// Note: It is safe to run Shell.cleanupRunningCmds concurrently with the waiter
// goroutine and with Cmd.wait. In particular, Shell.cleanupRunningCmds only
// calls c.{isRunning,Pid}, all of which are thread-safe with the waiter
// goroutine and with Cmd.wait.
func (sh *Shell) cleanupRunningCmds() {
	var wg sync.WaitGroup
	for _, c := range sh.cmds {
		if !c.started {
			continue
		}
		wg.Add(1)
		go func(cmd *Cmd) {
			defer wg.Done()
			cmd.cleanupProcessGroup()
		}(c)
	}
	wg.Wait()
}

func (sh *Shell) cleanup() {
	sh.calledCleanup = true
	// Clean up all children that are still running.
	sh.cleanupRunningCmds()
	// Close and delete all temporary files.
	for _, tempFile := range sh.tempFiles {
		name := tempFile.Name()
		if err := tempFile.Close(); err != nil {
			sh.tb.Logf("%q.Close() failed: %v\n", name, err)
		}
		if err := os.RemoveAll(name); err != nil {
			sh.tb.Logf("os.RemoveAll(%q) failed: %v\n", name, err)
		}
	}
	// Delete all temporary directories.
	for _, tempDir := range sh.tempDirs {
		if err := os.RemoveAll(tempDir); err != nil {
			sh.tb.Logf("os.RemoveAll(%q) failed: %v\n", tempDir, err)
		}
	}
	// Change back to the top of the dir stack.
	if len(sh.dirStack) > 0 {
		dir := sh.dirStack[0]
		if err := os.Chdir(dir); err != nil {
			sh.tb.Logf("os.Chdir(%q) failed: %v\n", dir, err)
		}
	}
	// Call cleanup handlers in LIFO order.
	for i := len(sh.cleanupHandlers) - 1; i >= 0; i-- {
		sh.cleanupHandlers[i]()
	}
	close(sh.cleanupDone)
}

////////////////////////////////////////
// Public utilities

var calledInitMain = false

// InitMain must be called early on in main(), before flags are parsed. In the
// parent process, it returns immediately with no effect. In a child process for
// a Shell.FuncCmd command, it runs the specified function, then exits.
func InitMain() {
	if calledInitMain {
		panic("gosh: already called gosh.InitMain")
	}
	calledInitMain = true
	s := os.Getenv(envInvocation)
	if s == "" {
		return
	}
	os.Unsetenv(envInvocation)
	InitChildMain()
	name, args, err := decodeInvocation(s)
	if err != nil {
		log.Fatal(err)
	}
	if err := callFunc(name, args...); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}

// BuildGoPkg compiles a Go package using the "go build" command and writes the
// resulting binary to the given binDir, or to the -o flag location if
// specified. If -o is relative, it is interpreted as relative to binDir. If the
// binary already exists at the target location, it is not rebuilt. Returns the
// absolute path to the binary.
func BuildGoPkg(sh *Shell, binDir, pkg string, flags ...string) string {
	sh.Ok()
	res, err := buildGoPkg(sh, binDir, pkg, flags...)
	sh.handleError(err)
	return res
}

func extractOutputFlag(flags ...string) (outputFlag string, otherFlags []string, err error) {
	for i := 0; i < len(flags); i++ {
		v := flags[i]
		if v == "-o" || v == "--o" {
			i++
			if i == len(flags) {
				return "", nil, errors.New("gosh: passed -o without location")
			}
			outputFlag = flags[i]
		} else {
			otherFlags = append(otherFlags, v)
		}
	}
	return
}

func buildGoPkg(sh *Shell, binDir, pkg string, flags ...string) (string, error) {
	outputFlag, flags, err := extractOutputFlag(flags...)
	if err != nil {
		return "", err
	}
	var binPath string
	if outputFlag == "" {
		binPath = filepath.Join(binDir, path.Base(pkg))
	} else if filepath.IsAbs(outputFlag) {
		binPath = outputFlag
	} else {
		binPath = filepath.Join(binDir, outputFlag)
	}
	// If the binary already exists at the target location, don't rebuild it.
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	// Build binary to tempBinPath (in a fresh temporary directory), then move it
	// to binPath.
	tempDir, err := ioutil.TempDir(binDir, "")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)
	tempBinPath := filepath.Join(tempDir, path.Base(pkg))
	args := []string{"build", "-o", tempBinPath}
	args = append(args, flags...)
	args = append(args, pkg)
	c, err := sh.cmd(nil, "go", args...)
	if err != nil {
		return "", err
	}
	if err := c.run(); err != nil {
		return "", err
	}
	// Create target directory, if needed.
	if err := os.MkdirAll(filepath.Dir(binPath), 0700); err != nil {
		return "", err
	}
	if err := sh.move(tempBinPath, binPath); err != nil {
		return "", err
	}
	sh.tb.Logf("Built executable: %s\n", binPath)
	return binPath, nil
}
