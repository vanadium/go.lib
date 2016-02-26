// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

import (
	"bytes"
	"errors"
	"io"
	"os"
)

// Pipeline represents a pipeline of commands, where the stdout and/or stderr of
// one command is connected to the stdin of the next command.
//
// The failure semantics of a pipeline are determined by the failure semantics
// of each command; by default the pipeline fails if any command fails, and
// read/write errors on closed pipes are ignored. This is different from bash,
// where the default is to only check the status of the last command, unless
// "set -o pipefail" is enabled to check the status of all commands, causing
// closed pipe errors to fail the pipeline. Use Cmd.ExitErrorIsOk and
// Cmd.IgnoreClosedPipeError to fine-tune the failure semantics.
//
// The implementation of Pipeline only uses exported methods from Shell and Cmd.
type Pipeline struct {
	sh    *Shell
	cmds  []*Cmd      // INVARIANT: len(cmds) > 0
	state []pipeState // INVARIANT: len(state) == len(cmds) - 1
}

type pipeState struct {
	mode       pipeMode  // describes how to connect cmds[i] to cmds[i+1]
	stdinRead  io.Closer // read-side closer of the stdin pipe for cmds[i+1]
	stdinWrite io.Closer // write-side closer of the stdin pipe for cmds[i+1]
}

type pipeMode int

const (
	pipeStdout pipeMode = iota
	pipeStderr
	pipeCombinedOutput
)

// NewPipeline returns a new Pipeline starting with c. The stdout of each
// command is connected to the stdin of the next command, via calls to
// Pipeline.PipeStdout. To construct pipelines involving stderr, call
// Pipeline.PipeStderr or Pipeline.PipeCombinedOutput directly.
//
// Each command must have been created from the same Shell. Errors are reported
// to c.Shell, via Shell.HandleError. Sets Cmd.IgnoreClosedPipeError to true for
// all commands.
func NewPipeline(c *Cmd, cmds ...*Cmd) *Pipeline {
	sh := c.Shell()
	sh.Ok()
	res, err := newPipeline(sh, c, cmds...)
	handleError(sh, err)
	return res
}

// Cmds returns the commands in the pipeline.
func (p *Pipeline) Cmds() []*Cmd {
	return p.cmds
}

// Clone returns a new Pipeline where p's commands are cloned and connected with
// the same pipeline structure as in p.
func (p *Pipeline) Clone() *Pipeline {
	p.sh.Ok()
	res, err := p.clone()
	handleError(p.sh, err)
	return res
}

// PipeStdout connects the stdout of the last command in p to the stdin of c,
// and appends c to the commands in p. Must be called before Start. Sets
// c.IgnoreClosedPipeError to true.
func (p *Pipeline) PipeStdout(c *Cmd) {
	p.sh.Ok()
	handleError(p.sh, p.pipeTo(c, pipeStdout, false))
}

// PipeStderr connects the stderr of the last command in p to the stdin of c,
// and appends c to the commands in p. Must be called before Start. Sets
// c.IgnoreClosedPipeError to true.
func (p *Pipeline) PipeStderr(c *Cmd) {
	p.sh.Ok()
	handleError(p.sh, p.pipeTo(c, pipeStderr, false))
}

// PipeCombinedOutput connects the combined stdout and stderr of the last
// command in p to the stdin of c, and appends c to the commands in p. Must be
// called before Start. Sets c.IgnoreClosedPipeError to true.
func (p *Pipeline) PipeCombinedOutput(c *Cmd) {
	p.sh.Ok()
	handleError(p.sh, p.pipeTo(c, pipeCombinedOutput, false))
}

// Start starts all commands in the pipeline.
func (p *Pipeline) Start() {
	p.sh.Ok()
	handleError(p.sh, p.start())
}

// Wait waits for all commands in the pipeline to exit.
func (p *Pipeline) Wait() {
	p.sh.Ok()
	handleError(p.sh, p.wait())
}

// Signal sends a signal to all underlying processes in the pipeline.
func (p *Pipeline) Signal(sig os.Signal) {
	p.sh.Ok()
	handleError(p.sh, p.signal(sig))
}

// Terminate sends a signal to all underlying processes in the pipeline, then
// waits for all processes to exit. Terminate is different from Signal followed
// by Wait: Terminate succeeds as long as all processes exit, whereas Wait fails
// if any process's exit code isn't 0.
func (p *Pipeline) Terminate(sig os.Signal) {
	p.sh.Ok()
	handleError(p.sh, p.terminate(sig))
}

