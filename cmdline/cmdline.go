// Package cmdline provides a data-driven framework to simplify writing
// command-line programs.  It includes built-in support for formatted help.
//
// Commands may be linked together to form a command tree.  Since commands may
// be arbitrarily nested within other commands, it's easy to create wrapper
// programs that invoke existing commands.
//
// The syntax for each command-line program is:
//
//   command [flags] [subcommand [flags]]* [args]
//
// Each sequence of flags on the command-line is associated with the command
// that immediately precedes them.  Global flags registered with the standard
// flags package are allowed anywhere a command-specific flag is allowed.
package cmdline

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

// ErrExitCode may be returned by the Run function of a Command to cause the
// program to exit with a specific error code.
type ErrExitCode int

func (x ErrExitCode) Error() string {
	return fmt.Sprintf("exit code %d", x)
}

// ErrUsage is returned to indicate an error in command usage; e.g. unknown
// flags, subcommands or args.  It corresponds to exit code 1.
const ErrUsage = ErrExitCode(1)

// Command represents a single command in a command-line program.  A program
// with subcommands is represented as a root Command with children representing
// each subcommand.  The command graph must be a tree; each command may either
// have exactly one parent (a sub-command), or no parent (the root), and cycles
// are not allowed.  This makes it easier to display the usage for subcommands.
type Command struct {
	Name     string       // Name of the command.
	Short    string       // Short description, shown in help called on parent.
	Long     string       // Long description, shown in help called on itself.
	Flags    flag.FlagSet // Flags for the command.
	ArgsName string       // Name of the args, shown in usage line.
	ArgsLong string       // Long description of the args, shown in help.

	// Children of the command.  The framework will match args[0] against each
	// child's name, and call Run on the first matching child.
	Children []*Command

	// Topics that provide additional info via the default help command.
	Topics []Topic

	// Run is a function that runs cmd with args.  If both Children and Run are
	// specified, Run will only be called if none of the children match.  It is an
	// error if neither is specified.  The special ErrExitCode error may be
	// returned to indicate the command should exit with a specific exit code.
	Run func(cmd *Command, args []string) error

	// parent holds the parent of this Command, or nil if this is the root.
	parent *Command

	// stdout and stderr are set through Init.
	stdout, stderr io.Writer

	// parseFlags holds the merged flags used for parsing.  Each command starts
	// with its own Flags, and we merge in all global flags.  If the same flag is
	// specified in both sets, the command's own flag wins.
	parseFlags *flag.FlagSet

	// isDefaultHelp indicates whether this is the the default help command
	// provided by the framework.
	isDefaultHelp bool

	// TODO(toddw): If necessary we can add alias support, e.g. for abbreviations.
	//   Alias map[string]string
}

// Topic represents an additional help topic that is accessed via the default
// help command.
type Topic struct {
	Name  string // Name of the topic.
	Short string // Short description, shown in help for the command.
	Long  string // Long description, shown in help for this topic.
}

// style describes the formatting style for usage descriptions.
type style int

const (
	styleText  style = iota // Default style, good for cmdline output.
	styleGoDoc              // Style good for godoc processing.
)

// String returns the human-readable representation of the style.
func (s *style) String() string {
	switch *s {
	case styleText:
		return "text"
	case styleGoDoc:
		return "godoc"
	default:
		panic(fmt.Errorf("Unhandled style %d", *s))
	}
}

// Set implements the flag.Value interface method.
func (s *style) Set(value string) error {
	switch value {
	case "text":
		*s = styleText
	case "godoc":
		*s = styleGoDoc
	default:
		return fmt.Errorf("Unknown style %q", value)
	}
	return nil
}

// Stdout is where output goes.  Typically os.Stdout.
func (cmd *Command) Stdout() io.Writer {
	return cmd.stdout
}

// Stderr is where error messages go.  Typically os.Stderr
func (cmd *Command) Stderr() io.Writer {
	return cmd.stderr
}

// UsageErrorf prints the error message represented by the printf-style format
// string and args, followed by the usage description of cmd.  Returns ErrUsage
// to make it easy to use from within the cmd.Run function.
func (cmd *Command) UsageErrorf(format string, v ...interface{}) error {
	fmt.Fprint(cmd.stderr, "ERROR: ")
	fmt.Fprintf(cmd.stderr, format, v...)
	fmt.Fprint(cmd.stderr, "\n\n")
	cmd.usage(cmd.stderr, true)
	return ErrUsage
}

