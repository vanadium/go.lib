package vlog

type LoggingOpts interface {
	LoggingOpt()
}

type AutoFlush bool
type AlsoLogToStderr bool
type LogDir string
type LogToStderr bool
type MaxStackBufSize int

// If true, logs are written to standard error as well as to files.
func (_ AlsoLogToStderr) LoggingOpt() {}

// Enable V-leveled logging at the specified level.
func (_ Level) LoggingOpt() {}

// log files will be written to this directory instead of the
// default temporary directory.
func (_ LogDir) LoggingOpt() {}

// If true, logs are written to standard error instead of to files.
func (_ LogToStderr) LoggingOpt() {}

// Set the max size (bytes) of the byte buffer to use for stack
// traces. The default max is 4M; use powers of 2 since the
// stack size will be grown exponentially until it exceeds the max.
// A min of 128K is enforced and any attempts to reduce this will
// be silently ignored.
func (_ MaxStackBufSize) LoggingOpt() {}

// The syntax of the argument is a comma-separated list of pattern=N,
// where pattern is a literal file name (minus the ".go" suffix) or
// "glob" pattern and N is a V level. For instance,
//	-gopher*=3
// sets the V level to 3 in all Go files whose names begin "gopher".
func (_ ModuleSpec) LoggingOpt() {}

// Log events at or above this severity are logged to standard
// error as well as to files.
func (_ StderrThreshold) LoggingOpt() {}

// When set to a file and line number holding a logging statement, such as
//	gopherflakes.go:234
// a stack trace will be written to the Info log whenever execution
// hits that statement. (Unlike with -vmodule, the ".go" must be
// present.)
func (_ TraceLocation) LoggingOpt() {}

// If true, enables automatic flushing of log output on every call
func (_ AutoFlush) LoggingOpt() {}

// TODO(cnicolaou): provide options for setting a remote network
// destination for logging.
