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

	// Build the gendoc binary in a temporary folder.
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("TempDir() failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	gendocBin := filepath.Join(tmpDir, "gendoc")
	buildArgs := []string{"go", "build", "-a", "-tags=" + flagTags, "-o=" + gendocBin, pkg}
	buildCmd := exec.Command("v23", buildArgs...)
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("%q failed: %v\n", strings.Join(buildCmd.Args, " "), err)
	}

	// Use it to generate the documentation.
	var tagsConstraint string
	if flagTags != "" {
		tagsConstraint = fmt.Sprintf("// +build %s\n\n", flagTags)
	}
	var out bytes.Buffer
	env := os.Environ()
	if len(args) == 0 {
		args = []string{"help", "-style=godoc", "..."}
	} else {
		env = append(env, "CMDLINE_STYLE=godoc")
	}
	runCmd := exec.Command(gendocBin, args...)
	runCmd.Stdout = &out
	runCmd.Env = env
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("%q failed: %v\n%v\n", strings.Join(runCmd.Args, " "), err)
	}
	doc := fmt.Sprintf(`// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

%s/*
%s*/
package main
`, tagsConstraint, out.String())

	// Write the result to doc.go.
	path, perm := filepath.Join(pkg, "doc.go"), os.FileMode(0644)
	if err := ioutil.WriteFile(path, []byte(doc), perm); err != nil {
		return fmt.Errorf("WriteFile(%v, %v) failed: %v\n", path, perm, err)
	}
	return nil
}
