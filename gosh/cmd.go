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
	"time"
)

var (
	errAlreadyCalledStart = errors.New("gosh: already called Cmd.Start")
	errAlreadyCalledWait  = errors.New("gosh: already called Cmd.Wait")
	errCloseStdout        = errors.New("gosh: use NopWriteCloser(os.Stdout) to prevent stdout from being closed")
	errCloseStderr        = errors.New("gosh: use NopWriteCloser(os.Stderr) to prevent stderr from being closed")
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
	// Args is the list of args for this Cmd, not including the path.
	Args []string
	// PropagateOutput is inherited from Shell.Opts.PropagateChildOutput.
	PropagateOutput bool
	// OutputDir is inherited from Shell.Opts.ChildOutputDir.
	OutputDir string
	// ExitErrorIsOk specifies whether an *exec.ExitError should be reported via
	// Shell.HandleError.
	ExitErrorIsOk bool
	// Stdin is a string to write to the child's stdin.
	Stdin string
	// Internal state.
	sh               *Shell
	c                *exec.Cmd
	stdinWriteCloser io.WriteCloser // from exec.Cmd.StdinPipe
	calledStart      bool
	calledWait       bool
	cond             *sync.Cond
	waitChan         chan error
	started          bool // protected by sh.cleanupMu
	exited           bool // protected by cond.L
	stdoutWriters    []io.Writer
	stderrWriters    []io.Writer
	closers          []io.Closer
	recvReady        bool              // protected by cond.L
	recvVars         map[string]string // protected by cond.L
}

// Clone returns a new Cmd with a copy of this Cmd's configuration.
func (c *Cmd) Clone() *Cmd {
	c.sh.Ok()
	res, err := c.clone()
	c.handleError(err)
	return res
}

// StdinPipe returns a thread-safe WriteCloser backed by a buffered pipe for the
// command's stdin. The returned pipe will be closed when the process exits, but
// may also be closed earlier by the caller, e.g. if the command does not exit
// until its stdin is closed. Must be called before Start. It is safe to call
// StdinPipe multiple times; calls after the first return the pipe created by
// the first call.
func (c *Cmd) StdinPipe() io.WriteCloser {
	c.sh.Ok()
	res, err := c.stdinPipe()
	c.handleError(err)
	return res
}

// StdoutPipe returns a Reader backed by a buffered pipe for the command's
// stdout. Must be called before Start. May be called more than once; each
// invocation creates a new pipe.
func (c *Cmd) StdoutPipe() io.Reader {
	c.sh.Ok()
	res, err := c.stdoutPipe()
	c.handleError(err)
	return res
}

// StderrPipe returns a Reader backed by a buffered pipe for the command's
// stderr. Must be called before Start. May be called more than once; each
// invocation creates a new pipe.
func (c *Cmd) StderrPipe() io.Reader {
	c.sh.Ok()
	res, err := c.stderrPipe()
	c.handleError(err)
	return res
}

// AddStdoutWriter configures this Cmd to tee the child's stdout to the given
// WriteCloser, which will be closed when the process exits.
//
// If the same WriteCloser is passed to both AddStdoutWriter and
// AddStderrWriter, Cmd will ensure that its methods are never called
// concurrently and that Close is only called once.
//
// Use NopWriteCloser to extend a Writer to a WriteCloser, or to prevent an
// existing WriteCloser from being closed. It is an error to pass in os.Stdout
// or os.Stderr, since they shouldn't be closed.
func (c *Cmd) AddStdoutWriter(wc io.WriteCloser) {
	c.sh.Ok()
	c.handleError(c.addStdoutWriter(wc))
}

// AddStderrWriter configures this Cmd to tee the child's stderr to the given
// WriteCloser, which will be closed when the process exits.
//
// If the same WriteCloser is passed to both AddStdoutWriter and
// AddStderrWriter, Cmd will ensure that its methods are never called
// concurrently and that Close is only called once.
//
// Use NopWriteCloser to extend a Writer to a WriteCloser, or to prevent an
// existing WriteCloser from being closed. It is an error to pass in os.Stdout
// or os.Stderr, since they shouldn't be closed.
func (c *Cmd) AddStderrWriter(wc io.WriteCloser) {
	c.sh.Ok()
	c.handleError(c.addStderrWriter(wc))
}

