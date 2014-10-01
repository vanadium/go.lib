package cmdline

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var (
	errEcho           = errors.New("echo error")
	flagExtra         bool
	optNoNewline      bool
	flagTopLevelExtra bool
	globalFlag1       string
	globalFlag2       *int64
)

// runEcho is used to implement commands for our tests.
func runEcho(cmd *Command, args []string) error {
	if len(args) == 1 {
		if args[0] == "error" {
			return errEcho
		} else if args[0] == "bad_arg" {
			return cmd.UsageErrorf("Invalid argument %v", args[0])
		}
	}
	if flagExtra {
		args = append(args, "extra")
	}
	if flagTopLevelExtra {
		args = append(args, "tlextra")
	}
	if optNoNewline {
		fmt.Fprint(cmd.Stdout(), args)
	} else {
		fmt.Fprintln(cmd.Stdout(), args)
	}
	return nil
}

// runHello is another function for test commands.
func runHello(cmd *Command, args []string) error {
	if flagTopLevelExtra {
		args = append(args, "tlextra")
	}
	fmt.Fprintln(cmd.Stdout(), strings.Join(append([]string{"Hello"}, args...), " "))
	return nil
}

type testCase struct {
	Args        []string
	Err         error
	Stdout      string
	Stderr      string
	GlobalFlag1 string
	GlobalFlag2 int64
}

func init() {
	flag.StringVar(&globalFlag1, "global1", "", "global test flag 1")
	globalFlag2 = flag.Int64("global2", 0, "global test flag 2")
}

func matchOutput(actual, expect string) bool {
	// The global flags include the flags from the testing package, so strip them
	// out before the comparison.
	re := regexp.MustCompile("   -test.*\n")
	return re.ReplaceAllLiteralString(actual, "") == expect
}

func runTestCases(t *testing.T, cmd *Command, tests []testCase) {
	for _, test := range tests {
		// Reset global variables before running each test case.
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		flagExtra = false
		flagTopLevelExtra = false
		optNoNewline = false
		globalFlag1 = ""
		*globalFlag2 = 0

		// Run the execute function and check against expected results.
		cmd.Init(nil, &stdout, &stderr)
		if err := cmd.Execute(test.Args); err != test.Err {
			t.Errorf("Ran with args %q\nEXPECTED error:\n%q\nACTUAL error:\n%q", test.Args, test.Err, err)
		}
		if !matchOutput(stdout.String(), test.Stdout) {
			t.Errorf("Ran with args %q\nEXPECTED stdout:\n%q\nACTUAL stdout:\n%q", test.Args, test.Stdout, stdout.String())
		}
		if !matchOutput(stderr.String(), test.Stderr) {
			t.Errorf("Ran with args %q\nEXPECTED stderr:\n%q\nACTUAL stderr:\n%q", test.Args, test.Stderr, stderr.String())
		}
		if globalFlag1 != test.GlobalFlag1 {
			t.Errorf("Value for global1 flag %q\nEXPECTED %q", globalFlag1, test.GlobalFlag1)
		}
		if *globalFlag2 != test.GlobalFlag2 {
			t.Errorf("Value for global2 flag %q\nEXPECTED %q", globalFlag2, test.GlobalFlag2)
		}
	}
}