// Run calls Start followed by Wait.
func (p *Pipeline) Run() {
	p.sh.Ok()
	handleError(p.sh, p.run())
}

// Stdout calls Start followed by Wait, then returns the last command's stdout.
func (p *Pipeline) Stdout() string {
	p.sh.Ok()
	res, err := p.stdout()
	handleError(p.sh, err)
	return res
}

// StdoutStderr calls Start followed by Wait, then returns the last command's
// stdout and stderr.
func (p *Pipeline) StdoutStderr() (string, string) {
	p.sh.Ok()
	stdout, stderr, err := p.stdoutStderr()
	handleError(p.sh, err)
	return stdout, stderr
}

// CombinedOutput calls Start followed by Wait, then returns the last command's
// combined stdout and stderr.
func (p *Pipeline) CombinedOutput() string {
	p.sh.Ok()
	res, err := p.combinedOutput()
	handleError(p.sh, err)
	return res
}

////////////////////////////////////////
// Internals

// handleError is used instead of direct calls to Shell.HandleError throughout
// the pipeline implementation. This is needed to handle the case where the user
// has set Shell.ContinueOnError to true.
//
// The general pattern is that after each Shell or Cmd method is called, we
// check p.sh.Err; if it's non-nil, we wrap it with errAlreadyHandled to
// indicate that Shell.HandleError has already been called with this error and
// should not be called again.
func handleError(sh *Shell, err error) {
	if _, ok := err.(errAlreadyHandled); ok {
		return // the shell has already handled this error
	}
	sh.HandleErrorWithSkip(err, 3)
}

type errAlreadyHandled struct {
	error
}