// usage prints the usage of cmd to the writer.  The firstCall boolean is set to
// false when printing usage for multiple commands, and is used to avoid
// printing redundant information (e.g. help command, global flags).
func (cmd *Command) usage(w io.Writer, firstCall bool) {
	printLong(w, cmd.Long)
	// Usage line.
	hasFlags := numFlags(&cmd.Flags) > 0
	fmt.Fprintf(w, "\nUsage:\n")
	path := namePath(cmd)
	pathf := "   " + path
	if hasFlags {
		pathf += " [flags]"
	}
	if len(cmd.Children) > 0 {
		fmt.Fprintf(w, "%s <command>\n", pathf)
	}
	if cmd.Run != nil {
		if cmd.ArgsName != "" {
			fmt.Fprintf(w, "%s %s\n", pathf, cmd.ArgsName)
		} else {
			fmt.Fprintf(w, "%s\n", pathf)
		}
	}
	if len(cmd.Children) == 0 && cmd.Run == nil {
		// This is a specification error.
		fmt.Fprintf(w, "%s [ERROR: neither Children nor Run is specified]\n", pathf)
	}
	// Commands.
	if len(cmd.Children) > 0 {
		fmt.Fprintf(w, "\nThe %s commands are:\n", cmd.Name)
		for _, child := range cmd.Children {
			// Don't repeatedly list default help command.
			if !child.isDefaultHelp || firstCall {
				fmt.Fprintf(w, "   %-11s %s\n", child.Name, child.Short)
			}
		}
		if firstCall {
			fmt.Fprintf(w, "Run \"%s help [command]\" for command usage.\n", path)
		}
	}
	// Args.
	if cmd.Run != nil && cmd.ArgsLong != "" {
		fmt.Fprintln(w)
		printLong(w, cmd.ArgsLong)
	}
	// Help topics.
	if len(cmd.Topics) > 0 {
		fmt.Fprintf(w, "\nThe %s additional help topics are:\n", cmd.Name)
		for _, topic := range cmd.Topics {
			fmt.Fprintf(w, "   %-11s %s\n", topic.Name, topic.Short)
		}
		if firstCall {
			fmt.Fprintf(w, "Run \"%s help [topic]\" for topic details.\n", path)
		}
	}
	// Flags.
	if hasFlags {
		fmt.Fprintf(w, "\nThe %s flags are:\n", cmd.Name)
		printFlags(w, &cmd.Flags)
	}
	// Global flags.
	if numFlags(flag.CommandLine) > 0 && firstCall {
		fmt.Fprintf(w, "\nThe global flags are:\n")
		printFlags(w, flag.CommandLine)
	}
}

// namePath returns the path of command names up to cmd.
func namePath(cmd *Command) string {
	var path []string
	for ; cmd != nil; cmd = cmd.parent {
		path = append([]string{cmd.Name}, path...)
	}
	return strings.Join(path, " ")
}

func printLong(w io.Writer, long string) {
	fmt.Fprint(w, strings.Trim(long, "\n"))
	fmt.Fprintln(w)
}

func numFlags(set *flag.FlagSet) (num int) {
	set.VisitAll(func(*flag.Flag) {
		num++
	})
	return
}

func printFlags(w io.Writer, set *flag.FlagSet) {
	set.VisitAll(func(f *flag.Flag) {
		fmt.Fprintf(w, "   -%s=%s: %s\n", f.Name, f.DefValue, f.Usage)
	})
}

// newDefaultHelp creates a new default help command.  We need to create new
// instances since the parent for each help command is different.
func newDefaultHelp() *Command {
	helpStyle := styleText
	help := &Command{
		Name:  helpName,
		Short: "Display help for commands or topics",
		Long: `
Help with no args displays the usage of the parent command.
Help with args displays the usage of the specified sub-command or help topic.
"help ..." recursively displays help for all commands and topics.
`,
		ArgsName: "[command/topic ...]",
		ArgsLong: `
[command/topic ...] optionally identifies a specific sub-command or help topic.
`,
		Run: func(cmd *Command, args []string) error {
			// Help applies to its parent - e.g. "foo help" applies to the foo command.
			return runHelp(cmd.stdout, cmd.parent, args, helpStyle)
		},
		isDefaultHelp: true,
	}
	help.Flags.Var(&helpStyle, "style", `The formatting style for help output, either "text" or "godoc".`)
	return help
}

const helpName = "help"

// runHelp runs the "help" command.
func runHelp(w io.Writer, cmd *Command, args []string, style style) error {
	if len(args) == 0 {
		cmd.usage(w, true)
		return nil
	}
	if args[0] == "..." {
		recursiveHelp(w, cmd, style, true)
		return nil
	}
	// Try to display help for the subcommand.
	subName, subArgs := args[0], args[1:]
	for _, child := range cmd.Children {
		if child.Name == subName {
			return runHelp(w, child, subArgs, style)
		}
	}
	// Try to display help for the help topic.
	for _, topic := range cmd.Topics {
		if topic.Name == subName {
			printLong(w, topic.Long)
			return nil
		}
	}
	return cmd.UsageErrorf("%s: unknown command or topic %q", cmd.Name, subName)
}

