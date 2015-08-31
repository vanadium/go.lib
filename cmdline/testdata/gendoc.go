// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command gendoc can be used for generating detailed godoc comments for
// cmdline-based tools.  The user specifies the cmdline-based tool source file
// directory <dir> using the first command-line argument and gendoc executes the
// tool with flags that generate detailed godoc comment and output it to
// <dir>/doc.go.  If more than one command-line argument is provided, they are
// passed through to the tool the gendoc executes.
//
// NOTE: The reason this command is located under a testdata directory is to
// enforce its idiomatic use through "go run <path>/testdata/gendoc.go <dir>
// [args]".
//
// NOTE: The gendoc command itself is not based on the cmdline library to avoid
// non-trivial bootstrapping.  In particular, if the compilation of gendoc
// requires GOPATH to contain the vanadium Go workspaces, then running the
// gendoc command requires the v23 tool, which in turn may depend on the gendoc
// command.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var flagTags string

func main() {
	flag.StringVar(&flagTags, "tags", "", "Tags for go build, also added as build constraints in the generated doc.go.")
	flag.Parse()
	if err := generate(flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(args []string) error {
	if got, want := len(args), 1; got < want {
		return fmt.Errorf("gendoc requires at least one argument\nusage: gendoc <dir> [args]")
	}
	pkg, args := args[0], args[1:]

	// Find out the binary name from the pkg name.
	var listOut bytes.Buffer
	listCmd := exec.Command("go", "list")
	listCmd.Stdout = &listOut
	if err := listCmd.Run(); err != nil {
		return fmt.Errorf("%q failed: %v\n%v\n", strings.Join(listCmd.Args, " "), err, listOut.String())
	}
	binName := filepath.Base(strings.TrimSpace(listOut.String()))

	// Install the gendoc binary in a temporary folder.
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("TempDir() failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	gendocBin := filepath.Join(tmpDir, binName)
	env := environ()
	env = append(env, "GOBIN="+tmpDir)
	installArgs := []string{"go", "install", "-tags=" + flagTags, pkg}
	installCmd := exec.Command("v23", installArgs...)
	installCmd.Env = env
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("%q failed: %v\n", strings.Join(installCmd.Args, " "), err)
	}

	// Use it to generate the documentation.
	var tagsConstraint string
	if flagTags != "" {
		tagsConstraint = fmt.Sprintf("// +build %s\n\n", flagTags)
	}
	var out bytes.Buffer
	if len(args) == 0 {
		args = []string{"help", "..."}
	}
	runCmd := exec.Command(gendocBin, args...)
	runCmd.Stdout = &out
	runCmd.Env = environ()
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("%q failed: %v\n%v\n", strings.Join(runCmd.Args, " "), err, out.String())
	}
	doc := fmt.Sprintf(`// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

%s/*
%s*/
package main
`, tagsConstraint, suppressParallelFlag(out.String()))

	// Write the result to doc.go.
	path, perm := filepath.Join(pkg, "doc.go"), os.FileMode(0644)
	if err := ioutil.WriteFile(path, []byte(doc), perm); err != nil {
		return fmt.Errorf("WriteFile(%v, %v) failed: %v\n", path, perm, err)
	}
	return nil
}

// suppressParallelFlag replaces the default value of the test.parallel flag
// with the literal string "<number of threads>". The default value of the
// test.parallel flag is GOMAXPROCS, which (since Go1.5) is set to the number
// of logical CPU threads on the current system. This causes problems with the
// vanadium-go-generate test, which requires that the output of gendoc is the
// same on all systems.
func suppressParallelFlag(input string) string {
	pattern := regexp.MustCompile("(?m:(^ -test\\.parallel=)(?:\\d)+$)")
	return pattern.ReplaceAllString(input, "$1<number of threads>")
}

// environ returns the environment variables to use when running the command to
// retrieve full help information.
func environ() []string {
	var env []string
	for _, e := range os.Environ() {
		// Strip out all existing CMDLINE_* envvars to start with a clean slate.
		// E.g. otherwise if CMDLINE_PREFIX is set, it'll taint all of the output.
		if !strings.HasPrefix(e, "CMDLINE_") {
			env = append(env, e)
		}
	}
	// We want the godoc style for our generated documentation.
	env = append(env, "CMDLINE_STYLE=godoc")
	return env
}
