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
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	envBinDir         = "GOSH_BIN_DIR"
	envChildOutputDir = "GOSH_CHILD_OUTPUT_DIR"
	envInvocation     = "GOSH_INVOCATION"
	envSpawnedByShell = "GOSH_SPAWNED_BY_SHELL"
)

var (
	errAlreadyCalledCleanup        = errors.New("gosh: already called Shell.Cleanup")
	errDidNotCallMaybeRunFnAndExit = errors.New("gosh: did not call Shell.MaybeRunFnAndExit")
	errDidNotCallNewShell          = errors.New("gosh: did not call gosh.NewShell")
)

// Shell represents a shell. Not thread-safe.
type Shell struct {
	// Err is the most recent error from this Shell or any of its Cmds (may be
	// nil).
	Err error
	// Opts is the Opts struct for this Shell, with default values filled in.
	Opts Opts
	// Vars is the map of env vars for this Shell.
	Vars map[string]string
	// Args is the list of args to append to subsequent command invocations.
	Args []string
	// Internal state.
	calledNewShell bool
	dirStack       []string   // for pushd/popd
	cleanupMu      sync.Mutex // protects the fields below; held during cleanup
	calledCleanup  bool
	cmds           []*Cmd
	tempFiles      []*os.File
	tempDirs       []string
	cleanupFns     []func()
}

// Opts configures Shell.
type Opts struct {
	// Fatalf is called whenever an error is encountered.
	// If not specified, defaults to panic(fmt.Sprintf(format, v...)).
	Fatalf func(format string, v ...interface{})
	// Logf is called to log things.
	// If not specified, defaults to log.Printf(format, v...).
	Logf func(format string, v ...interface{})
	// Child stdout and stderr are propagated up to the parent's stdout and stderr
	// iff PropagateChildOutput is true.
	PropagateChildOutput bool
	// If specified, each child's stdout and stderr streams are also piped to
	// files in this directory.
	// If not specified, defaults to GOSH_CHILD_OUTPUT_DIR.
	ChildOutputDir string
	// Directory where BuildGoPkg() writes compiled binaries.
	// If not specified, defaults to GOSH_BIN_DIR.
	BinDir string
}

// NewShell returns a new Shell.
func NewShell(opts Opts) *Shell {
	sh, err := newShell(opts)
	sh.HandleError(err)
	return sh
}

// HandleError sets sh.Err. If err is not nil, it also calls sh.Opts.Fatalf.
func (sh *Shell) HandleError(err error) {
	sh.Ok()
	sh.Err = err
	if err != nil && sh.Opts.Fatalf != nil {
		sh.Opts.Fatalf("%v", err)
	}
}

// Cmd returns a Cmd for an invocation of the named program.
func (sh *Shell) Cmd(name string, args ...string) *Cmd {
	sh.Ok()
	res, err := sh.cmd(nil, name, args...)
	sh.HandleError(err)
	return res
}

// Fn returns a Cmd for an invocation of the given registered Fn.
func (sh *Shell) Fn(fn *Fn, args ...interface{}) *Cmd {
	sh.Ok()
	res, err := sh.fn(fn, args...)
	sh.HandleError(err)
	return res
}

// Main returns a Cmd for an invocation of the given registered main() function.
// Intended usage: Have your program's main() call RealMain, then write a parent
// program that uses Shell.Main to run RealMain in a child process. With this
// approach, RealMain can be compiled into the parent program's binary. Caveat:
// potential flag collisions.
func (sh *Shell) Main(fn *Fn, args ...string) *Cmd {
	sh.Ok()
	res, err := sh.main(fn, args...)
	sh.HandleError(err)
	return res
}

// Wait waits for all commands started by this Shell to exit.
func (sh *Shell) Wait() {
	sh.Ok()
	sh.HandleError(sh.wait())
}

