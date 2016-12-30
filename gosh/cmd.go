// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"v.io/x/lib/lookpath"
)

var (
	errAlreadyCalledStart = errors.New("gosh: already called Cmd.Start")
	errAlreadyCalledWait  = errors.New("gosh: already called Cmd.Wait")
	errAlreadySetStdin    = errors.New("gosh: already set stdin")
	errDidNotCallStart    = errors.New("gosh: did not call Cmd.Start")
	errProcessExited      = errors.New("gosh: process exited")
)

// Cmd represents a command. Not thread-safe.
// Public fields should not be modified after calling Start.
type Cmd struct {
	// Err is the most recent error from this Cmd (may be nil).
	Err error
	// Path is the path of the command to run.
	Path string
	// Vars is the map of env vars for this Cmd.
	Vars map[string]string
	// Args is the list of args for this Cmd, starting with the resolved path.
	// Note, we set Args[0] to the resolved path (rather than the user-specified
	// name) so that a command started by Shell can reliably determine the path to
	// its executable.
	Args []string
	// IgnoreParentExit, if true, makes it so the child process does not exit when
	// its parent exits. Only takes effect if the child process was spawned via
	// Shell.FuncCmd or explicitly calls InitChildMain.
	IgnoreParentExit bool
	// ExitAfter, if non-zero, specifies that the child process should exit after
	// the given duration has elapsed. Only takes effect if the child process was
	// spawned via Shell.FuncCmd or explicitly calls InitChildMain.
	ExitAfter time.Duration
	// PropagateOutput is inherited from Shell.PropagateChildOutput.
	PropagateOutput bool
	// OutputDir is inherited from Shell.ChildOutputDir.
	OutputDir string
	// ExitErrorIsOk specifies whether an *exec.ExitError should be reported via
	// Shell.HandleError.
	ExitErrorIsOk bool
	// IgnoreClosedPipeError, if true, causes errors from read/write on a closed
	// pipe to be indistinguishable from success. These errors often occur in
	// command pipelines, e.g. "yes | head -1", where "yes" will receive a closed
	// pipe error when it tries to write on stdout, after "head" has exited. If a
	// closed pipe error occurs, Cmd.Err will be nil, and no err is reported to
	// Shell.HandleError.
	IgnoreClosedPipeError bool
	// ExtraFiles is used to populate ExtraFiles in the underlying exec.Cmd
	// object. Does not get cloned.
	ExtraFiles []*os.File
	// Internal state.
	sh                *Shell
	c                 *exec.Cmd
	calledStart       bool
	calledWait        bool
	cond              *sync.Cond
	waitChan          chan error
	stdinDoneChan     chan error
	started           bool // protected by sh.cleanupMu
	exited            bool // protected by cond.L
	calledCleanup     bool // protected by cleanupMu
	cleanupMu         sync.Mutex
	stdoutHeadTail    *headTail
	stderrHeadTail    *headTail
	stdoutWriters     []io.Writer
	stderrWriters     []io.Writer
	afterStartClosers []io.Closer
	afterWaitClosers  []io.Closer
	recvVars          map[string]string // protected by cond.L
}

// Shell returns the shell that this Cmd was created from.
func (c *Cmd) Shell() *Shell {
	return c.sh
}

// Clone returns a new Cmd with a copy of this Cmd's configuration.
func (c *Cmd) Clone() *Cmd {
	c.sh.Ok()
	res, err := c.clone()
	c.handleError(err)
	return res
}

// StdinPipe returns a WriteCloser backed by an unlimited-size pipe for the
// command's stdin. The pipe will be closed when the process exits, but may also
// be closed earlier by the caller, e.g. if the command does not exit until its
// stdin is closed. Must be called before Start. Only one call may be made to
// StdinPipe or SetStdinReader; subsequent calls will fail.
func (c *Cmd) StdinPipe() io.WriteCloser {
	c.sh.Ok()
	res, err := c.stdinPipe()
	c.handleError(err)
	return res
}

// StdoutPipe returns a ReadCloser backed by an unlimited-size pipe for the
// command's stdout. The pipe will be closed when the process exits, but may
// also be closed earlier by the caller, e.g. if all expected output has been
// received. Must be called before Start. May be called more than once; each
// call creates a new pipe.
func (c *Cmd) StdoutPipe() io.ReadCloser {
	c.sh.Ok()
	res, err := c.stdoutPipe()
	c.handleError(err)
	return res
}

