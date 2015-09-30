// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmdline

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"v.io/x/lib/envvar"
	"v.io/x/lib/textutil"
)

// EnvFromOS returns a new environment based on the operating system.
func EnvFromOS() *Env {
	return &Env{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Vars:   envvar.SliceToMap(os.Environ()),
	}
}

// Env represents the environment for command parsing and running.  Typically
// EnvFromOS is used to produce a default environment.  The environment may be
// explicitly set for finer control; e.g. in tests.
type Env struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Vars   map[string]string // Environment variables

	// Usage is a function that prints usage information to w.  Typically set by
	// calls to Main or Parse to print usage of the leaf command.
	Usage func(env *Env, w io.Writer)
}

// UsageErrorf prints the error message represented by the printf-style format
// and args, followed by the output of the Usage function.  Returns ErrUsage to
// make it easy to use from within the Runner.Run function.
func (e *Env) UsageErrorf(format string, args ...interface{}) error {
	return usageErrorf(e, e.Usage, format, args...)
}

// Clone creates a deep copy of Env.
func (e *Env) clone() *Env {
	return &Env{
		Stdin:  e.Stdin,
		Stdout: e.Stdout,
		Stderr: e.Stderr,
		Vars:   envvar.CopyMap(e.Vars),
		Usage:  e.Usage,
	}
}

func usageErrorf(env *Env, usage func(*Env, io.Writer), format string, args ...interface{}) error {
	fmt.Fprint(env.Stderr, "ERROR: ")
	fmt.Fprintf(env.Stderr, format, args...)
	fmt.Fprint(env.Stderr, "\n\n")
	if usage != nil {
		usage(env, env.Stderr)
	} else {
		fmt.Fprint(env.Stderr, "usage error\n")
	}
	return ErrUsage
}

// defaultWidth is a reasonable default for the output width in runes.
const defaultWidth = 80

func (e *Env) width() int {
	if width, err := strconv.Atoi(e.Vars["CMDLINE_WIDTH"]); err == nil && width != 0 {
		return width
	}
	if _, width, err := textutil.TerminalSize(); err == nil && width != 0 {
		return width
	}
	return defaultWidth
}

func (e *Env) style() style {
	style := styleCompact
	style.Set(e.Vars["CMDLINE_STYLE"])
	return style
}

func (e *Env) prefix() string {
	return e.Vars["CMDLINE_PREFIX"]
}

func (e *Env) pathDirs() []string {
	return strings.Split(e.Vars["PATH"], string(os.PathListSeparator))
}

func (e *Env) firstCall() bool {
	return e.Vars["CMDLINE_FIRST_CALL"] == ""
}

// style describes the formatting style for usage descriptions.
type style int

const (
	styleCompact style = iota // Default style, good for compact cmdline output.
	styleFull                 // Similar to compact but shows global flags.
	styleGoDoc                // Style good for godoc processing.
	styleShort                // Style good for displaying help of binary subcommands.
)

func (s *style) String() string {
	switch *s {
	case styleCompact:
		return "compact"
	case styleFull:
		return "full"
	case styleGoDoc:
		return "godoc"
	case styleShort:
		return "short"
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
	case "short":
		*s = styleShort
	default:
		return fmt.Errorf("unknown style %q", value)
	}
	return nil
}