// Rename renames (moves) a file. It's just like os.Rename, but retries once on
// error.
func (sh *Shell) Rename(oldpath, newpath string) {
	sh.Ok()
	sh.HandleError(sh.rename(oldpath, newpath))
}

// BuildGoPkg compiles a Go package using the "go build" command and writes the
// resulting binary to sh.Opts.BinDir. Returns the absolute path to the binary.
// Included in Shell for convenience, but could have just as easily been
// provided as a utility function.
func (sh *Shell) BuildGoPkg(pkg string, flags ...string) string {
	sh.Ok()
	res, err := sh.buildGoPkg(pkg, flags...)
	sh.HandleError(err)
	return res
}

// MakeTempFile creates a new temporary file in os.TempDir, opens the file for
// reading and writing, and returns the resulting *os.File.
func (sh *Shell) MakeTempFile() *os.File {
	sh.Ok()
	res, err := sh.makeTempFile()
	sh.HandleError(err)
	return res
}

// MakeTempDir creates a new temporary directory in os.TempDir and returns the
// path of the new directory.
func (sh *Shell) MakeTempDir() string {
	sh.Ok()
	res, err := sh.makeTempDir()
	sh.HandleError(err)
	return res
}

// Pushd behaves like Bash pushd.
func (sh *Shell) Pushd(dir string) {
	sh.Ok()
	sh.HandleError(sh.pushd(dir))
}

// Popd behaves like Bash popd.
func (sh *Shell) Popd() {
	sh.Ok()
	sh.HandleError(sh.popd())
}

// AddToCleanup registers the given function to be called by Shell.Cleanup().
func (sh *Shell) AddToCleanup(fn func()) {
	sh.Ok()
	sh.HandleError(sh.addToCleanup(fn))
}

// Cleanup cleans up all resources (child processes, temporary files and
// directories) associated with this Shell. It is safe (and recommended) to call
// Cleanup after a Shell error. It is also safe to call Cleanup multiple times;
// calls after the first return immediately with no effect.
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

// onTerminationSignal starts a goroutine that listens for various termination
// signals and calls the given function when such a signal is received.
func onTerminationSignal(fn func(os.Signal)) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		fn(<-ch)
	}()
}

// Note: On error, newShell returns a *Shell with Opts.Fatalf initialized to
// simplify things for the caller.
func newShell(opts Opts) (*Shell, error) {
	if opts.Fatalf == nil {
		opts.Fatalf = func(format string, v ...interface{}) {
			panic(fmt.Sprintf(format, v...))
		}
	}
	if opts.Logf == nil {
		opts.Logf = func(format string, v ...interface{}) {
			log.Printf(format, v...)
		}
	}
	if opts.ChildOutputDir == "" {
		opts.ChildOutputDir = os.Getenv(envChildOutputDir)
	}
	sh := &Shell{
		Opts:           opts,
		Vars:           map[string]string{},
		calledNewShell: true,
	}
	if sh.Opts.BinDir == "" {
		sh.Opts.BinDir = os.Getenv(envBinDir)
		if sh.Opts.BinDir == "" {
			var err error
			if sh.Opts.BinDir, err = sh.makeTempDir(); err != nil {
				sh.cleanup()
				return sh, err
			}
		}
	}
	// Call sh.cleanup() if needed when a termination signal is received.
	onTerminationSignal(func(sig os.Signal) {
		sh.logf("Received signal: %v\n", sig)
		sh.cleanupMu.Lock()
		defer sh.cleanupMu.Unlock()
		if !sh.calledCleanup {
			sh.cleanup()
		}
		// Note: We hold cleanupMu during os.Exit(1) so that the main goroutine will
		// not call Shell.Ok() and panic before we exit.
		os.Exit(1)
	})
	return sh, nil
}

func (sh *Shell) logf(format string, v ...interface{}) {
	if sh.Opts.Logf != nil {
		sh.Opts.Logf(format, v...)
	}
}