// StderrPipe returns a ReadCloser backed by an unlimited-size pipe for the
// command's stderr. The pipe will be closed when the process exits, but may
// also be closed earlier by the caller, e.g. if all expected output has been
// received. Must be called before Start. May be called more than once; each
// call creates a new pipe.
func (c *Cmd) StderrPipe() io.ReadCloser {
	c.sh.Ok()
	res, err := c.stderrPipe()
	c.handleError(err)
	return res
}

// SetStdinReader configures this Cmd to read stdin from the given Reader. Must
// be called before Start. Only one call may be made to StdinPipe or
// SetStdinReader; subsequent calls will fail.
func (c *Cmd) SetStdinReader(r io.Reader) {
	c.sh.Ok()
	c.handleError(c.setStdinReader(r))
}

// AddStdoutWriter configures this Cmd to tee stdout to the given Writer. Must
// be called before Start. If the same Writer is passed to both AddStdoutWriter
// and AddStderrWriter, Cmd will ensure that Write is never called concurrently.
func (c *Cmd) AddStdoutWriter(w io.Writer) {
	c.sh.Ok()
	c.handleError(c.addStdoutWriter(w))
}

// AddStderrWriter configures this Cmd to tee stderr to the given Writer. Must
// be called before Start. If the same Writer is passed to both AddStdoutWriter
// and AddStderrWriter, Cmd will ensure that Write is never called concurrently.
func (c *Cmd) AddStderrWriter(w io.Writer) {
	c.sh.Ok()
	c.handleError(c.addStderrWriter(w))
}

// Start starts the command.
func (c *Cmd) Start() {
	c.sh.Ok()
	c.handleError(c.start())
}

// AwaitVars waits for the child process to send values for the given vars
// (e.g. using SendVars). Must not be called before Start or after Wait.
func (c *Cmd) AwaitVars(keys ...string) map[string]string {
	c.sh.Ok()
	res, err := c.awaitVars(keys...)
	c.handleError(err)
	return res
}

// Wait waits for the command to exit.
func (c *Cmd) Wait() {
	c.sh.Ok()
	c.handleError(c.wait())
}

// Signal sends a signal to the underlying process.
func (c *Cmd) Signal(sig os.Signal) {
	c.sh.Ok()
	c.handleError(c.signal(sig))
}

// Terminate sends a signal to the underlying process, then waits for it to
// exit. Terminate is different from Signal followed by Wait: Terminate succeeds
// as long as the process exits, whereas Wait fails if the exit code isn't 0.
func (c *Cmd) Terminate(sig os.Signal) {
	c.sh.Ok()
	c.handleError(c.terminate(sig))
}

// Run calls Start followed by Wait.
func (c *Cmd) Run() {
	c.sh.Ok()
	c.handleError(c.run())
}

// Stdout calls Start followed by Wait, then returns the command's stdout.
func (c *Cmd) Stdout() string {
	c.sh.Ok()
	res, err := c.stdout()
	c.handleError(err)
	return res
}

// StdoutStderr calls Start followed by Wait, then returns the command's stdout
// and stderr.
func (c *Cmd) StdoutStderr() (string, string) {
	c.sh.Ok()
	stdout, stderr, err := c.stdoutStderr()
	c.handleError(err)
	return stdout, stderr
}

// CombinedOutput calls Start followed by Wait, then returns the command's
// combined stdout and stderr.
func (c *Cmd) CombinedOutput() string {
	c.sh.Ok()
	res, err := c.combinedOutput()
	c.handleError(err)
	return res
}

// Pid returns the command's PID, or -1 if the command has not been started.
func (c *Cmd) Pid() int {
	if !c.started {
		return -1
	}
	return c.c.Process.Pid
}

////////////////////////////////////////
// Internals

const headTailCapacity = 1 << 15

func newCmdInternal(sh *Shell, vars map[string]string, path string, args []string) (*Cmd, error) {
	c := &Cmd{
		Path:           path,
		Vars:           vars,
		Args:           append([]string{path}, args...),
		sh:             sh,
		c:              &exec.Cmd{},
		cond:           sync.NewCond(&sync.Mutex{}),
		waitChan:       make(chan error, 1),
		stdoutHeadTail: newHeadTail(headTailCapacity),
		stderrHeadTail: newHeadTail(headTailCapacity),
		recvVars:       map[string]string{},
	}
	// Protect against concurrent signal-triggered Shell.cleanup().
	sh.cleanupMu.Lock()
	defer sh.cleanupMu.Unlock()
	if sh.calledCleanup {
		return nil, errAlreadyCalledCleanup
	}
	sh.cmds = append(sh.cmds, c)
	return c, nil
}

