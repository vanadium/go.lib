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
	"sync"
	"time"
)

var (
	errAlreadyCalledStart = errors.New("gosh: already called Cmd.Start")
	errAlreadyCalledWait  = errors.New("gosh: already called Cmd.Wait")
	errDidNotCallStart    = errors.New("gosh: did not call Cmd.Start")
)

// Cmd represents a command. Not thread-safe.
// Public fields should not be modified after calling Start.
type Cmd struct {
	// Vars is the map of env vars for this Cmd.
	Vars map[string]string
	// Args is the list of args for this Cmd.
	Args []string
	// SuppressOutput is inherited from Shell.Opts.SuppressChildOutput.
	SuppressOutput bool
	// OutputDir is inherited from Shell.Opts.ChildOutputDir.
	OutputDir string
	// Stdin specifies this Cmd's stdin. See comments in exec.Cmd for detailed
	// semantics.
	Stdin io.Reader
	// Internal state.
	sh             *Shell
	c              *exec.Cmd
	name           string
	calledWait     bool
	stdoutWriters  []io.Writer
	stderrWriters  []io.Writer
	closeAfterWait []io.Closer
	condReady      *sync.Cond
	recvReady      bool // protected by condReady.L
	condVars       *sync.Cond
	recvVars       map[string]string // protected by condVars.L
}

// StdoutPipe returns a Reader backed by a buffered pipe for this command's
// stdout. Must be called before Start. May be called more than once; each
// invocation creates a new pipe.
func (c *Cmd) StdoutPipe() io.Reader {
	c.sh.Ok()
	res, err := c.stdoutPipe()
	c.sh.HandleError(err)
	return res
}

// StderrPipe returns a Reader backed by a buffered pipe for this command's
// stderr. Must be called before Start. May be called more than once; each
// invocation creates a new pipe.
func (c *Cmd) StderrPipe() io.Reader {
	c.sh.Ok()
	res, err := c.stderrPipe()
	c.sh.HandleError(err)
	return res
}

// Start starts this command.
func (c *Cmd) Start() {
	c.sh.Ok()
	c.sh.HandleError(c.start())
}

// AwaitReady waits for the child process to call SendReady. Must not be called
// before Start or after Wait.
func (c *Cmd) AwaitReady() {
	c.sh.Ok()
	c.sh.HandleError(c.awaitReady())
}

// AwaitVars waits for the child process to send values for the given vars
// (using SendVars). Must not be called before Start or after Wait.
func (c *Cmd) AwaitVars(keys ...string) map[string]string {
	c.sh.Ok()
	res, err := c.awaitVars(keys...)
	c.sh.HandleError(err)
	return res
}

// Wait waits for this command to exit.
func (c *Cmd) Wait() {
	c.sh.Ok()
	c.sh.HandleError(c.wait())
}

// TODO(sadovsky): Maybe add a method to send SIGINT, wait for a bit, then send
// SIGKILL if the process hasn't exited.

// Shutdown sends the given signal to this command, then waits for it to exit.
func (c *Cmd) Shutdown(sig os.Signal) {
	c.sh.Ok()
	c.sh.HandleError(c.shutdown(sig))
}

// Run calls Start followed by Wait.
func (c *Cmd) Run() {
	c.sh.Ok()
	c.sh.HandleError(c.run())
}

// Output calls Start followed by Wait, then returns this command's stdout and
// stderr.
func (c *Cmd) Output() ([]byte, []byte) {
	c.sh.Ok()
	stdout, stderr, err := c.output()
	c.sh.HandleError(err)
	return stdout, stderr
}

// CombinedOutput calls Start followed by Wait, then returns this command's
// combined stdout and stderr.
func (c *Cmd) CombinedOutput() []byte {
	c.sh.Ok()
	res, err := c.combinedOutput()
	c.sh.HandleError(err)
	return res
}

// Process returns the underlying process handle for this command.
func (c *Cmd) Process() *os.Process {
	c.sh.Ok()
	res, err := c.process()
	c.sh.HandleError(err)
	return res
}

////////////////////////////////////////
// Internals