func TestNoCommands(t *testing.T) {
	cmd := &Command{
		Name:  "nocmds",
		Short: "Nocmds is invalid.",
		Long:  "Nocmds has no commands and no run function.",
	}

	var tests = []testCase{
		{
			Args: []string{},
			Err:  ErrUsage,
			Stderr: `ERROR: nocmds: neither Children nor Run is specified

Nocmds has no commands and no run function.

Usage:
   nocmds [ERROR: neither Children nor Run is specified]

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"foo"},
			Err:  ErrUsage,
			Stderr: `ERROR: nocmds: neither Children nor Run is specified

Nocmds has no commands and no run function.

Usage:
   nocmds [ERROR: neither Children nor Run is specified]

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
	}
	runTestCases(t, cmd, tests)
}

func TestOneCommand(t *testing.T) {
	cmdEcho := &Command{
		Name:  "echo",
		Short: "Print strings on stdout",
		Long: `
Echo prints any strings passed in to stdout.
`,
		Run:      runEcho,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be echoed.",
	}

	prog := &Command{
		Name:     "onecmd",
		Short:    "Onecmd program.",
		Long:     "Onecmd only has the echo command.",
		Children: []*Command{cmdEcho},
	}

	var tests = []testCase{
		{
			Args: []string{},
			Err:  ErrUsage,
			Stderr: `ERROR: onecmd: no command specified

Onecmd only has the echo command.

Usage:
   onecmd <command>

The onecmd commands are:
   echo        Print strings on stdout
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"foo"},
			Err:  ErrUsage,
			Stderr: `ERROR: onecmd: unknown command "foo"

Onecmd only has the echo command.

Usage:
   onecmd <command>

The onecmd commands are:
   echo        Print strings on stdout
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help"},
			Stdout: `Onecmd only has the echo command.

Usage:
   onecmd <command>

The onecmd commands are:
   echo        Print strings on stdout
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "echo"},
			Stdout: `Echo prints any strings passed in to stdout.

Usage:
   onecmd echo [strings]

[strings] are arbitrary strings that will be echoed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "help"},
			Stdout: `Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   onecmd help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "..."},
			Stdout: `Onecmd only has the echo command.

Usage:
   onecmd <command>

The onecmd commands are:
   echo        Print strings on stdout
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
================================================================================
Echo prints any strings passed in to stdout.

Usage:
   onecmd echo [strings]

[strings] are arbitrary strings that will be echoed.
================================================================================
Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   onecmd help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".
================================================================================
`,
		},
		{
			Args: []string{"help", "foo"},
			Err:  ErrUsage,
			Stderr: `ERROR: onecmd: unknown command "foo"

Onecmd only has the echo command.

Usage:
   onecmd <command>

The onecmd commands are:
   echo        Print strings on stdout
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args:   []string{"echo", "foo", "bar"},
			Stdout: "[foo bar]\n",
		},
		{
			Args: []string{"echo", "error"},
			Err:  errEcho,
		},
		{
			Args: []string{"echo", "bad_arg"},
			Err:  ErrUsage,
			Stderr: `ERROR: Invalid argument bad_arg

Echo prints any strings passed in to stdout.

Usage:
   onecmd echo [strings]

[strings] are arbitrary strings that will be echoed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
	}
	runTestCases(t, prog, tests)
}

func TestMultiCommands(t *testing.T) {
	cmdEcho := &Command{
		Run:   runEcho,
		Name:  "echo",
		Short: "Print strings on stdout",
		Long: `
Echo prints any strings passed in to stdout.
`,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be echoed.",
	}
	var cmdEchoOpt = &Command{
		Run:   runEcho,
		Name:  "echoopt",
		Short: "Print strings on stdout, with opts",
		// Try varying number of header/trailer newlines around the long description.
		Long: `Echoopt prints any args passed in to stdout.


`,
		ArgsName: "[args]",
		ArgsLong: "[args] are arbitrary strings that will be echoed.",
	}
	cmdEchoOpt.Flags.BoolVar(&optNoNewline, "n", false, "Do not output trailing newline")

	prog := &Command{
		Name:     "multi",
		Short:    "Multi test command",
		Long:     "Multi has two variants of echo.",
		Children: []*Command{cmdEcho, cmdEchoOpt},
	}
	prog.Flags.BoolVar(&flagExtra, "extra", false, "Print an extra arg")

	var tests = []testCase{
		{
			Args: []string{},
			Err:  ErrUsage,
			Stderr: `ERROR: multi: no command specified

Multi has two variants of echo.

Usage:
   multi [flags] <command>

The multi commands are:
   echo        Print strings on stdout
   echoopt     Print strings on stdout, with opts
   help        Display help for commands

The multi flags are:
   -extra=false: Print an extra arg

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help"},
			Stdout: `Multi has two variants of echo.

Usage:
   multi [flags] <command>

The multi commands are:
   echo        Print strings on stdout
   echoopt     Print strings on stdout, with opts
   help        Display help for commands

The multi flags are:
   -extra=false: Print an extra arg

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "..."},
			Stdout: `Multi has two variants of echo.

Usage:
   multi [flags] <command>

The multi commands are:
   echo        Print strings on stdout
   echoopt     Print strings on stdout, with opts
   help        Display help for commands

The multi flags are:
   -extra=false: Print an extra arg

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
================================================================================
Echo prints any strings passed in to stdout.

Usage:
   multi echo [strings]

[strings] are arbitrary strings that will be echoed.
================================================================================
Echoopt prints any args passed in to stdout.

Usage:
   multi echoopt [flags] [args]

[args] are arbitrary strings that will be echoed.

The echoopt flags are:
   -n=false: Do not output trailing newline
================================================================================
Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   multi help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".
================================================================================
`,
		},
		{
			Args: []string{"help", "echo"},
			Stdout: `Echo prints any strings passed in to stdout.

Usage:
   multi echo [strings]

[strings] are arbitrary strings that will be echoed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "echoopt"},
			Stdout: `Echoopt prints any args passed in to stdout.

Usage:
   multi echoopt [flags] [args]

[args] are arbitrary strings that will be echoed.

The echoopt flags are:
   -n=false: Do not output trailing newline

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "foo"},
			Err:  ErrUsage,
			Stderr: `ERROR: multi: unknown command "foo"

Multi has two variants of echo.

Usage:
   multi [flags] <command>

The multi commands are:
   echo        Print strings on stdout
   echoopt     Print strings on stdout, with opts
   help        Display help for commands

The multi flags are:
   -extra=false: Print an extra arg

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args:   []string{"echo", "foo", "bar"},
			Stdout: "[foo bar]\n",
		},
		{
			Args:   []string{"-extra", "echo", "foo", "bar"},
			Stdout: "[foo bar extra]\n",
		},
		{
			Args: []string{"echo", "error"},
			Err:  errEcho,
		},
		{
			Args:   []string{"echoopt", "foo", "bar"},
			Stdout: "[foo bar]\n",
		},
		{
			Args:   []string{"-extra", "echoopt", "foo", "bar"},
			Stdout: "[foo bar extra]\n",
		},
		{
			Args:   []string{"echoopt", "-n", "foo", "bar"},
			Stdout: "[foo bar]",
		},
		{
			Args:   []string{"-extra", "echoopt", "-n", "foo", "bar"},
			Stdout: "[foo bar extra]",
		},
		{
			Args:        []string{"-global1=globalStringValue", "-extra", "echoopt", "-n", "foo", "bar"},
			Stdout:      "[foo bar extra]",
			GlobalFlag1: "globalStringValue",
		},
		{
			Args:        []string{"-global2=42", "echoopt", "-n", "foo", "bar"},
			Stdout:      "[foo bar]",
			GlobalFlag2: 42,
		},
		{
			Args:        []string{"-global1=globalStringOtherValue", "-global2=43", "-extra", "echoopt", "-n", "foo", "bar"},
			Stdout:      "[foo bar extra]",
			GlobalFlag1: "globalStringOtherValue",
			GlobalFlag2: 43,
		},
		{
			Args: []string{"echoopt", "error"},
			Err:  errEcho,
		},
		{
			Args: []string{"echo", "-n", "foo", "bar"},
			Err:  ErrUsage,
			Stderr: `ERROR: flag provided but not defined: -n
Echo prints any strings passed in to stdout.

Usage:
   multi echo [strings]

[strings] are arbitrary strings that will be echoed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"-nosuchflag", "echo", "foo", "bar"},
			Err:  ErrUsage,
			Stderr: `ERROR: flag provided but not defined: -nosuchflag
Multi has two variants of echo.

Usage:
   multi [flags] <command>

The multi commands are:
   echo        Print strings on stdout
   echoopt     Print strings on stdout, with opts
   help        Display help for commands

The multi flags are:
   -extra=false: Print an extra arg

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
	}
	runTestCases(t, prog, tests)
}