func newCmd(sh *Shell, vars map[string]string, name string, args ...string) (*Cmd, error) {
	// Mimics https://golang.org/src/os/exec/exec.go Command.
	if filepath.Base(name) == name {
		lp, err := lookpath.Look(sh.Vars, name)
		if err != nil {
			return nil, fmt.Errorf("gosh: failed to locate executable: %s", name)
		}
		name = lp
	}
	return newCmdInternal(sh, vars, name, args)
}

func isExitError(err error) bool {
	_, ok := err.(*exec.ExitError)
	return ok
}

func (c *Cmd) errorIsOk(err error) bool {
	return err == nil || c.ExitErrorIsOk && isExitError(err)
}

// An explanation of closed pipe errors. Consider the pipeline "yes | head -1",
// where yes keeps writing "y\n" to stdout, and head succeeds after it reads the
// first line. There is an os pipe connecting the two commands, and when head
// exits, it causes yes to receive a closed pipe error on its next write. Should
// we consider such a pipeline to have succeeded or failed?
//
// Bash only looks at the exit status of the last command by default, thus the
// pipeline succeeds. But that's dangerous, since yes could have crashed and we
// wouldn't know. It's recommended to run "set -o pipefail" to tell bash to
// check the exit status of all commands. But that causes the pipeline to fail.
//
// IgnoreClosedPipeError handles this case. gosh.Pipeline sets this option to
// true, so that by default the pipeline above will succeed, but will fail on
// any other error. Note that the exec package always returns an ExitError if
// the child process exited with a non-zero exit code, or died due to a signal;
// io.ErrClosedPipe and os.PathError are only returned if the child process
// exited with a zero exit code, and Write on the io.MultiWriter in the parent
// process received the closed pipe error.
//
// Starting in go 1.6, by default all go programs will exit with SIGPIPE if they
// try to write to a broken os.Stdout or os.Stderr. This is the behavior we
// want; it means that normal go programs will behave as expected wrt
// IgnoreClosedPipeError, and gosh.Pipeline will work by default.

var sep = strings.Repeat("-", 40)

func (c *Cmd) handleError(err error) {
	if c.IgnoreClosedPipeError && isClosedPipeError(err) {
		err = nil
	}
	c.Err = err
	if c.errorIsOk(err) {
		err = nil
	}
	if isExitError(err) && !c.sh.ContinueOnError {
		c.sh.tb.Logf("gosh: command failed: %s\n", strings.Join(c.Args, " "))
		c.sh.tb.Logf("\nSTDOUT\n%s\n%s\n", sep, c.stdoutHeadTail.String())
		c.sh.tb.Logf("\nSTDERR\n%s\n%s\n", sep, c.stderrHeadTail.String())
	}
	c.sh.HandleErrorWithSkip(err, c.sh.ErrorDepth+1)
}

func (c *Cmd) isRunning() bool {
	if !c.started {
		return false
	}
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	return !c.exited
}

// recvWriter listens for gosh vars from a child process.
type recvWriter struct {
	c             *Cmd
	buf           []byte
	matchedPrefix int
	matchedSuffix int
}

func (w *recvWriter) Write(p []byte) (n int, err error) {
	for i, b := range p {
		if w.matchedPrefix < len(varsPrefix) {
			// Look for matching prefix.
			if b != varsPrefix[w.matchedPrefix] {
				w.matchedPrefix = 0
			}
			if b == varsPrefix[w.matchedPrefix] {
				w.matchedPrefix++
			}
			continue
		}
		w.buf = append(w.buf, b)
		// Look for matching suffix.
		if b != varsSuffix[w.matchedSuffix] {
			w.matchedSuffix = 0
		}
		if b == varsSuffix[w.matchedSuffix] {
			w.matchedSuffix++
		}
		if w.matchedSuffix != len(varsSuffix) {
			continue
		}
		// Found matching suffix.
		data := w.buf[:len(w.buf)-len(varsSuffix)]
		w.buf = w.buf[:0]
		w.matchedPrefix, w.matchedSuffix = 0, 0
		vars := make(map[string]string)
		if err := json.Unmarshal(data, &vars); err != nil {
			return i, err
		}
		w.c.cond.L.Lock()
		w.c.recvVars = mergeMaps(w.c.recvVars, vars)
		w.c.cond.Signal()
		w.c.cond.L.Unlock()
	}
	return len(p), nil
}