// recursiveHelp prints help recursively via DFS from this cmd onward.
func recursiveHelp(w io.Writer, cmd *Command, style style, firstCall bool) {
	if !firstCall {
		// Title-case required for godoc to recognize this as a section header.
		header := strings.Title(namePath(cmd))
		lineBreak(w, style)
		fmt.Fprintf(w, "%s\n\n", header)
	}
	cmd.usage(w, firstCall)
	for _, child := range cmd.Children {
		// Don't repeatedly print default help command.
		if !child.isDefaultHelp || firstCall {
			recursiveHelp(w, child, style, false)
		}
	}
	for _, topic := range cmd.Topics {
		// Title-case required for godoc to recognize this as a section header.
		header := strings.Title(namePath(cmd)+" "+topic.Name) + " - help topic"
		lineBreak(w, style)
		fmt.Fprintf(w, "%s\n\n", header)
		printLong(w, topic.Long)
	}
}

func lineBreak(w io.Writer, style style) {
	switch style {
	case styleText:
		fmt.Fprintln(w, strings.Repeat("=", 80))
	case styleGoDoc:
		fmt.Fprintln(w)
	}
}

// Init initializes all nodes in the command tree rooted at cmd.  Init must be
// called before Execute.
func (cmd *Command) Init(parent *Command, stdout, stderr io.Writer) {
	cmd.parent = parent
	cmd.stdout = stdout
	cmd.stderr = stderr
	// Add help command, if it doesn't already exist.
	hasHelp := false
	for _, child := range cmd.Children {
		if child.Name == helpName {
			hasHelp = true
			break
		}
	}
	if !hasHelp && cmd.Name != helpName && len(cmd.Children) > 0 {
		cmd.Children = append(cmd.Children, newDefaultHelp())
	}
	// Merge command-specific and global flags into parseFlags.  We want to handle
	// all error output ourselves, so we:
	//   1) Set flag.ContinueOnError so that Parse() doesn't exit or panic.
	//   2) Discard all output (can't be nil, that means stderr).
	//   3) Set an empty Usage function (can't be nil, that means default).
	cmd.parseFlags = flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
	cmd.parseFlags.SetOutput(ioutil.Discard)
	cmd.parseFlags.Usage = emptyUsage
	mergeFlags(cmd.parseFlags, &cmd.Flags)
	mergeFlags(cmd.parseFlags, flag.CommandLine)
	// Call children recursively.
	for _, child := range cmd.Children {
		child.Init(cmd, stdout, stderr)
	}
}

func mergeFlags(dst, src *flag.FlagSet) {
	src.VisitAll(func(f *flag.Flag) {
		if dst.Lookup(f.Name) == nil {
			dst.Var(f.Value, f.Name, f.Usage)
		}
	})
}

func emptyUsage() {}

// Execute the command with the given args.  The returned error is ErrUsage if
// there are usage errors, otherwise it is whatever the leaf command returns
// from its Run function.
func (cmd *Command) Execute(args []string) error {
	// Parse the merged flags.
	if err := cmd.parseFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			cmd.usage(cmd.stdout, true)
			return nil
		}
		return cmd.UsageErrorf(err.Error())
	}
	args = cmd.parseFlags.Args()
	// Look for matching children.
	if len(args) > 0 {
		subName, subArgs := args[0], args[1:]
		for _, child := range cmd.Children {
			if child.Name == subName {
				return child.Execute(subArgs)
			}
		}
	}
	// No matching children, try Run.
	if cmd.Run != nil {
		if cmd.ArgsName == "" && len(args) > 0 {
			if len(cmd.Children) > 0 {
				return cmd.UsageErrorf("%s: unknown command %q", cmd.Name, args[0])
			}
			return cmd.UsageErrorf("%s doesn't take any arguments", cmd.Name)
		}
		return cmd.Run(cmd, args)
	}
	switch {
	case len(cmd.Children) == 0:
		return cmd.UsageErrorf("%s: neither Children nor Run is specified", cmd.Name)
	case len(args) > 0:
		return cmd.UsageErrorf("%s: unknown command %q", cmd.Name, args[0])
	default:
		return cmd.UsageErrorf("%s: no command specified", cmd.Name)
	}
}

// Main executes the command tree rooted at cmd, writing output to os.Stdout,
// writing errors to os.Stderr, and getting args from os.Args.  We'll call
// os.Exit with a non-zero exit code on errors.  It's meant as a simple
// one-liner for the main function of command-line tools.
func (cmd *Command) Main() {
	cmd.Init(nil, os.Stdout, os.Stderr)
	if err := cmd.Execute(os.Args[1:]); err != nil {
		if code, ok := err.(ErrExitCode); ok {
			os.Exit(int(code))
		}
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
}