func TestMultiLevelCommands(t *testing.T) {
	cmdEcho := &Command{
		Run:   runEcho,
		Name:  "echo",
		Short: "Print strings on stdout",
		Long: `
Echo prints any strings passed in to stdout.
`,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be echoed.",
	}
	cmdEchoOpt := &Command{
		Run:   runEcho,
		Name:  "echoopt",
		Short: "Print strings on stdout, with opts",
		// Try varying number of header/trailer newlines around the long description.
		Long: `Echoopt prints any args passed in to stdout.


`,
		ArgsName: "[args]",
		ArgsLong: "[args] are arbitrary strings that will be echoed.",
	}
	cmdEchoOpt.Flags.BoolVar(&optNoNewline, "n", false, "Do not output trailing newline")
	cmdHello := &Command{
		Run:   runHello,
		Name:  "hello",
		Short: "Print strings on stdout preceded by \"Hello\"",
		Long: `
Hello prints any strings passed in to stdout preceded by "Hello".
`,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be printed.",
	}
	echoProg := &Command{
		Name:     "echoprog",
		Short:    "Set of echo commands",
		Long:     "Echoprog has two variants of echo.",
		Children: []*Command{cmdEcho, cmdEchoOpt},
	}
	echoProg.Flags.BoolVar(&flagExtra, "extra", false, "Print an extra arg")
	prog := &Command{
		Name:     "toplevelprog",
		Short:    "Top level prog",
		Long:     "Toplevelprog has the echo subprogram and the hello command.",
		Children: []*Command{echoProg, cmdHello},
	}
	prog.Flags.BoolVar(&flagTopLevelExtra, "tlextra", false, "Print an extra arg for all commands")

	var tests = []testCase{
		{
			Args: []string{},
			Err:  ErrUsage,
			Stderr: `ERROR: toplevelprog: no command specified

Toplevelprog has the echo subprogram and the hello command.

Usage:
   toplevelprog [flags] <command>

The toplevelprog commands are:
   echoprog    Set of echo commands
   hello       Print strings on stdout preceded by "Hello"
   help        Display help for commands

The toplevelprog flags are:
   -tlextra=false: Print an extra arg for all commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help"},
			Stdout: `Toplevelprog has the echo subprogram and the hello command.

Usage:
   toplevelprog [flags] <command>

The toplevelprog commands are:
   echoprog    Set of echo commands
   hello       Print strings on stdout preceded by "Hello"
   help        Display help for commands

The toplevelprog flags are:
   -tlextra=false: Print an extra arg for all commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "..."},
			Stdout: `Toplevelprog has the echo subprogram and the hello command.

Usage:
   toplevelprog [flags] <command>

The toplevelprog commands are:
   echoprog    Set of echo commands
   hello       Print strings on stdout preceded by "Hello"
   help        Display help for commands

The toplevelprog flags are:
   -tlextra=false: Print an extra arg for all commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
================================================================================
Echoprog has two variants of echo.

Usage:
   toplevelprog echoprog [flags] <command>

The echoprog commands are:
   echo        Print strings on stdout
   echoopt     Print strings on stdout, with opts

The echoprog flags are:
   -extra=false: Print an extra arg
================================================================================
Echo prints any strings passed in to stdout.

Usage:
   toplevelprog echoprog echo [strings]

[strings] are arbitrary strings that will be echoed.
================================================================================
Echoopt prints any args passed in to stdout.

Usage:
   toplevelprog echoprog echoopt [flags] [args]

[args] are arbitrary strings that will be echoed.

The echoopt flags are:
   -n=false: Do not output trailing newline
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   toplevelprog hello [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   toplevelprog help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".
================================================================================
`,
		},
		{
			Args: []string{"help", "echoprog"},
			Stdout: `Echoprog has two variants of echo.

Usage:
   toplevelprog echoprog [flags] <command>

The echoprog commands are:
   echo        Print strings on stdout
   echoopt     Print strings on stdout, with opts
   help        Display help for commands

The echoprog flags are:
   -extra=false: Print an extra arg

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"echoprog", "help", "..."},
			Stdout: `Echoprog has two variants of echo.

Usage:
   toplevelprog echoprog [flags] <command>

The echoprog commands are:
   echo        Print strings on stdout
   echoopt     Print strings on stdout, with opts
   help        Display help for commands

The echoprog flags are:
   -extra=false: Print an extra arg

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
================================================================================
Echo prints any strings passed in to stdout.

Usage:
   toplevelprog echoprog echo [strings]

[strings] are arbitrary strings that will be echoed.
================================================================================
Echoopt prints any args passed in to stdout.

Usage:
   toplevelprog echoprog echoopt [flags] [args]

[args] are arbitrary strings that will be echoed.

The echoopt flags are:
   -n=false: Do not output trailing newline
================================================================================
Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   toplevelprog echoprog help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".
================================================================================
`,
		},
		{
			Args: []string{"echoprog", "help", "echoopt"},
			Stdout: `Echoopt prints any args passed in to stdout.

Usage:
   toplevelprog echoprog echoopt [flags] [args]

[args] are arbitrary strings that will be echoed.

The echoopt flags are:
   -n=false: Do not output trailing newline

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "hello"},
			Stdout: `Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   toplevelprog hello [strings]

[strings] are arbitrary strings that will be printed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "foo"},
			Err:  ErrUsage,
			Stderr: `ERROR: toplevelprog: unknown command "foo"

Toplevelprog has the echo subprogram and the hello command.

Usage:
   toplevelprog [flags] <command>

The toplevelprog commands are:
   echoprog    Set of echo commands
   hello       Print strings on stdout preceded by "Hello"
   help        Display help for commands

The toplevelprog flags are:
   -tlextra=false: Print an extra arg for all commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args:   []string{"echoprog", "echo", "foo", "bar"},
			Stdout: "[foo bar]\n",
		},
		{
			Args:   []string{"echoprog", "-extra", "echo", "foo", "bar"},
			Stdout: "[foo bar extra]\n",
		},
		{
			Args: []string{"echoprog", "echo", "error"},
			Err:  errEcho,
		},
		{
			Args:   []string{"echoprog", "echoopt", "foo", "bar"},
			Stdout: "[foo bar]\n",
		},
		{
			Args:   []string{"echoprog", "-extra", "echoopt", "foo", "bar"},
			Stdout: "[foo bar extra]\n",
		},
		{
			Args:   []string{"echoprog", "echoopt", "-n", "foo", "bar"},
			Stdout: "[foo bar]",
		},
		{
			Args:   []string{"echoprog", "-extra", "echoopt", "-n", "foo", "bar"},
			Stdout: "[foo bar extra]",
		},
		{
			Args: []string{"echoprog", "echoopt", "error"},
			Err:  errEcho,
		},
		{
			Args:   []string{"--tlextra", "echoprog", "-extra", "echoopt", "foo", "bar"},
			Stdout: "[foo bar extra tlextra]\n",
		},
		{
			Args:   []string{"hello", "foo", "bar"},
			Stdout: "Hello foo bar\n",
		},
		{
			Args:   []string{"--tlextra", "hello", "foo", "bar"},
			Stdout: "Hello foo bar tlextra\n",
		},
		{
			Args: []string{"hello", "--extra", "foo", "bar"},
			Err:  ErrUsage,
			Stderr: `ERROR: flag provided but not defined: -extra
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   toplevelprog hello [strings]

[strings] are arbitrary strings that will be printed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"-extra", "echoprog", "echoopt", "foo", "bar"},
			Err:  ErrUsage,
			Stderr: `ERROR: flag provided but not defined: -extra
Toplevelprog has the echo subprogram and the hello command.

Usage:
   toplevelprog [flags] <command>

The toplevelprog commands are:
   echoprog    Set of echo commands
   hello       Print strings on stdout preceded by "Hello"
   help        Display help for commands

The toplevelprog flags are:
   -tlextra=false: Print an extra arg for all commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
	}
	runTestCases(t, prog, tests)
}

