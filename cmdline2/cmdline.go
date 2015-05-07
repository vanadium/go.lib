// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cmdline2 implements a data-driven mechanism for writing command-line
// programs with built-in support for help.
//
// Commands are linked together to form a command tree.  Since commands may be
// arbitrarily nested within other commands, it's easy to create wrapper
// programs that invoke existing commands.
//
// The syntax for each command-line program is:
//
//   command [flags] [subcommand [flags]]* [args]
//
// Each sequence of flags is associated with the command that immediately
// precedes it.  Flags registered on flag.CommandLine are considered global
// flags, and are allowed anywhere a command-specific flag is allowed.
//
// Pretty usage documentation is automatically generated, and accessible either
// via the standard -h / -help flags from the Go flag package, or a special help
// command.  The help command is automatically appended to commands that already
// have at least one child, and don't already have a "help" child.
//
// Pitfalls
//
// The cmdline2 package must be in full control of flag parsing.  Typically you
// call cmdline.Main in your main function, and flag parsing is taken care of.
// If a more complicated ordering is required, you can call cmdline.Parse and
// then handle any special initialization.
//
// The problem is that flags registered on the root command must be merged
// together with the global flags for the root command to be parsed.  If
// flag.Parse is called before cmdline.Main or cmdline.Parse, it will fail if
// any root command flags are specified on the command line.
package cmdline2

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	_ "v.io/x/lib/metadata" // for the -v23.metadata flag
)

// Command represents a single command in a command-line program.  A program
// with subcommands is represented as a root Command with children representing
// each subcommand.  The command graph must be a tree; each command may either
// have no parent (the root) or exactly one parent, and cycles are not allowed.
type Command struct {
	Name     string       // Name of the command.
	Short    string       // Short description, shown in help called on parent.
	Long     string       // Long description, shown in help called on itself.
	Flags    flag.FlagSet // Flags for the command.
	ArgsName string       // Name of the args, shown in usage line.
	ArgsLong string       // Long description of the args, shown in help.

	// Children of the command - set for non-leaf commands in the tree.
	// Exactly one of Children or Runner must be specified.
	Children []*Command

	// Runner that runs the command - set for leaf commands in the tree.
	// Use RunnerFunc to adapt regular functions into Runners.
	Runner Runner

	// Topics that provide additional info via the default help command.
	Topics []Topic
}

// Runner is the interface for running commands.  Return ErrExitCode to indicate
// the command should exit with a specific exit code.
type Runner interface {
	Run(env *Env, args []string) error
}

// RunnerFunc is an adapter that turns regular functions into Runners.
type RunnerFunc func(*Env, []string) error

// Run implements the Runner interface method by calling f(env, args).
func (f RunnerFunc) Run(env *Env, args []string) error {
	return f(env, args)
}

// Topic represents a help topic that is accessed via the help command.
type Topic struct {
	Name  string // Name of the topic.
	Short string // Short description, shown in help for the command.
	Long  string // Long description, shown in help for this topic.
}

// Main implements the main function for the command tree rooted at root.
//
// It initializes a new environment from the underlying operating system, parses
// os.Args[1:] against the root command, and runs the resulting runner.  Calls
// os.Exit with an exit code that is 0 for success, or non-zero for errors.
//
// Most main packages should be implemented as follows:
//
//   var root := &cmdline.Command{...}
//
//   func main() {
//     cmdline.Main(root)
//   }
func Main(root *Command) {
	env := NewEnv()
	err := ParseAndRun(root, env, os.Args[1:])
	os.Exit(ExitCode(err, env.Stderr))
}

// Parse parses args against the command tree rooted at root down to a leaf
// command.  A single path through the command tree is traversed, based on the
// sub-commands specified in args.  Global and command-specific flags are parsed
// as the tree is traversed.
//
// On success returns the runner corresponding to the leaf command, along with
// the args to pass to the runner.  In addition the env.Usage function is set to
// produce a usage message corresponding to the leaf command.
//
// Most main packages should just call Main.  Parse should only be used if
// special processing is required after parsing the args, and before the runner
// is run.  An example:
//
//   var root := &cmdline.Command{...}
//
//   func main() {
//     env := cmdline.NewEnv()
//     os.Exit(cmdline.ExitCode(parseAndRun(env), env.Stderr))
//   }
//
//   func parseAndRun(env *cmdline.Env) error {
//     runner, args, err := cmdline.Parse(env, root, os.Args[1:])
//     if err != nil {
//       return err
//     }
//     // ... perform initialization that might parse flags ...
//     return runner.Run(env, args)
//   }
//
// Parse sets flag.CommandLine to the root flag set that was parsed, so that
// subsequent calls to flag.Parsed return true.
func Parse(root *Command, env *Env, args []string) (Runner, []string, error) {
	// Set env.Usage to the usage of the root command, in case the parse fails.
	path := []*Command{root}
	env.Usage = makeHelpRunner(path, env, flag.CommandLine).usageFunc
	if err := cleanTree(path); err != nil {
		return nil, nil, err
	}
	cleanFlags(flag.CommandLine)
	runner, args, globals, err := root.parse(nil, env, args, flag.CommandLine)
	if err != nil {
		return nil, nil, err
	}
	flag.CommandLine = globals
	return runner, args, nil
}