func (sh *Shell) cmd(vars map[string]string, name string, args ...string) (*Cmd, error) {
	if vars == nil {
		vars = make(map[string]string)
	}
	vars[envSpawnedByShell] = "1"
	c, err := newCmd(sh, mergeMaps(sliceToMap(os.Environ()), sh.Vars, vars), name, append(args, sh.Args...)...)
	if err != nil {
		return nil, err
	}
	c.PropagateOutput = sh.Opts.PropagateChildOutput
	c.OutputDir = sh.Opts.ChildOutputDir
	return c, nil
}

func (sh *Shell) fn(fn *Fn, args ...interface{}) (*Cmd, error) {
	// Safeguard against the developer forgetting to call MaybeRunFnAndExit, which
	// could lead to infinite recursion.
	if !calledMaybeRunFnAndExit {
		return nil, errDidNotCallMaybeRunFnAndExit
	}
	b, err := encInvocation(fn.name, args...)
	if err != nil {
		return nil, err
	}
	vars := map[string]string{envInvocation: string(b)}
	return sh.cmd(vars, os.Args[0])
}

func (sh *Shell) main(fn *Fn, args ...string) (*Cmd, error) {
	// Safeguard against the developer forgetting to call MaybeRunFnAndExit, which
	// could lead to infinite recursion.
	if !calledMaybeRunFnAndExit {
		return nil, errDidNotCallMaybeRunFnAndExit
	}
	// Check that fn has the required signature.
	t := fn.value.Type()
	if t.NumIn() != 0 || t.NumOut() != 0 {
		return nil, errors.New("gosh: main function must have no input or output parameters")
	}
	b, err := encInvocation(fn.name)
	if err != nil {
		return nil, err
	}
	vars := map[string]string{envInvocation: string(b)}
	return sh.cmd(vars, os.Args[0], args...)
}

func (sh *Shell) wait() error {
	// Note: It is illegal to call newCmd() concurrently with Shell.wait(), so we
	// need not hold cleanupMu when accessing sh.cmds below.
	var res error
	for _, c := range sh.cmds {
		if !c.started || c.calledWait {
			continue
		}
		if err := c.wait(); !c.errorIsOk(err) {
			sh.logf("%s (PID %d) failed: %v\n", c.Path, c.Pid(), err)
			res = err
		}
	}
	return res
}

func (sh *Shell) rename(oldpath, newpath string) error {
	if err := os.Rename(oldpath, newpath); err != nil {
		// Concurrent, same-directory rename operations sometimes fail on certain
		// filesystems, so we retry once after a random backoff.
		time.Sleep(time.Duration(rand.Int63n(1000)) * time.Millisecond)
		if err := os.Rename(oldpath, newpath); err != nil {
			return err
		}
	}
	return nil
}

func (sh *Shell) buildGoPkg(pkg string, flags ...string) (string, error) {
	binPath := filepath.Join(sh.Opts.BinDir, path.Base(pkg))
	// If this binary has already been built, don't rebuild it.
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	// Build binary to tempBinPath, then move it to binPath.
	tempDir, err := ioutil.TempDir(sh.Opts.BinDir, "")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)
	tempBinPath := filepath.Join(tempDir, path.Base(pkg))
	args := []string{"build", "-x", "-o", tempBinPath}
	args = append(args, flags...)
	args = append(args, pkg)
	c, err := sh.cmd(nil, "go", args...)
	if err != nil {
		return "", err
	}
	c.PropagateOutput = false
	if err := c.run(); err != nil {
		return "", err
	}
	if err := sh.rename(tempBinPath, binPath); err != nil {
		return "", err
	}
	return binPath, nil
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

func (sh *Shell) addToCleanup(fn func()) error {
	sh.cleanupMu.Lock()
	defer sh.cleanupMu.Unlock()
	if sh.calledCleanup {
		return errAlreadyCalledCleanup
	}
	sh.cleanupFns = append(sh.cleanupFns, fn)
	return nil
}