func TestMultiLevelCommandsOrdering(t *testing.T) {
	cmdHello11 := &Command{
		Name:  "hello11",
		Short: "Print strings on stdout preceded by \"Hello\"",
		Long: `
Hello prints any strings passed in to stdout preceded by "Hello".
`,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be printed.",
		Run:      runHello,
	}
	cmdHello12 := &Command{
		Name:  "hello12",
		Short: "Print strings on stdout preceded by \"Hello\"",
		Long: `
Hello prints any strings passed in to stdout preceded by "Hello".
`,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be printed.",
		Run:      runHello,
	}
	cmdHello21 := &Command{
		Name:  "hello21",
		Short: "Print strings on stdout preceded by \"Hello\"",
		Long: `
Hello prints any strings passed in to stdout preceded by "Hello".
`,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be printed.",
		Run:      runHello,
	}
	cmdHello22 := &Command{
		Name:  "hello22",
		Short: "Print strings on stdout preceded by \"Hello\"",
		Long: `
Hello prints any strings passed in to stdout preceded by "Hello".
`,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be printed.",
		Run:      runHello,
	}
	cmdHello31 := &Command{
		Name:  "hello31",
		Short: "Print strings on stdout preceded by \"Hello\"",
		Long: `
Hello prints any strings passed in to stdout preceded by "Hello".
`,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be printed.",
		Run:      runHello,
	}
	cmdHello32 := &Command{
		Name:  "hello32",
		Short: "Print strings on stdout preceded by \"Hello\"",
		Long: `
Hello prints any strings passed in to stdout preceded by "Hello".
`,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be printed.",
		Run:      runHello,
	}
	progHello3 := &Command{
		Name:     "prog3",
		Short:    "Set of hello commands",
		Long:     "Prog3 has two variants of hello.",
		Children: []*Command{cmdHello31, cmdHello32},
	}
	progHello2 := &Command{
		Name:     "prog2",
		Short:    "Set of hello commands",
		Long:     "Prog2 has two variants of hello and a subprogram prog3.",
		Children: []*Command{cmdHello21, progHello3, cmdHello22},
	}
	progHello1 := &Command{
		Name:     "prog1",
		Short:    "Set of hello commands",
		Long:     "Prog1 has two variants of hello and a subprogram prog2.",
		Children: []*Command{cmdHello11, cmdHello12, progHello2},
	}

	var tests = []testCase{
		{
			Args: []string{},
			Err:  ErrUsage,
			Stderr: `ERROR: prog1: no command specified

Prog1 has two variants of hello and a subprogram prog2.

Usage:
   prog1 <command>

The prog1 commands are:
   hello11     Print strings on stdout preceded by "Hello"
   hello12     Print strings on stdout preceded by "Hello"
   prog2       Set of hello commands
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help"},
			Stdout: `Prog1 has two variants of hello and a subprogram prog2.

Usage:
   prog1 <command>

The prog1 commands are:
   hello11     Print strings on stdout preceded by "Hello"
   hello12     Print strings on stdout preceded by "Hello"
   prog2       Set of hello commands
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "..."},
			Stdout: `Prog1 has two variants of hello and a subprogram prog2.

Usage:
   prog1 <command>

The prog1 commands are:
   hello11     Print strings on stdout preceded by "Hello"
   hello12     Print strings on stdout preceded by "Hello"
   prog2       Set of hello commands
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 hello11 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 hello12 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Prog2 has two variants of hello and a subprogram prog3.

Usage:
   prog1 prog2 <command>

The prog2 commands are:
   hello21     Print strings on stdout preceded by "Hello"
   prog3       Set of hello commands
   hello22     Print strings on stdout preceded by "Hello"
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 hello21 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Prog3 has two variants of hello.

Usage:
   prog1 prog2 prog3 <command>

The prog3 commands are:
   hello31     Print strings on stdout preceded by "Hello"
   hello32     Print strings on stdout preceded by "Hello"
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 prog3 hello31 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 prog3 hello32 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 hello22 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   prog1 help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".
================================================================================
`,
		},
		{
			Args: []string{"prog2", "help", "..."},
			Stdout: `Prog2 has two variants of hello and a subprogram prog3.

Usage:
   prog1 prog2 <command>

The prog2 commands are:
   hello21     Print strings on stdout preceded by "Hello"
   prog3       Set of hello commands
   hello22     Print strings on stdout preceded by "Hello"
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 hello21 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Prog3 has two variants of hello.

Usage:
   prog1 prog2 prog3 <command>

The prog3 commands are:
   hello31     Print strings on stdout preceded by "Hello"
   hello32     Print strings on stdout preceded by "Hello"
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 prog3 hello31 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 prog3 hello32 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 hello22 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   prog1 prog2 help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".
================================================================================
`,
		},
		{
			Args: []string{"prog2", "prog3", "help", "..."},
			Stdout: `Prog3 has two variants of hello.

Usage:
   prog1 prog2 prog3 <command>

The prog3 commands are:
   hello31     Print strings on stdout preceded by "Hello"
   hello32     Print strings on stdout preceded by "Hello"
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 prog3 hello31 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 prog3 hello32 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   prog1 prog2 prog3 help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".
================================================================================
`,
		},
		{
			Args: []string{"help", "prog2", "prog3", "..."},
			Stdout: `Prog3 has two variants of hello.

Usage:
   prog1 prog2 prog3 <command>

The prog3 commands are:
   hello31     Print strings on stdout preceded by "Hello"
   hello32     Print strings on stdout preceded by "Hello"
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 prog3 hello31 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 prog3 hello32 [strings]

[strings] are arbitrary strings that will be printed.
================================================================================
Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   prog1 prog2 prog3 help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".
================================================================================
`,
		},
		{
			Args: []string{"help", "-style=godoc", "..."},
			Stdout: `Prog1 has two variants of hello and a subprogram prog2.

Usage:
   prog1 <command>

The prog1 commands are:
   hello11     Print strings on stdout preceded by "Hello"
   hello12     Print strings on stdout preceded by "Hello"
   prog2       Set of hello commands
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2

Prog1 Hello11

Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 hello11 [strings]

[strings] are arbitrary strings that will be printed.

Prog1 Hello12

Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 hello12 [strings]

[strings] are arbitrary strings that will be printed.

Prog1 Prog2

Prog2 has two variants of hello and a subprogram prog3.

Usage:
   prog1 prog2 <command>

The prog2 commands are:
   hello21     Print strings on stdout preceded by "Hello"
   prog3       Set of hello commands
   hello22     Print strings on stdout preceded by "Hello"

Prog1 Prog2 Hello21

Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 hello21 [strings]

[strings] are arbitrary strings that will be printed.

Prog1 Prog2 Prog3

Prog3 has two variants of hello.

Usage:
   prog1 prog2 prog3 <command>

The prog3 commands are:
   hello31     Print strings on stdout preceded by "Hello"
   hello32     Print strings on stdout preceded by "Hello"

Prog1 Prog2 Prog3 Hello31

Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 prog3 hello31 [strings]

[strings] are arbitrary strings that will be printed.

Prog1 Prog2 Prog3 Hello32

Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 prog3 hello32 [strings]

[strings] are arbitrary strings that will be printed.

Prog1 Prog2 Hello22

Hello prints any strings passed in to stdout preceded by "Hello".

Usage:
   prog1 prog2 hello22 [strings]

[strings] are arbitrary strings that will be printed.

Prog1 Help

Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   prog1 help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".

`,
		},
	}

	runTestCases(t, progHello1, tests)
}