func (c *Cmd) makeStdoutStderr() (io.Writer, io.Writer, error) {
	c.stderrWriters = append(c.stderrWriters, &recvWriter{c: c})
	c.stdoutWriters = append(c.stdoutWriters, c.stdoutHeadTail)
	c.stderrWriters = append(c.stderrWriters, c.stderrHeadTail)
	if c.PropagateOutput {
		c.stdoutWriters = append(c.stdoutWriters, os.Stdout)
		c.stderrWriters = append(c.stderrWriters, os.Stderr)
	}
	if c.OutputDir != "" {
		t := time.Now().Format("20060102.150405.000000")
		name := filepath.Join(c.OutputDir, filepath.Base(c.Path)+"."+t)
		const flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
		switch file, err := os.OpenFile(name+".stdout", flags, 0600); {
		case err != nil:
			return nil, nil, err
		default:
			c.stdoutWriters = append(c.stdoutWriters, file)
			c.afterWaitClosers = append(c.afterWaitClosers, file)
		}
		switch file, err := os.OpenFile(name+".stderr", flags, 0600); {
		case err != nil:
			return nil, nil, err
		default:
			c.stderrWriters = append(c.stderrWriters, file)
			c.afterWaitClosers = append(c.afterWaitClosers, file)
		}
	}
	switch hasOut, hasErr := len(c.stdoutWriters) > 0, len(c.stderrWriters) > 0; {
	case hasOut && hasErr:
		// Make writes synchronous between stdout and stderr. This ensures all
		// writers that capture both will see the same ordering, and don't need to
		// worry about concurrent writes.
		sharedMu := &sync.Mutex{}
		stdout := &sharedLockWriter{sharedMu, io.MultiWriter(c.stdoutWriters...)}
		stderr := &sharedLockWriter{sharedMu, io.MultiWriter(c.stderrWriters...)}
		return stdout, stderr, nil
	case hasOut:
		return io.MultiWriter(c.stdoutWriters...), nil, nil
	case hasErr:
		return nil, io.MultiWriter(c.stderrWriters...), nil
	}
	return nil, nil, nil
}

type sharedLockWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (w *sharedLockWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	n, err := w.w.Write(p)
	w.mu.Unlock()
	return n, err
}

func (c *Cmd) clone() (*Cmd, error) {
	args := make([]string, len(c.Args))
	copy(args, c.Args)
	res, err := newCmdInternal(c.sh, copyMap(c.Vars), c.Path, args[1:])
	if err != nil {
		return nil, err
	}
	res.IgnoreParentExit = c.IgnoreParentExit
	res.ExitAfter = c.ExitAfter
	res.PropagateOutput = c.PropagateOutput
	res.OutputDir = c.OutputDir
	res.ExitErrorIsOk = c.ExitErrorIsOk
	res.IgnoreClosedPipeError = c.IgnoreClosedPipeError
	return res, nil
}

func (c *Cmd) stdinPipe() (io.WriteCloser, error) {
	switch {
	case c.calledStart:
		return nil, errAlreadyCalledStart
	case c.c.Stdin != nil:
		return nil, errAlreadySetStdin
	}
	// We want to provide an unlimited-size pipe to the user. If we set c.c.Stdin
	// directly to the newBufferedPipe, the os/exec package will create an os.Pipe
	// for us, along with a goroutine to copy data over. And exec.Cmd.Wait will
	// wait for this goroutine to exit before returning, even if the process has
	// already exited. That means the user will be forced to call Close on the
	// returned WriteCloser, which is annoying.
	//
	// Instead, we set c.c.Stdin to our own os.Pipe, so that os/exec won't create
	// the pipe nor the goroutine. We chain our newBufferedPipe in front of this,
	// with our own copier goroutine. This gives the user a pipe that never blocks
	// on Write, and which they don't need to Close if the process exits.
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.c.Stdin = pr
	c.afterStartClosers = append(c.afterStartClosers, pr)
	bp := newBufferedPipe()
	c.afterWaitClosers = append(c.afterWaitClosers, bp)
	c.stdinDoneChan = make(chan error, 1)
	go c.stdinPipeCopier(pw, bp) // pw is closed by stdinPipeCopier
	return bp, nil
}

