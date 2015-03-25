// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vlog

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/cosmosnicolaou/llog"
)

const (
	initialMaxStackBufSize = 128 * 1024
)

type logger struct {
	log             *llog.Log
	mu              sync.Mutex // guards updates to the vars below.
	autoFlush       bool
	maxStackBufSize int
	logDir          string
	configured      bool
}

func (l *logger) maybeFlush() {
	if l.autoFlush {
		l.log.Flush()
	}
}

var (
	Log        *logger
	Configured = errors.New("logger has already been configured")
)

const stackSkip = 1

func init() {
	Log = &logger{log: llog.NewLogger("vanadium", stackSkip)}
}

// NewLogger creates a new instance of the logging interface.
func NewLogger(name string) Logger {
	// Create an instance of the runtime with just logging enabled.
	return &logger{log: llog.NewLogger(name, stackSkip)}
}

// Configure configures all future logging. Some options
// may not be usable if ConfigureLogging is called from an init function,
// in which case an error will be returned. The Configured error is returned
// if ConfigureLogger has already been called unless the
// OverridePriorConfiguration options is included.
func (l *logger) Configure(opts ...LoggingOpts) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	override := false
	for _, o := range opts {
		switch v := o.(type) {
		case OverridePriorConfiguration:
			override = bool(v)
		}
	}
	if l.configured && !override {
		return Configured
	}
	for _, o := range opts {
		switch v := o.(type) {
		case AlsoLogToStderr:
			l.log.SetAlsoLogToStderr(bool(v))
		case Level:
			l.log.SetV(llog.Level(v))
		case LogDir:
			l.logDir = string(v)
			l.log.SetLogDir(l.logDir)
		case LogToStderr:
			l.log.SetLogToStderr(bool(v))
		case MaxStackBufSize:
			sz := int(v)
			if sz > initialMaxStackBufSize {
				l.maxStackBufSize = sz
				l.log.SetMaxStackBufSize(sz)
			}
		case ModuleSpec:
			l.log.SetVModule(v.ModuleSpec)
		case TraceLocation:
			l.log.SetTraceLocation(v.TraceLocation)
		case StderrThreshold:
			l.log.SetStderrThreshold(llog.Severity(v))
		case AutoFlush:
			l.autoFlush = bool(v)
		}
	}
	l.configured = true
	return nil
}

// LogDir returns the directory where the log files are written.
func (l *logger) LogDir() string {
	if len(l.logDir) != 0 {
		return l.logDir
	}
	return os.TempDir()
}

// Stats returns stats on how many lines/bytes haven been written to
// this set of logs.
func (l *logger) Stats() LevelStats {
	return LevelStats(l.log.Stats())
}

// Info logs to the INFO log.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func (l *logger) Info(args ...interface{}) {
	l.log.Print(llog.InfoLog, args...)
	l.maybeFlush()
}

// Infof logs to the INFO log.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func (l *logger) Infof(format string, args ...interface{}) {
	l.log.Printf(llog.InfoLog, format, args...)
	l.maybeFlush()
}

func infoStack(l *logger, all bool) {
	n := initialMaxStackBufSize
	var trace []byte
	for n <= l.maxStackBufSize {
		trace = make([]byte, n)
		nbytes := runtime.Stack(trace, all)
		if nbytes < len(trace) {
			l.log.Printf(llog.InfoLog, "%s", trace[:nbytes])
			return
		}
		n *= 2
	}
	l.log.Printf(llog.InfoLog, "%s", trace)
	l.maybeFlush()
}

// InfoStack logs the current goroutine's stack if the all parameter
// is false, or the stacks of all goroutines if it's true.
func (l *logger) InfoStack(all bool) {
	infoStack(l, all)
}

func (l *logger) V(v Level) bool {
	return l.log.V(llog.Level(v))
}

type discardInfo struct{}

func (_ *discardInfo) Info(args ...interface{})                 {}
func (_ *discardInfo) Infof(format string, args ...interface{}) {}
func (_ *discardInfo) InfoStack(all bool)                       {}

func (l *logger) VI(v Level) InfoLog {
	if l.log.V(llog.Level(v)) {
		return l
	}
	return &discardInfo{}
}

// Flush flushes all pending log I/O.
func (l *logger) FlushLog() {
	l.log.Flush()
}

// Error logs to the ERROR and INFO logs.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func (l *logger) Error(args ...interface{}) {
	l.log.Print(llog.ErrorLog, args...)
	l.maybeFlush()
}

// Errorf logs to the ERROR and INFO logs.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func (l *logger) Errorf(format string, args ...interface{}) {
	l.log.Printf(llog.ErrorLog, format, args...)
	l.maybeFlush()
}

// Fatal logs to the FATAL, ERROR and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func (l *logger) Fatal(args ...interface{}) {
	l.log.Print(llog.FatalLog, args...)
}

// Fatalf logs to the FATAL, ERROR and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func (l *logger) Fatalf(format string, args ...interface{}) {
	l.log.Printf(llog.FatalLog, format, args...)
}

// Panic is equivalent to Error() followed by a call to panic().
func (l *logger) Panic(args ...interface{}) {
	l.Error(args...)
	panic(fmt.Sprint(args...))
}

// Panicf is equivalent to Errorf() followed by a call to panic().
func (l *logger) Panicf(format string, args ...interface{}) {
	l.Errorf(format, args...)
	panic(fmt.Sprintf(format, args...))
}