func TestCommandAndArgs(t *testing.T) {
	cmdEcho := &Command{
		Name:  "echo",
		Short: "Print strings on stdout",
		Long: `
Echo prints any strings passed in to stdout.
`,
		Run:      runEcho,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be echoed.",
	}

	prog := &Command{
		Name:     "cmdargs",
		Short:    "Cmdargs program.",
		Long:     "Cmdargs has the echo command and a Run function with args.",
		Children: []*Command{cmdEcho},
		Run:      runHello,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be printed.",
	}

	var tests = []testCase{
		{
			Args:   []string{},
			Stdout: "Hello\n",
		},
		{
			Args:   []string{"foo"},
			Stdout: "Hello foo\n",
		},
		{
			Args: []string{"help"},
			Stdout: `Cmdargs has the echo command and a Run function with args.

Usage:
   cmdargs <command>
   cmdargs [strings]

The cmdargs commands are:
   echo        Print strings on stdout
   help        Display help for commands

[strings] are arbitrary strings that will be printed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "echo"},
			Stdout: `Echo prints any strings passed in to stdout.

Usage:
   cmdargs echo [strings]

[strings] are arbitrary strings that will be echoed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "..."},
			Stdout: `Cmdargs has the echo command and a Run function with args.

Usage:
   cmdargs <command>
   cmdargs [strings]

The cmdargs commands are:
   echo        Print strings on stdout
   help        Display help for commands

[strings] are arbitrary strings that will be printed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
================================================================================
Echo prints any strings passed in to stdout.

Usage:
   cmdargs echo [strings]

[strings] are arbitrary strings that will be echoed.
================================================================================
Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   cmdargs help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".
================================================================================
`,
		},
		{
			Args: []string{"help", "foo"},
			Err:  ErrUsage,
			Stderr: `ERROR: cmdargs: unknown command "foo"

Cmdargs has the echo command and a Run function with args.

Usage:
   cmdargs <command>
   cmdargs [strings]

The cmdargs commands are:
   echo        Print strings on stdout
   help        Display help for commands

[strings] are arbitrary strings that will be printed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args:   []string{"echo", "foo", "bar"},
			Stdout: "[foo bar]\n",
		},
		{
			Args: []string{"echo", "error"},
			Err:  errEcho,
		},
		{
			Args: []string{"echo", "bad_arg"},
			Err:  ErrUsage,
			Stderr: `ERROR: Invalid argument bad_arg

Echo prints any strings passed in to stdout.

Usage:
   cmdargs echo [strings]

[strings] are arbitrary strings that will be echoed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
	}
	runTestCases(t, prog, tests)
}

