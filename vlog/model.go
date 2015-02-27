package vlog

import (
	// TODO(cnicolaou): remove this dependency in the future. For now this
	// saves us some code.
	"github.com/cosmosnicolaou/llog"
)

type InfoLog interface {
	// Info logs to the INFO log.
	// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
	Info(args ...interface{})

	// Infoln logs to the INFO log.
	// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
	Infof(format string, args ...interface{})

	// InfoStack logs the current goroutine's stack if the all parameter
	// is false, or the stacks of all goroutines if it's true.
	InfoStack(all bool)
}

type Verbosity interface {
	// V returns true if the configured logging level is greater than or equal to its parameter
	V(level Level) bool
	// VI is like V, except that it returns an instance of the Info
	// interface that will either log (if level >= the configured level)
	// or discard its parameters. This allows for logger.VI(2).Info
	// style usage.
	VI(level Level) InfoLog
}

// Level specifies a level of verbosity for V logs.
// It can be set via the Level optional parameter to ConfigureLogger.
// It implements the flag.Value interface to support command line option parsing.
type Level llog.Level

// Set is part of the flag.Value interface.
func (l *Level) Set(v string) error {
	return (*llog.Level)(l).Set(v)
}

// Get is part of the flag.Value interface.
func (l *Level) Get(v string) interface{} {
	return *l
}

// String is part of the flag.Value interface.
func (l *Level) String() string {
	return (*llog.Level)(l).String()
}

// StderrThreshold identifies the sort of log: info, warning etc.
// The values match the corresponding constants in C++ - e.g WARNING etc.
// It can be set via the StderrThreshold optional parameter to ConfigureLogger.
// It implements the flag.Value interface to support command line option parsing.
type StderrThreshold llog.Severity

// Set is part of the flag.Value interface.
func (s *StderrThreshold) Set(v string) error {
	return (*llog.Severity)(s).Set(v)
}

// Get is part of the flag.Value interface.
func (s *StderrThreshold) Get(v string) interface{} {
	return *s
}

// String is part of the flag.Value interface.
func (s *StderrThreshold) String() string {
	return (*llog.Severity)(s).String()
}

// ModuleSpec allows for the setting of specific log levels for specific
// modules. The syntax is recordio=2,file=1,gfs*=3
// It can be set via the ModuleSpec optional parameter to ConfigureLogger.
// It implements the flag.Value interface to support command line option parsing.
type ModuleSpec struct {
	llog.ModuleSpec
}

// TraceLocation specifies the location, file:N, which when encountered will
// cause logging to emit a stack trace.
// It can be set via the TraceLocation optional parameter to ConfigureLogger.
// It implements the flag.Value interface to support command line option parsing.
type TraceLocation struct {
	llog.TraceLocation
}

// LevelStats tracks the number of lines of output and number of bytes
// per severity level.
type LevelStats llog.Stats

type Logger interface {
	InfoLog
	Verbosity

	// Flush flushes all pending log I/O.
	FlushLog()

	// Error logs to the ERROR and INFO logs.
	// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
	Error(args ...interface{})

	// Errorf logs to the ERROR and INFO logs.
	// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
	Errorf(format string, args ...interface{})

	// Fatal logs to the FATAL, ERROR and INFO logs,
	// including a stack trace of all running goroutines, then calls os.Exit(255).
	// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
	Fatal(args ...interface{})

	// Fatalf logs to the FATAL, ERROR and INFO logs,
	// including a stack trace of all running goroutines, then calls os.Exit(255).
	// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
	Fatalf(format string, args ...interface{})

	// Panic is equivalent to Error() followed by a call to panic().
	Panic(args ...interface{})

	// Panicf is equivalent to Errorf() followed by a call to panic().
	Panicf(format string, args ...interface{})

	// ConfigureLogger configures all future logging. Some options
	// may not be usable if ConfigureLogger
	// is called from an init function, in which case an error will
	// be returned.
	// Some options only take effect if they are set before the logger
	// is used.  Once anything is logged using the logger, these options
	// will silently be ignored.  For example, LogDir, LogToStderr or
	// AlsoLogToStderr fall in this category.
	ConfigureLogger(opts ...LoggingOpts) error

	// Stats returns stats on how many lines/bytes haven been written to
	// this set of logs per severity level.
	Stats() LevelStats

	// LogDir returns the currently configured directory for storing logs.
	LogDir() string
}

// Runtime defines the methods that the runtime must implement.
type Runtime interface {
	// Logger returns the current logger, if any, in use by the Runtime.
	// TODO(cnicolaou): remove this.
	Logger() Logger

	// NewLogger creates a new instance of the logging interface that is
	// separate from the one provided by Runtime.
	NewLogger(name string, opts ...LoggingOpts) (Logger, error)
}
