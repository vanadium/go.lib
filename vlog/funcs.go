package vlog

import (
	"github.com/cosmosnicolaou/llog"
)

// Info logs to the INFO log.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Info(args ...interface{}) {
	Log.log.Print(llog.InfoLog, args...)
	Log.maybeFlush()
}

// Infof logs to the INFO log.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Infof(format string, args ...interface{}) {
	Log.log.Printf(llog.InfoLog, format, args...)
	Log.maybeFlush()
}

// InfoStack logs the current goroutine's stack if the all parameter
// is false, or the stacks of all goroutines if it's true.
func InfoStack(all bool) {
	infoStack(Log, all)
}

// V returns true if the configured logging level is greater than or equal to its parameter
func V(level Level) bool {
	return Log.log.V(llog.Level(level))
}

// VI is like V, except that it returns an instance of the Info
// interface that will either log (if level >= the configured level)
// or discard its parameters. This allows for logger.VI(2).Info
// style usage.
func VI(level Level) InfoLog {
	if Log.log.V(llog.Level(level)) {
		return Log
	}
	return &discardInfo{}
}

// Flush flushes all pending log I/O.
func FlushLog() {
	Log.FlushLog()
}

// Error logs to the ERROR and INFO logs.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Error(args ...interface{}) {
	Log.log.Print(llog.ErrorLog, args...)
	Log.maybeFlush()
}

// Errorf logs to the ERROR and INFO logs.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Errorf(format string, args ...interface{}) {
	Log.log.Printf(llog.ErrorLog, format, args...)
	Log.maybeFlush()
}

// Fatal logs to the FATAL, ERROR and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Fatal(args ...interface{}) {
	Log.log.Print(llog.FatalLog, args...)
}

// Fatalf logs to the FATAL, ERROR and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Fatalf(format string, args ...interface{}) {
	Log.log.Printf(llog.FatalLog, format, args...)
}

// ConfigureLogging configures all future logging. Some options
// may not be usable if ConfigureLogging is called from an init function,
// in which case an error will be returned.
func ConfigureLogger(opts ...LoggingOpts) error {
	return Log.ConfigureLogger(opts...)
}

// Stats returns stats on how many lines/bytes haven been written to
// this set of logs.
func Stats() LevelStats {
	return Log.Stats()
}

// Panic is equivalent to Error() followed by a call to panic().
func Panic(args ...interface{}) {
	Log.Panic(args...)
}

// Panicf is equivalent to Errorf() followed by a call to panic().
func Panicf(format string, args ...interface{}) {
	Log.Panicf(format, args...)
}