// Start starts the command.
func (c *Cmd) Start() {
	c.sh.Ok()
	c.handleError(c.start())
}

// AwaitReady waits for the child process to call SendReady. Must not be called
// before Start or after Wait.
func (c *Cmd) AwaitReady() {
	c.sh.Ok()
	c.handleError(c.awaitReady())
}

// AwaitVars waits for the child process to send values for the given vars
// (using SendVars). Must not be called before Start or after Wait.
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

// Kill causes the process to exit immediately.
func (c *Cmd) Kill() {
	c.sh.Ok()
	c.handleError(c.kill())
}

// Signal sends a signal to the process.
func (c *Cmd) Signal(sig os.Signal) {
	c.sh.Ok()
	c.handleError(c.signal(sig))
}

// Shutdown sends a signal to the process, then waits for it to exit.
func (c *Cmd) Shutdown(sig os.Signal) {
	c.sh.Ok()
	c.handleError(c.shutdown(sig))
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

// Pid returns the command's PID, or -1 if the command has not been started.
func (c *Cmd) Pid() int {
	if !c.started {
		return -1
	}
	return c.c.Process.Pid
}

////////////////////////////////////////
// Internals

func newCmdInternal(sh *Shell, vars map[string]string, path string, args []string) (*Cmd, error) {
	c := &Cmd{
		Path:     path,
		Vars:     vars,
		Args:     args,
		sh:       sh,
		c:        &exec.Cmd{},
		cond:     sync.NewCond(&sync.Mutex{}),
		waitChan: make(chan error, 1),
		recvVars: map[string]string{},
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
		if lp, err := exec.LookPath(name); err != nil {
			return nil, err
		} else {
			name = lp
		}
	}
	return newCmdInternal(sh, vars, name, args)
}

func (c *Cmd) errorIsOk(err error) bool {
	if c.ExitErrorIsOk {
		if _, ok := err.(*exec.ExitError); ok {
			return true
		}
	}
	return err == nil
}

func (c *Cmd) handleError(err error) {
	c.Err = err
	if !c.errorIsOk(err) {
		c.sh.HandleError(err)
	}
}

func (c *Cmd) addWriter(writers *[]io.Writer, w io.Writer) {
	*writers = append(*writers, w)
}

func (c *Cmd) closeClosers() {
	// If the same WriteCloser was passed to both AddStdoutWriter and
	// AddStderrWriter, we should only close it once.
	cm := map[io.Closer]bool{}
	for _, c := range c.closers {
		if !cm[c] {
			cm[c] = true
			c.Close() // best-effort; ignore returned error
		}
	}
}

func (c *Cmd) isRunning() bool {
	if !c.started {
		return false
	}
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	return !c.exited
}

// recvWriter listens for gosh messages from a child process.
type recvWriter struct {
	c          *Cmd
	buf        bytes.Buffer
	readPrefix bool // if true, we've read len(msgPrefix) for the current line
	skipLine   bool // if true, ignore bytes until next '\n'
}

func (w *recvWriter) Write(p []byte) (n int, err error) {
	for _, b := range p {
		if b == '\n' {
			if w.readPrefix && !w.skipLine {
				m := msg{}
				if err := json.Unmarshal(w.buf.Bytes(), &m); err != nil {
					return 0, err
				}
				switch m.Type {
				case typeReady:
					w.c.cond.L.Lock()
					w.c.recvReady = true
					w.c.cond.Signal()
					w.c.cond.L.Unlock()
				case typeVars:
					w.c.cond.L.Lock()
					w.c.recvVars = mergeMaps(w.c.recvVars, m.Vars)
					w.c.cond.Signal()
					w.c.cond.L.Unlock()
				default:
					return 0, fmt.Errorf("unknown message type: %q", m.Type)
				}
			}
			// Reset state for next line.
			w.readPrefix, w.skipLine = false, false
			w.buf.Reset()
		} else if !w.skipLine {
			w.buf.WriteByte(b)
			if !w.readPrefix && w.buf.Len() == len(msgPrefix) {
				w.readPrefix = true
				prefix := string(w.buf.Next(len(msgPrefix)))
				if prefix != msgPrefix {
					w.skipLine = true
				}
			}
		}
	}
	return len(p), nil
}

func (c *Cmd) makeMultiWriter(stdout bool, t string) (io.Writer, error) {
	std, writers := os.Stderr, &c.stderrWriters
	if stdout {
		std, writers = os.Stdout, &c.stdoutWriters
		c.addWriter(writers, &recvWriter{c: c})
	}
	if c.PropagateOutput {
		c.addWriter(writers, std)
		// Don't add std to c.closers, since we don't want to close os.Stdout or
		// os.Stderr for the entire address space when c exits.
	}
	if c.OutputDir != "" {
		suffix := "stderr"
		if stdout {
			suffix = "stdout"
		}
		name := filepath.Join(c.OutputDir, filepath.Base(c.Path)+"."+t+"."+suffix)
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, err
		}
		c.addWriter(writers, file)
		c.closers = append(c.closers, file)
	}
	return io.MultiWriter(*writers...), nil
}

