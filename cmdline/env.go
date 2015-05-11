// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmdline

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"v.io/x/lib/textutil"
)

// NewEnv returns a new environment with defaults based on the operating system.
func NewEnv() *Env {
	return &Env{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
}

// Env represents the environment for command parsing and running.  Typically
// NewEnv is used to produce a default environment.  The environment may be
// explicitly set for finer control; e.g. in tests.
type Env struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// TODO(toddw): Add env vars, using a new "v.io/x/lib/envvar" package.
	// Vars envvar.Vars

	// Usage is a function that prints usage information to w.  Typically set by
	// calls to Main or Parse to print usage of the leaf command.
	Usage func(w io.Writer)
}

// UsageErrorf prints the error message represented by the printf-style format
// and args, followed by the output of the Usage function.  Returns ErrUsage to
// make it easy to use from within the Runner.Run function.
func (e *Env) UsageErrorf(format string, args ...interface{}) error {
	return usageErrorf(e.Stderr, e.Usage, format, args...)
}

func usageErrorf(w io.Writer, usage func(io.Writer), format string, args ...interface{}) error {
	fmt.Fprint(w, "ERROR: ")
	fmt.Fprintf(w, format, args...)
	fmt.Fprint(w, "\n\n")
	if usage != nil {
		usage(w)
	} else {
		fmt.Fprint(w, "usage error\n")
	}
	return ErrUsage
}

// defaultWidth is a reasonable default for the output width in runes.
const defaultWidth = 80

func (e *Env) width() int {
	// TODO(toddw): Replace os.Getenv with lookup in env.Vars.
	if width, err := strconv.Atoi(os.Getenv("CMDLINE_WIDTH")); err == nil && width != 0 {
		return width
	}
	if _, width, err := textutil.TerminalSize(); err == nil && width != 0 {
		return width
	}
	return defaultWidth
}

func (e *Env) style() style {
	// TODO(toddw): Replace os.Getenv with lookup in env.Vars.
	style := styleCompact
	style.Set(os.Getenv("CMDLINE_STYLE"))
	return style
}

// style describes the formatting style for usage descriptions.
type style int

const (
	styleCompact style = iota // Default style, good for compact cmdline output.
	styleFull                 // Similar to compact but shows global flags.
	styleGoDoc                // Style good for godoc processing.
)

func (s *style) String() string {
	switch *s {
	case styleCompact:
		return "compact"
	case styleFull:
		return "full"
	case styleGoDoc:
		return "godoc"
	default:
		panic(fmt.Errorf("unhandled style %d", *s))
	}
}

// Set implements the flag.Value interface method.
func (s *style) Set(value string) error {
	switch value {
	case "compact":
		*s = styleCompact
	case "full":
		*s = styleFull
	case "godoc":
		*s = styleGoDoc
	default:
		return fmt.Errorf("unknown style %q", value)
	}
	return nil
}