func newPipeline(sh *Shell, first *Cmd, cmds ...*Cmd) (*Pipeline, error) {
	p := &Pipeline{sh: sh, cmds: []*Cmd{first}}
	first.IgnoreClosedPipeError = true
	for _, c := range cmds {
		if err := p.pipeTo(c, pipeStdout, false); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (p *Pipeline) clone() (*Pipeline, error) {
	// Replicate the pipeline structure with cloned commands.
	first := p.cmds[0].Clone()
	if p.sh.Err != nil {
		return nil, errAlreadyHandled{p.sh.Err}
	}
	res := &Pipeline{sh: p.sh, cmds: []*Cmd{first}}
	for i := 1; i < len(p.cmds); i++ {
		if err := res.pipeTo(p.cmds[i], p.state[i-1].mode, true); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (p *Pipeline) pipeTo(c *Cmd, mode pipeMode, clone bool) (e error) {
	if p.sh != c.Shell() {
		return errors.New("gosh: pipeline cmds have different shells")
	}
	if clone {
		c = c.Clone()
		if p.sh.Err != nil {
			return errAlreadyHandled{p.sh.Err}
		}
	} else {
		c.IgnoreClosedPipeError = true
	}
	// We could just use c.StdinPipe() here, but that provides unlimited size
	// buffering using a newBufferedPipe chained to an os.Pipe. We want limited
	// size buffering to avoid unlimited memory growth, so we just use an os.Pipe.
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() {
		// Close both ends of the pipe on failure; on success they'll be closed at
		// the appropriate times in start and wait.
		if e != nil {
			pr.Close()
			pw.Close()
		}
	}()
	if c.SetStdinReader(pr); p.sh.Err != nil {
		return errAlreadyHandled{p.sh.Err}
	}
	last := p.cmds[len(p.cmds)-1]
	if mode == pipeStdout || mode == pipeCombinedOutput {
		if last.AddStdoutWriter(pw); p.sh.Err != nil {
			return errAlreadyHandled{p.sh.Err}
		}
	}
	if mode == pipeStderr || mode == pipeCombinedOutput {
		if last.AddStderrWriter(pw); p.sh.Err != nil {
			return errAlreadyHandled{p.sh.Err}
		}
	}
	p.cmds = append(p.cmds, c)
	p.state = append(p.state, pipeState{mode, pr, pw})
	return nil
}

// TODO(toddw): Clean up resources in Shell.Cleanup. E.g. we'll currently leak
// the os.Pipe fds if the user sets up a pipeline but never calls Start (or
// Wait, Terminate).

func (p *Pipeline) start() error {
	// Start all commands in the pipeline, capturing the first error.
	// Ensure all commands are processed by avoiding early-exit.
	var shErr, closeErr error
	for i, c := range p.cmds {
		p.sh.Err = nil
		if c.Start(); p.sh.Err != nil && shErr == nil {
			shErr = p.sh.Err
		}
		// Close the read-side of the stdin pipe for this command. The fd has been
		// passed to the child process via Start, so we don't need it open anymore.
		if i > 0 {
			if err := p.state[i-1].stdinRead.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
	}
	// If anything failed, close the write-side of the stdin pipes too.
	if shErr != nil || closeErr != nil {
		for _, state := range p.state {
			state.stdinWrite.Close() // ignore error, since start or close failed.
		}
	}
	if shErr != nil {
		p.sh.Err = shErr
		return errAlreadyHandled{shErr}
	}
	return closeErr
}

func (p *Pipeline) wait() error {
	// Wait for all commands in the pipeline, capturing the first error.
	// Ensure all commands are processed by avoiding early-exit.
	var shErr, closeErr error
	for i, c := range p.cmds {
		p.sh.Err = nil
		if c.Wait(); p.sh.Err != nil && shErr == nil {
			shErr = p.sh.Err
		}
		// Close the write-side of the stdin pipe for the next command, so that the
		// next command will eventually read an EOF.
		if i < len(p.state) {
			if err := p.state[i].stdinWrite.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
	}
	if shErr != nil {
		p.sh.Err = shErr
		return errAlreadyHandled{shErr}
	}
	return closeErr
}

func (p *Pipeline) signal(sig os.Signal) error {
	// Signal all commands in the pipeline, capturing the first error.
	// Ensure all commands are processed, by avoiding early-exit.
	var shErr error
	for _, c := range p.cmds {
		p.sh.Err = nil
		if c.Signal(sig); p.sh.Err != nil && shErr == nil {
			shErr = p.sh.Err
		}
	}
	if shErr != nil {
		p.sh.Err = shErr
		return errAlreadyHandled{shErr}
	}
	return nil
}

func (p *Pipeline) terminate(sig os.Signal) error {
	// Terminate all commands in the pipeline, capturing the first error.
	// Ensure all commands are processed, by avoiding early-exit.
	var shErr, closeErr error
	for i, c := range p.cmds {
		p.sh.Err = nil
		if c.Terminate(sig); p.sh.Err != nil && shErr == nil {
			shErr = p.sh.Err
		}
		// Close the write-side of the stdin pipe for the next command, so that the
		// next command will eventually read an EOF.
		if i < len(p.state) {
			if err := p.state[i].stdinWrite.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
	}
	if shErr != nil {
		p.sh.Err = shErr
		return errAlreadyHandled{shErr}
	}
	return closeErr
}

func (p *Pipeline) run() error {
	if err := p.start(); err != nil {
		return err
	}
	return p.wait()
}

func (p *Pipeline) stdout() (string, error) {
	var stdout bytes.Buffer
	last := p.cmds[len(p.cmds)-1]
	if last.AddStdoutWriter(&stdout); p.sh.Err != nil {
		return "", errAlreadyHandled{p.sh.Err}
	}
	err := p.run()
	return stdout.String(), err
}

func (p *Pipeline) stdoutStderr() (string, string, error) {
	var stdout, stderr bytes.Buffer
	last := p.cmds[len(p.cmds)-1]
	if last.AddStdoutWriter(&stdout); p.sh.Err != nil {
		return "", "", errAlreadyHandled{p.sh.Err}
	}
	if last.AddStderrWriter(&stderr); p.sh.Err != nil {
		return "", "", errAlreadyHandled{p.sh.Err}
	}
	err := p.run()
	return stdout.String(), stderr.String(), err
}

func (p *Pipeline) combinedOutput() (string, error) {
	var output bytes.Buffer
	last := p.cmds[len(p.cmds)-1]
	if last.addStdoutWriter(&output); p.sh.Err != nil {
		return "", errAlreadyHandled{p.sh.Err}
	}
	if last.addStderrWriter(&output); p.sh.Err != nil {
		return "", errAlreadyHandled{p.sh.Err}
	}
	err := p.run()
	return output.String(), err
}