func (c *Cmd) clone() (*Cmd, error) {
	vars := make(map[string]string, len(c.Vars))
	for k, v := range c.Vars {
		vars[k] = v
	}
	args := make([]string, len(c.Args))
	copy(args, c.Args)
	res, err := newCmdInternal(c.sh, vars, c.Path, args)
	if err != nil {
		return nil, err
	}
	res.PropagateOutput = c.PropagateOutput
	res.OutputDir = c.OutputDir
	res.ExitErrorIsOk = c.ExitErrorIsOk
	res.Stdin = c.Stdin
	return res, nil
}

func (c *Cmd) stdinPipe() (io.WriteCloser, error) {
	if c.calledStart {
		return nil, errAlreadyCalledStart
	}
	if c.stdinWriteCloser != nil {
		return c.stdinWriteCloser, nil
	}
	var err error
	c.stdinWriteCloser, err = c.c.StdinPipe()
	return c.stdinWriteCloser, err
}

func (c *Cmd) stdoutPipe() (io.Reader, error) {
	if c.calledStart {
		return nil, errAlreadyCalledStart
	}
	p := NewBufferedPipe()
	c.addWriter(&c.stdoutWriters, p)
	c.closers = append(c.closers, p)
	return p, nil
}

func (c *Cmd) stderrPipe() (io.Reader, error) {
	if c.calledStart {
		return nil, errAlreadyCalledStart
	}
	p := NewBufferedPipe()
	c.addWriter(&c.stderrWriters, p)
	c.closers = append(c.closers, p)
	return p, nil
}

func (c *Cmd) addStdoutWriter(wc io.WriteCloser) error {
	switch {
	case c.calledStart:
		return errAlreadyCalledStart
	case wc == os.Stdout:
		return errCloseStdout
	case wc == os.Stderr:
		return errCloseStderr
	}
	c.addWriter(&c.stdoutWriters, wc)
	c.closers = append(c.closers, wc)
	return nil
}

func (c *Cmd) addStderrWriter(wc io.WriteCloser) error {
	switch {
	case c.calledStart:
		return errAlreadyCalledStart
	case wc == os.Stdout:
		return errCloseStdout
	case wc == os.Stderr:
		return errCloseStderr
	}
	c.addWriter(&c.stderrWriters, wc)
	c.closers = append(c.closers, wc)
	return nil
}

// TODO(sadovsky): Maybe wrap every child process with a "supervisor" process
// that calls WatchParent().

type threadSafeWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *threadSafeWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