// ParseAndRun is a convenience that calls Parse, and then calls Run on the
// returned runner with the given env and parsed args.
func ParseAndRun(root *Command, env *Env, args []string) error {
	runner, args, err := Parse(root, env, args)
	if err != nil {
		return err
	}
	return runner.Run(env, args)
}

func trimSpace(s *string) { *s = strings.TrimSpace(*s) }

func cleanTree(path []*Command) error {
	cmd, cmdPath := path[len(path)-1], pathName(path)
	trimSpace(&cmd.Name)
	trimSpace(&cmd.Short)
	trimSpace(&cmd.Long)
	trimSpace(&cmd.ArgsName)
	trimSpace(&cmd.ArgsLong)
	for tx := range cmd.Topics {
		trimSpace(&cmd.Topics[tx].Name)
		trimSpace(&cmd.Topics[tx].Short)
		trimSpace(&cmd.Topics[tx].Long)
	}
	cleanFlags(&cmd.Flags)
	// Check that exactly one of Children or Runner is specified, so that
	// subsequent code can rely on this invariant.
	switch hasC, hasR := len(cmd.Children) > 0, cmd.Runner != nil; {
	case hasC && hasR:
		return fmt.Errorf("%v: both Children and Run specified", cmdPath)
	case !hasC && !hasR:
		return fmt.Errorf("%v: neither Children nor Run specified", cmdPath)
	}
	for _, child := range cmd.Children {
		cleanTree(append(path, child))
	}
	return nil
}

func cleanFlags(flags *flag.FlagSet) {
	flags.VisitAll(func(f *flag.Flag) {
		trimSpace(&f.Usage)
	})
}

func pathName(path []*Command) string {
	name := path[0].Name
	for _, cmd := range path[1:] {
		name += " " + cmd.Name
	}
	return name
}

func (cmd *Command) parse(path []*Command, env *Env, args []string, globals *flag.FlagSet) (Runner, []string, *flag.FlagSet, error) {
	path = append(path, cmd)
	cmdPath := pathName(path)
	runHelp := makeHelpRunner(path, env, globals)
	env.Usage = runHelp.usageFunc
	// Parse the merged command-specific and global flags.
	flags := newSilentFlagSet(cmd.Name)
	mergeFlags(flags, &cmd.Flags)
	mergeFlags(flags, globals)
	switch err := flags.Parse(args); {
	case err == flag.ErrHelp:
		return runHelp, nil, flags, nil
	case err != nil:
		return nil, nil, nil, env.UsageErrorf("%s: %v", cmdPath, err)
	}
	// Look for matching children.
	args = flags.Args()
	if len(cmd.Children) > 0 {
		if len(args) == 0 {
			return nil, nil, nil, env.UsageErrorf("%s: no command specified", cmdPath)
		}
		subName, subArgs := args[0], args[1:]
		for _, child := range cmd.Children {
			if child.Name == subName {
				runner, args, _, err := child.parse(path, env, subArgs, globals)
				return runner, args, flags, err
			}
		}
		// Every non-leaf command gets a default help command.
		if helpName == subName {
			runner, args, _, err := runHelp.newCommand().parse(path, env, subArgs, globals)
			return runner, args, flags, err
		}
		return nil, nil, nil, env.UsageErrorf("%s: unknown command %q", cmdPath, subName)
	}
	// We've reached a leaf command.
	if len(args) > 0 && cmd.ArgsName == "" {
		return nil, nil, nil, env.UsageErrorf("%s: doesn't take arguments", cmdPath)
	}
	return cmd.Runner, args, flags, nil
}

func newSilentFlagSet(name string) *flag.FlagSet {
	// Silence the many different ways flags.Parse can produce ugly output; we
	// just want it to return any errors and handle the output ourselves.
	//   1) Set flag.ContinueOnError so that Parse() doesn't exit or panic.
	//   2) Discard all output (can't be nil, that means stderr).
	//   3) Set an empty Usage (can't be nil, that means use the default).
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(ioutil.Discard)
	flags.Usage = func() {}
	return flags
}

func mergeFlags(dst, src *flag.FlagSet) {
	src.VisitAll(func(f *flag.Flag) {
		// If there is a collision in flag names, the existing flag in dst wins.
		// Note that flag.Var will panic if it sees a collision.
		if dst.Lookup(f.Name) == nil {
			dst.Var(f.Value, f.Name, f.Usage)
		}
	})
}

// ErrExitCode may be returned by Runner.Run to cause the program to exit with a
// specific error code.
type ErrExitCode int

// Error implements the error interface method.
func (x ErrExitCode) Error() string {
	return fmt.Sprintf("exit code %d", x)
}

// ErrUsage indicates an error in command usage; e.g. unknown flags, subcommands
// or args.  It corresponds to exit code 2.
const ErrUsage = ErrExitCode(2)

// ExitCode returns the exit code corresponding to err.
//   0:    if err == nil
//   code: if err is ErrExitCode(code)
//   1:    all other errors
// Writes the error message for "all other errors" to w, if w is non-nil.
func ExitCode(err error, w io.Writer) int {
	if err == nil {
		return 0
	}
	if code, ok := err.(ErrExitCode); ok {
		return int(code)
	}
	if w != nil {
		// We don't print "ERROR: exit code N" above to avoid cluttering the output.
		fmt.Fprintf(w, "ERROR: %v\n", err)
	}
	return 1
}