func TestCommandAndRunNoArgs(t *testing.T) {
	cmdEcho := &Command{
		Name:  "echo",
		Short: "Print strings on stdout",
		Long: `
Echo prints any strings passed in to stdout.
`,
		Run:      runEcho,
		ArgsName: "[strings]",
		ArgsLong: "[strings] are arbitrary strings that will be echoed.",
	}

	prog := &Command{
		Name:     "cmdrun",
		Short:    "Cmdrun program.",
		Long:     "Cmdrun has the echo command and a Run function with no args.",
		Children: []*Command{cmdEcho},
		Run:      runHello,
	}

	var tests = []testCase{
		{
			Args:   []string{},
			Stdout: "Hello\n",
		},
		{
			Args: []string{"foo"},
			Err:  ErrUsage,
			Stderr: `ERROR: cmdrun: unknown command "foo"

Cmdrun has the echo command and a Run function with no args.

Usage:
   cmdrun <command>
   cmdrun

The cmdrun commands are:
   echo        Print strings on stdout
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help"},
			Stdout: `Cmdrun has the echo command and a Run function with no args.

Usage:
   cmdrun <command>
   cmdrun

The cmdrun commands are:
   echo        Print strings on stdout
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "echo"},
			Stdout: `Echo prints any strings passed in to stdout.

Usage:
   cmdrun echo [strings]

[strings] are arbitrary strings that will be echoed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args: []string{"help", "..."},
			Stdout: `Cmdrun has the echo command and a Run function with no args.

Usage:
   cmdrun <command>
   cmdrun

The cmdrun commands are:
   echo        Print strings on stdout
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
================================================================================
Echo prints any strings passed in to stdout.

Usage:
   cmdrun echo [strings]

[strings] are arbitrary strings that will be echoed.
================================================================================
Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   cmdrun help [flags] [command ...]

[command ...] is an optional sequence of commands to display detailed usage.
The special-case "help ..." recursively displays help for all commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".
================================================================================
`,
		},
		{
			Args: []string{"help", "foo"},
			Err:  ErrUsage,
			Stderr: `ERROR: cmdrun: unknown command "foo"

Cmdrun has the echo command and a Run function with no args.

Usage:
   cmdrun <command>
   cmdrun

The cmdrun commands are:
   echo        Print strings on stdout
   help        Display help for commands

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
		{
			Args:   []string{"echo", "foo", "bar"},
			Stdout: "[foo bar]\n",
		},
		{
			Args: []string{"echo", "error"},
			Err:  errEcho,
		},
		{
			Args: []string{"echo", "bad_arg"},
			Err:  ErrUsage,
			Stderr: `ERROR: Invalid argument bad_arg

Echo prints any strings passed in to stdout.

Usage:
   cmdrun echo [strings]

[strings] are arbitrary strings that will be echoed.

The global flags are:
   -global1=: global test flag 1
   -global2=0: global test flag 2
`,
		},
	}
	runTestCases(t, prog, tests)
}