func (c *Cmd) start() error {
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
	// Wrap Writers in threadSafeWriters as needed so that if the same WriteCloser
	// was passed to both AddStdoutWriter and AddStderrWriter, the writes are
	// serialized.
	isStdoutWriter := map[io.Writer]bool{}
	for _, w := range c.stdoutWriters {
		isStdoutWriter[w] = true
	}
	safe := map[io.Writer]*threadSafeWriter{}
	for i, w := range c.stderrWriters {
		if isStdoutWriter[w] {
			if safe[w] == nil {
				safe[w] = &threadSafeWriter{w: w}
			}
			c.stderrWriters[i] = safe[w]
		}
	}
	if len(safe) > 0 {
		for i, w := range c.stdoutWriters {
			if s := safe[w]; s != nil {
				c.stdoutWriters[i] = s
			}
		}
	}
	// Configure the command.
	c.c.Path = c.Path
	c.c.Env = mapToSlice(c.Vars)
	c.c.Args = append([]string{c.Path}, c.Args...)
	if c.Stdin != "" {
		if c.stdinWriteCloser != nil {
			return errors.New("gosh: cannot both set Stdin and call StdinPipe")
		}
		c.c.Stdin = strings.NewReader(c.Stdin)
	}
	t := time.Now().Format("20060102.150405.000000")
	var err error
	if c.c.Stdout, err = c.makeMultiWriter(true, t); err != nil {
		return err
	}
	if c.c.Stderr, err = c.makeMultiWriter(false, t); err != nil {
		return err
	}
	// Start the command.
	onExit := func(err error) {
		c.cond.L.Lock()
		c.exited = true
		c.cond.Signal()
		c.cond.L.Unlock()
		c.closeClosers()
		c.waitChan <- err
	}
	if err = c.c.Start(); err != nil {
		onExit(errors.New("gosh: start failed"))
		return err
	}
	c.started = true
	// Spawn a "waiter" goroutine that calls exec.Cmd.Wait and thus waits for the
	// process to exit. Calling exec.Cmd.Wait here rather than in gosh.Cmd.Wait
	// ensures that the child process is reaped once it exits. Note, gosh.Cmd.wait
	// blocks on waitChan.
	go func() {
		onExit(c.c.Wait())
	}()
	return nil
}

// TODO(sadovsky): Maybe add optional timeouts for
// Cmd.{awaitReady,awaitVars,wait}.

func (c *Cmd) awaitReady() error {
	if !c.started {
		return errDidNotCallStart
	} else if c.calledWait {
		return errAlreadyCalledWait
	}
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	for !c.exited && !c.recvReady {
		c.cond.Wait()
	}
	// Return nil error if both conditions triggered simultaneously.
	if !c.recvReady {
		return errProcessExited
	}
	return nil
}

func (c *Cmd) awaitVars(keys ...string) (map[string]string, error) {
	if !c.started {
		return nil, errDidNotCallStart
	} else if c.calledWait {
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
	if !c.started {
		return errDidNotCallStart
	} else if c.calledWait {
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

func (c *Cmd) kill() error {
	if !c.started {
		return errDidNotCallStart
	}
	if !c.isRunning() {
		return nil
	}
	if err := c.c.Process.Kill(); err != nil && err.Error() != errFinished {
		return err
	}
	return nil
}

func (c *Cmd) signal(sig os.Signal) error {
	if !c.started {
		return errDidNotCallStart
	}
	if !c.isRunning() {
		return nil
	}
	if err := c.c.Process.Signal(sig); err != nil && err.Error() != errFinished {
		return err
	}
	return nil
}

func (c *Cmd) shutdown(sig os.Signal) error {
	if err := c.signal(sig); err != nil {
		return err
	}
	if err := c.wait(); err != nil {
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
	c.addWriter(&c.stdoutWriters, &stdout)
	err := c.run()
	return stdout.String(), err
}

func (c *Cmd) stdoutStderr() (string, string, error) {
	if c.calledStart {
		return "", "", errAlreadyCalledStart
	}
	var stdout, stderr bytes.Buffer
	c.addWriter(&c.stdoutWriters, &stdout)
	c.addWriter(&c.stderrWriters, &stderr)
	err := c.run()
	return stdout.String(), stderr.String(), err
}
