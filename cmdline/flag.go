package cmdline

import (
	"flag"
	"os"
	"runtime"
)

// VariableFlag can be used to define a command-line flag whose
// default value contains variables whose value can change depending
// on the environment in which the tool is executed. Such flags need a
// special treatment to avoid changes to the auto-generated
// documentation, which lists the default values, every time the
// documentation is auto-generated using a different environment.
func VariableFlag(raw string, mapping func(string) string) flag.Getter {
	return &variableFlag{mapping, raw, os.Expand(raw, mapping)}
}

// RuntimeFlag substitutes variables defined by the Go runtime
// package with their values.
func RuntimeFlag(raw string) flag.Getter {
	return VariableFlag(raw, func(raw string) string {
		switch raw {
		case "GOARCH":
			return runtime.GOARCH
		case "GOOS":
			return runtime.GOOS
		default:
			return ""
		}
	})
}

// EnvFlag substitutes variables defined by the OS environment with
// their values.
func EnvFlag(raw string) flag.Getter {
	return VariableFlag(raw, os.Getenv)
}

type variableFlag struct {
	mapping func(string) string
	raw     string
	value   string
}

// Get implements the flag.Getter interface. It returns the flag value
// in which variables have been substituted by their values.
func (f *variableFlag) Get() interface{} {
	return f.value
}

// Set implements the flag.Getter interface. The raw value may contain
// variables and the function stores both the raw flag value and the
// parsed value in which variables have been substituted by their
// values.
func (f *variableFlag) Set(raw string) error {
	f.raw = raw
	f.value = os.Expand(raw, f.mapping)
	return nil
}

// String implements the flag.Getter interface. It returns the raw
// flag value, which may contain variables.
func (f *variableFlag) String() string {
	return f.raw
}
