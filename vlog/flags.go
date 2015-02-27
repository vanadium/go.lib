package vlog

import (
	"flag"
	"fmt"

	"github.com/cosmosnicolaou/llog"
)

var (
	toStderr        bool
	alsoToStderr    bool
	logDir          string
	verbosity       Level
	stderrThreshold StderrThreshold = StderrThreshold(llog.ErrorLog)
	vmodule         ModuleSpec
	traceLocation   TraceLocation
	maxStackBufSize int
)

var flagDefs = []struct {
	name         string
	variable     interface{}
	defaultValue interface{}
	description  string
}{
	{"log_dir", &logDir, "", "if non-empty, write log files to this directory"},
	{"logtostderr", &toStderr, false, "log to standard error instead of files"},
	{"alsologtostderr", &alsoToStderr, true, "log to standard error as well as files"},
	{"max_stack_buf_size", &maxStackBufSize, 4192 * 1024, "max size in bytes of the buffer to use for logging stack traces"},
	{"v", &verbosity, nil, "log level for V logs"},
	{"stderrthreshold", &stderrThreshold, nil, "logs at or above this threshold go to stderr"},
	{"vmodule", &vmodule, nil, "comma-separated list of pattern=N settings for file-filtered logging"},
	{"log_backtrace_at", &traceLocation, nil, "when logging hits line file:N, emit a stack trace"},
}

func init() {
	istest := false
	if flag.CommandLine.Lookup("test.v") != nil {
		istest = true
	}
	for _, flagDef := range flagDefs {
		if istest && flagDef.name == "v" {
			continue
		}
		switch v := flagDef.variable.(type) {
		case *string:
			flag.StringVar(v, flagDef.name,
				flagDef.defaultValue.(string), flagDef.description)
		case *bool:
			flag.BoolVar(v, flagDef.name,
				flagDef.defaultValue.(bool), flagDef.description)
		case *int:
			flag.IntVar(v, flagDef.name,
				flagDef.defaultValue.(int), flagDef.description)
		case flag.Value:
			if flagDef.defaultValue != nil {
				panic(fmt.Sprintf("default value not supported for flag %s", flagDef.name))
			}
			flag.Var(v, flagDef.name, flagDef.description)
		default:
			panic("invalid flag type")
		}
	}
}

// ConfigureLibraryLoggerFromFlags will configure the internal global logger
// using command line flags.  It assumes that flag.Parse() has already been
// called to initialize the flag variables.
func ConfigureLibraryLoggerFromFlags() error {
	return ConfigureLoggerFromFlags(Log)
}

// ConfigureLoggerFromLogs will configure the supplied logger using
// command line flags.
func ConfigureLoggerFromFlags(l Logger) error {
	return l.ConfigureLogger(
		LogToStderr(toStderr),
		AlsoLogToStderr(alsoToStderr),
		LogDir(logDir),
		Level(verbosity),
		StderrThreshold(stderrThreshold),
		ModuleSpec(vmodule),
		TraceLocation(traceLocation),
		MaxStackBufSize(maxStackBufSize),
	)
}

func (l *logger) String() string {
	return l.log.String()
}

// ExplicitlySetFlags returns a map of the logging command line flags and their
// values formatted as strings.  Only the flags that were explicitly set are
// returned. This is intended for use when an application needs to know what
// value the flags were set to, for example when creating subprocesses.
func (l *logger) ExplicitlySetFlags() map[string]string {
	logFlagNames := make(map[string]bool)
	for _, flagDef := range flagDefs {
		logFlagNames[flagDef.name] = true
	}
	args := make(map[string]string)
	flag.Visit(func(f *flag.Flag) {
		if logFlagNames[f.Name] {
			args[f.Name] = f.Value.String()
		}
	})
	return args
}