func newCmd(sh *Shell, vars map[string]string, name string, args ...string) (*Cmd, error) {
	// Mimics https://golang.org/src/os/exec/exec.go Command.
	if filepath.Base(name) == name {
		if lp, err := exec.LookPath(name); err != nil {
			return nil, err
		} else {
			name = lp
		}
	}
	c := &Cmd{
		Vars:      vars,
		Args:      args,
		sh:        sh,
		name:      name,
		condReady: sync.NewCond(&sync.Mutex{}),
		condVars:  sync.NewCond(&sync.Mutex{}),
		recvVars:  map[string]string{},
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

func (c *Cmd) calledStart() bool {
	return c.c != nil
}

func closeAll(closers []io.Closer) {
	for _, c := range closers {
		c.Close()
	}
}

func addWriter(writers *[]io.Writer, w io.Writer) {
	*writers = append(*writers, w)
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
					w.c.condReady.L.Lock()
					w.c.recvReady = true
					w.c.condReady.Signal()
					w.c.condReady.L.Unlock()
				case typeVars:
					w.c.condVars.L.Lock()
					w.c.recvVars = mergeMaps(w.c.recvVars, m.Vars)
					w.c.condVars.Signal()
					w.c.condVars.L.Unlock()
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

func (c *Cmd) initMultiWriter(f *os.File, t string) (io.Writer, error) {
	var writers *[]io.Writer
	if f == os.Stdout {
		writers = &c.stdoutWriters
	} else {
		writers = &c.stderrWriters
	}
	if !c.SuppressOutput {
		addWriter(writers, f)
	}
	if c.OutputDir != "" {
		suffix := "stderr"
		if f == os.Stdout {
			suffix = "stdout"
		}
		name := filepath.Join(c.OutputDir, filepath.Base(c.name)+"."+t+"."+suffix)
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, err
		}
		addWriter(writers, file)
		c.closeAfterWait = append(c.closeAfterWait, file)
	}
	if f == os.Stdout {
		addWriter(writers, &recvWriter{c: c})
	}
	return io.MultiWriter(*writers...), nil
}

func (c *Cmd) stdoutPipe() (io.Reader, error) {
	if c.calledStart() {
		return nil, errAlreadyCalledStart
	}
	p := NewBufferedPipe()
	addWriter(&c.stdoutWriters, p)
	c.closeAfterWait = append(c.closeAfterWait, p)
	return p, nil
}

func (c *Cmd) stderrPipe() (io.Reader, error) {
	if c.calledStart() {
		return nil, errAlreadyCalledStart
	}
	p := NewBufferedPipe()
	addWriter(&c.stderrWriters, p)
	c.closeAfterWait = append(c.closeAfterWait, p)
	return p, nil
}

func (c *Cmd) start() error {
	if c.calledStart() {
		return errAlreadyCalledStart
	}
	// Protect against Cmd.start() writing to c.c.Process concurrently with
	// signal-triggered Shell.cleanup() reading from it.
	c.sh.cleanupMu.Lock()
	defer c.sh.cleanupMu.Unlock()
	if c.sh.calledCleanup {
		return errAlreadyCalledCleanup
	}
	c.c = exec.Command(c.name, c.Args...)
	c.c.Env = mapToSlice(c.Vars)
	c.c.Stdin = c.Stdin
	t := time.Now().UTC().Format("20060102.150405.000000")
	var err error
	if c.c.Stdout, err = c.initMultiWriter(os.Stdout, t); err != nil {
		return err
	}
	if c.c.Stderr, err = c.initMultiWriter(os.Stderr, t); err != nil {
		return err
	}
	// TODO(sadovsky): Maybe wrap every child process with a "supervisor" process
	// that calls WatchParent().
	err = c.c.Start()
	if err != nil {
		closeAll(c.closeAfterWait)
	}
	return err
}

// TODO(sadovsky): Add timeouts for Cmd.{awaitReady,awaitVars,wait}.

func (c *Cmd) awaitReady() error {
	if !c.calledStart() {
		return errDidNotCallStart
	} else if c.calledWait {
		return errAlreadyCalledWait
	}
	// http://golang.org/pkg/sync/#Cond.Wait
	c.condReady.L.Lock()
	for !c.recvReady {
		c.condReady.Wait()
	}
	c.condReady.L.Unlock()
	return nil
}

func (c *Cmd) awaitVars(keys ...string) (map[string]string, error) {
	if !c.calledStart() {
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
	// http://golang.org/pkg/sync/#Cond.Wait
	c.condVars.L.Lock()
	updateRes()
	for len(res) < len(wantKeys) {
		c.condVars.Wait()
		updateRes()
	}
	c.condVars.L.Unlock()
	return res, nil
}

func (c *Cmd) wait() error {
	if !c.calledStart() {
		return errDidNotCallStart
	} else if c.calledWait {
		return errAlreadyCalledWait
	}
	c.calledWait = true
	err := c.c.Wait()
	closeAll(c.closeAfterWait)
	return err
}

func (c *Cmd) shutdown(sig os.Signal) error {
	if !c.calledStart() {
		return errDidNotCallStart
	}
	if err := c.c.Process.Signal(sig); err != nil {
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

func (c *Cmd) output() ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	addWriter(&c.stdoutWriters, &stdout)
	addWriter(&c.stderrWriters, &stderr)
	err := c.run()
	return stdout.Bytes(), stderr.Bytes(), err
}

type threadSafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *threadSafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *threadSafeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Bytes()
}

func (c *Cmd) combinedOutput() ([]byte, error) {
	buf := &threadSafeBuffer{}
	addWriter(&c.stdoutWriters, buf)
	addWriter(&c.stderrWriters, buf)
	err := c.run()
	return buf.Bytes(), err
}

func (c *Cmd) process() (*os.Process, error) {
	if !c.calledStart() {
		return nil, errDidNotCallStart
	}
	return c.c.Process, nil
}