func (c *Cmd) stdinPipeCopier(dst io.WriteCloser, src io.Reader) {
	var firstErr error
	if _, err := io.Copy(dst, src); err != nil && !isClosedPipeError(err) {
		firstErr = err
	}
	if err := dst.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	c.stdinDoneChan <- firstErr
}

// isClosedPipeError returns true iff the error indicates a closed pipe. This
// typically occurs with a pipeline of commands "A | B"; if B exits first, the
// next write by A will receive a closed pipe error. Also see:
// https://github.com/golang/go/issues/9173
func isClosedPipeError(err error) bool {
	if err == io.ErrClosedPipe {
		return true
	}
	// Closed pipe on os.Pipe; mirrors logic in os/exec/exec_posix.go.
	if pe, ok := err.(*os.PathError); ok {
		if pe.Op == "write" && pe.Path == "|1" && pe.Err == syscall.EPIPE {
			return true
		}
	}
	// Process exited due to a SIGPIPE signal.
	if ee, ok := err.(*exec.ExitError); ok {
		if ws, ok := ee.ProcessState.Sys().(syscall.WaitStatus); ok {
			if ws.Signaled() && ws.Signal() == syscall.SIGPIPE {
				return true
			}
		}
	}
	return false
}

func (c *Cmd) setStdinReader(r io.Reader) error {
	switch {
	case c.calledStart:
		return errAlreadyCalledStart
	case c.c.Stdin != nil:
		return errAlreadySetStdin
	}
	c.c.Stdin = r
	return nil
}

func (c *Cmd) stdoutPipe() (io.ReadCloser, error) {
	if c.calledStart {
		return nil, errAlreadyCalledStart
	}
	p := newBufferedPipe()
	c.stdoutWriters = append(c.stdoutWriters, p)
	c.afterWaitClosers = append(c.afterWaitClosers, p)
	return p, nil
}

func (c *Cmd) stderrPipe() (io.ReadCloser, error) {
	if c.calledStart {
		return nil, errAlreadyCalledStart
	}
	p := newBufferedPipe()
	c.stderrWriters = append(c.stderrWriters, p)
	c.afterWaitClosers = append(c.afterWaitClosers, p)
	return p, nil
}

func (c *Cmd) addStdoutWriter(w io.Writer) error {
	if c.calledStart {
		return errAlreadyCalledStart
	}
	c.stdoutWriters = append(c.stdoutWriters, w)
	return nil
}

func (c *Cmd) addStderrWriter(w io.Writer) error {
	if c.calledStart {
		return errAlreadyCalledStart
	}
	c.stderrWriters = append(c.stderrWriters, w)
	return nil
}

// startExitWaiter spawns a goroutine that calls exec.Cmd.Wait, waiting for the
// process to exit. Calling exec.Cmd.Wait here rather than in gosh.Cmd.Wait
// ensures that the child process is reaped once it exits. Note, gosh.Cmd.wait
// blocks on waitChan.
func (c *Cmd) startExitWaiter() {
	go func() {
		waitErr := c.c.Wait()
		c.cond.L.Lock()
		c.exited = true
		c.cond.Signal()
		c.cond.L.Unlock()
		if err := closeClosers(c.afterWaitClosers); waitErr == nil {
			waitErr = err
		}
		if c.stdinDoneChan != nil {
			// Wait for the stdinPipeCopier goroutine to finish.
			if err := <-c.stdinDoneChan; waitErr == nil {
				waitErr = err
			}
		}
		c.waitChan <- waitErr
		c.cleanupProcessGroup()
	}()
}

