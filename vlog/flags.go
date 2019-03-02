// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vlog

import (
	"flag"

	"v.io/x/lib/llog"
)

var (
	// Reexport the logging severity levels from the underlying llog package.
	InfoLog    = llog.InfoLog
	WarningLog = llog.WarningLog
	ErrorLog   = llog.ErrorLog
	FatalLog   = llog.FatalLog

	CommandLineLoggingFlags LoggingFlags
)

func init() {
	RegisterLoggingFlags(flag.CommandLine, &CommandLineLoggingFlags, "")
}

// LoggingFlags represents all of the flags that can be used to configure
// logging.
type LoggingFlags struct {
	ToStderr        bool
	AlsoToStderr    bool
	LogDir          string
	Verbosity       Level
	StderrThreshold StderrThreshold
	VModule         ModuleSpec
	VPath           FilepathSpec
	TraceLocation   TraceLocation
	MaxStackBufSize int
}

// RegisterLoggingFlags registers the logging flags with the specified
// flagset and with prefix prepepended to their flag names.
//   --<prefix>v  NOTE, see below
//   --<prefix>log_dir
//   --<prefix>logtostderr
//   --<prefix>alsologtostderr
//   --<prefix>max_stack_buf_size
//   --<prefix>stderrthreshold
//   --<prefix>vmodule
//   --<prefix>vpath
//   --<prefix>log_backtrace_at
//
// The verbosity flag is problematic with go test since it also uses --v
// or --test.v when the test is compiled. If --test.v is already defined
// RegisterLoggingFlags will use --<prefix>vlevel instead of -v.
func RegisterLoggingFlags(fs *flag.FlagSet, lf *LoggingFlags, prefix string) {
	vflag := prefix + "v"
	if fs.Lookup("test.v") != nil {
		vflag = prefix + "vlevel"
	}
	lf.StderrThreshold = StderrThreshold(llog.ErrorLog)
	fs.Var(&lf.Verbosity, vflag, "log level for V logs")
	fs.StringVar(&lf.LogDir, prefix+"log_dir", "", "if non-empty, write log files to this directory")
	fs.BoolVar(&lf.ToStderr, prefix+"logtostderr", false, "log to standard error instead of files")
	fs.BoolVar(&lf.AlsoToStderr, prefix+"alsologtostderr", true, "log to standard error as well as files")
	fs.IntVar(&lf.MaxStackBufSize, prefix+"max_stack_buf_size", 4192*1024, "max size in bytes of the buffer to use for logging stack traces")
	fs.Var(&lf.StderrThreshold, prefix+"stderrthreshold", "logs at or above this threshold go to stderr")
	fs.Var(&lf.VModule, prefix+"vmodule", "comma-separated list of globpattern=N settings for filename-filtered logging (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns baz or *az or b* but not by bar/baz or baz.go or az or b.*")
	fs.Var(&lf.VPath, prefix+"vpath", "comma-separated list of regexppattern=N settings for file pathname-filtered logging (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns foo/bar/baz or fo.*az or oo/ba or b.z but not by foo/bar/baz.go or fo*az")
	fs.Var(&lf.TraceLocation, prefix+"log_backtrace_at",
		"when logging hits line file:N, emit a stack trace")
}

// ConfigureLibraryLoggerFromFlags will configure the internal global logger
// using command line flags.  It assumes that flag.Parse() has already been
// called to initialize the flag variables.
func ConfigureLibraryLoggerFromFlags() error {
	return Log.ConfigureFromFlags()
}

// String implements string.Stringer.
func (l *Logger) String() string {
	return l.log.String()
}

// ConfigureFromLoggingFlags will configure the logger using the specified
// LoggingFlags.
func (l *Logger) ConfigureFromLoggingFlags(lf *LoggingFlags, opts ...LoggingOpts) error {
	all := []LoggingOpts{
		LogToStderr(lf.ToStderr),
		AlsoLogToStderr(lf.AlsoToStderr),
		LogDir(lf.LogDir),
		Level(lf.Verbosity),
		StderrThreshold(lf.StderrThreshold),
		ModuleSpec(lf.VModule),
		FilepathSpec(lf.VPath),
		TraceLocation(lf.TraceLocation),
		MaxStackBufSize(lf.MaxStackBufSize),
	}
	all = append(all, opts...)
	return l.Configure(all...)
}

// ConfigureFromFlags will configure the logger using command line flags.
func (l *Logger) ConfigureFromFlags(opts ...LoggingOpts) error {
	return l.ConfigureFromLoggingFlags(&CommandLineLoggingFlags, opts...)
}

// ConfigureFromArgs will configure the logger using the supplied args.
func (l *Logger) ConfigureFromArgs(opts ...LoggingOpts) error {
	return l.Configure(opts...)
}

var defaultFlags = []string{
	"log_dir",
	"logtostderr",
	"alsologtostderr",
	"max_stack_buf_size",
	"stderrthreshold",
	"vmodule",
	"vpath",
	"log_backtrace_at",
}

// ExplicitlySetFlags returns a map of the logging command line flags and their
// values formatted as strings.  Only the flags that were explicitly set are
// returned. This is intended for use when an application needs to know what
// value the flags were set to, for example when creating subprocesses.
func (l *Logger) ExplicitlySetFlags() map[string]string {
	vflag := "v"
	if flag.CommandLine.Lookup("test.v") != nil {
		vflag = "vlevel"
	}
	logFlagNames := make(map[string]bool, len(defaultFlags)+1)
	for _, name := range defaultFlags {
		logFlagNames[name] = true
	}
	logFlagNames[vflag] = true
	args := make(map[string]string)
	flag.Visit(func(f *flag.Flag) {
		if logFlagNames[f.Name] {
			args[f.Name] = f.Value.String()
		}
	})
	return args
}