// forEachRunningCmd applies fn to each running child process.
func (sh *Shell) forEachRunningCmd(fn func(*Cmd)) bool {
	anyRunning := false
	for _, c := range sh.cmds {
		if c.isRunning() {
			anyRunning = true
			if fn != nil {
				fn(c)
			}
		}
	}
	return anyRunning
}

// Note: It is safe to run Shell.terminateRunningCmds concurrently with the
// waiter goroutine and with Cmd.wait. In particular, Shell.terminateRunningCmds
// only calls c.{isRunning,Pid,Signal,Kill}, all of which are thread-safe with
// the waiter goroutine and with Cmd.wait.
func (sh *Shell) terminateRunningCmds() {
	// Try Cmd.signal first; if that doesn't work, use Cmd.kill.
	anyRunning := sh.forEachRunningCmd(func(c *Cmd) {
		if err := c.signal(os.Interrupt); err != nil {
			sh.logf("%d.Signal(os.Interrupt) failed: %v\n", c.Pid(), err)
		}
	})
	// If any child is still running, wait for 100ms.
	if anyRunning {
		time.Sleep(100 * time.Millisecond)
		anyRunning = sh.forEachRunningCmd(func(c *Cmd) {
			sh.logf("%s (PID %d) did not die\n", c.Path, c.Pid())
		})
	}
	// If any child is still running, wait for another second, then call Cmd.kill
	// for all running children.
	if anyRunning {
		time.Sleep(time.Second)
		sh.forEachRunningCmd(func(c *Cmd) {
			if err := c.kill(); err != nil {
				sh.logf("%d.Kill() failed: %v\n", c.Pid(), err)
			}
		})
		sh.logf("Killed all remaining child processes\n")
	}
}

func (sh *Shell) cleanup() {
	sh.calledCleanup = true
	// Terminate all children that are still running.
	sh.terminateRunningCmds()
	// Close and delete all temporary files.
	for _, tempFile := range sh.tempFiles {
		name := tempFile.Name()
		if err := tempFile.Close(); err != nil {
			sh.logf("%q.Close() failed: %v\n", name, err)
		}
		if err := os.RemoveAll(name); err != nil {
			sh.logf("os.RemoveAll(%q) failed: %v\n", name, err)
		}
	}
	// Delete all temporary directories.
	for _, tempDir := range sh.tempDirs {
		if err := os.RemoveAll(tempDir); err != nil {
			sh.logf("os.RemoveAll(%q) failed: %v\n", tempDir, err)
		}
	}
	// Change back to the top of the dir stack.
	if len(sh.dirStack) > 0 {
		dir := sh.dirStack[0]
		if err := os.Chdir(dir); err != nil {
			sh.logf("os.Chdir(%q) failed: %v\n", dir, err)
		}
	}
	// Call any registered cleanup functions in LIFO order.
	for i := len(sh.cleanupFns) - 1; i >= 0; i-- {
		sh.cleanupFns[i]()
	}
}

////////////////////////////////////////
// Public utilities

var calledMaybeRunFnAndExit = false

// MaybeRunFnAndExit must be called first thing in main() or TestMain(), before
// flags are parsed. In the parent process, it returns immediately with no
// effect. In a child process for a Shell.Fn() or Shell.Main() command, it runs
// the specified function, then exits.
func MaybeRunFnAndExit() {
	calledMaybeRunFnAndExit = true
	s := os.Getenv(envInvocation)
	if s == "" {
		return
	}
	os.Unsetenv(envInvocation)
	// Call MaybeWatchParent rather than WatchParent so that envSpawnedByShell
	// gets cleared.
	MaybeWatchParent()
	name, args, err := decInvocation(s)
	if err != nil {
		log.Fatal(err)
	}
	if err := Call(name, args...); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}

// Run calls MaybeRunFnAndExit(), then returns run(). Exported so that TestMain
// functions can simply call os.Exit(gosh.Run(m.Run)).
func Run(run func() int) int {
	MaybeRunFnAndExit()
	return run()
}