func closeClosers(closers []io.Closer) error {
	var firstErr error
	for _, closer := range closers {
		if err := closer.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// TODO(sadovsky): Maybe add optional timeouts for Cmd.{awaitVars,wait}.

func (c *Cmd) awaitVars(keys ...string) (map[string]string, error) {
	switch {
	case !c.started:
		return nil, errDidNotCallStart
	case c.calledWait:
		return nil, errAlreadyCalledWait
	}
	wantKeys := map[string]bool{}
	for _, key := range keys {
		wantKeys[key] = true
	}
	res := map[string]string{}
	updateRes := func() {
		for k, v := range c.recvVars {
			if _, ok := wantKeys[k]; ok {
				res[k] = v
			}
		}
	}
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	updateRes()
	for !c.exited && len(res) < len(wantKeys) {
		c.cond.Wait()
		updateRes()
	}
	// Return nil error if both conditions triggered simultaneously.
	if len(res) < len(wantKeys) {
		return nil, errProcessExited
	}
	return res, nil
}

func (c *Cmd) wait() error {
	switch {
	case !c.started:
		return errDidNotCallStart
	case c.calledWait:
		return errAlreadyCalledWait
	}
	c.calledWait = true
	return <-c.waitChan
}

// Note: We check for this particular error message to handle the unavoidable
// race between sending a signal to a process and the process exiting.
// https://golang.org/src/os/exec_unix.go
// https://golang.org/src/os/exec_windows.go
const errFinished = "os: process already finished"

// NOTE(sadovsky): Technically speaking, Process.Signal(os.Kill) is different
// from Process.Kill. Currently, gosh.Cmd does not provide a way to trigger
// Process.Kill. If it proves necessary, we'll add a "gosh.Kill" implementation
// of the os.Signal interface, and have the signal and terminate methods map
// that to Process.Kill.
func (c *Cmd) signal(sig os.Signal) error {
	switch {
	case !c.started:
		return errDidNotCallStart
	case c.calledWait:
		return errAlreadyCalledWait
	}
	if !c.isRunning() {
		return nil
	}
	if err := c.c.Process.Signal(sig); err != nil && err.Error() != errFinished {
		return err
	}
	return nil
}

func (c *Cmd) terminate(sig os.Signal) error {
	if err := c.signal(sig); err != nil {
		return err
	}
	if err := c.wait(); err != nil {
		// Succeed as long as the process exited, regardless of the exit code.
		if _, ok := err.(*exec.ExitError); !ok {
			return err
		}
	}
	return nil
}

func (c *Cmd) run() error {
	if err := c.start(); err != nil {
		return err
	}
	return c.wait()
}

func (c *Cmd) stdout() (string, error) {
	if c.calledStart {
		return "", errAlreadyCalledStart
	}
	var stdout bytes.Buffer
	c.stdoutWriters = append(c.stdoutWriters, &stdout)
	err := c.run()
	return stdout.String(), err
}

func (c *Cmd) stdoutStderr() (string, string, error) {
	if c.calledStart {
		return "", "", errAlreadyCalledStart
	}
	var stdout, stderr bytes.Buffer
	c.stdoutWriters = append(c.stdoutWriters, &stdout)
	c.stderrWriters = append(c.stderrWriters, &stderr)
	err := c.run()
	return stdout.String(), stderr.String(), err
}

func (c *Cmd) combinedOutput() (string, error) {
	if c.calledStart {
		return "", errAlreadyCalledStart
	}
	var output bytes.Buffer
	c.stdoutWriters = append(c.stdoutWriters, &output)
	c.stderrWriters = append(c.stderrWriters, &output)
	err := c.run()
	return output.String(), err
}

////////////////////////////////////////
// Head-and-tail buffer

// headTail stores the first and last 'capacity' written bytes.
type headTail struct {
	head     []byte
	tail     *ringBuffer
	nWritten int // number of bytes written
}

func newHeadTail(capacity int) *headTail {
	return &headTail{head: make([]byte, capacity)}
}

// Write writes to the buffer.
func (b *headTail) Write(p []byte) (int, error) {
	nHead := len(b.head) - b.nWritten // number of bytes to write to head
	if nHead > len(p) {
		nHead = len(p)
	} else if nHead < 0 {
		nHead = 0
	}
	if nHead > 0 {
		copy(b.head[b.nWritten:], p[:nHead])
	}
	// Write any remaining bytes to tail.
	if len(p) > nHead {
		if b.tail == nil {
			b.tail = newRingBuffer(len(b.head))
		}
		b.tail.Append(p[nHead:])
	}
	b.nWritten += len(p)
	return len(p), nil
}

// String returns the buffer as a string.
func (b *headTail) String() string {
	if b.nWritten == 0 {
		return "[ empty ]"
	}
	if b.tail == nil {
		return string(b.head[:b.nWritten])
	}
	tail := b.tail.String()
	skipped := b.nWritten - 2*len(b.head)
	if skipped <= 0 {
		return fmt.Sprintf("%s%s", b.head, tail)
	}
	return fmt.Sprintf("%s\n[ ... skipping %d bytes ... ]\n%s", b.head, skipped, tail)
}
