// Command gendoc can be used for generating detailed godoc comments
// for cmdline-based tools. The user specifies the cmdline-based tool
// source file directory <dir> using the first command-line argument
// and gendoc executes the tool with flags that generate detailed
// godoc comment and output it to <dir>/doc.go. If more than one
// command-line argument is provided, they are passed through to the
// tool the gendoc executes.
//
// NOTE: The reason this command is located in under a testdata
// directory is to enforce its idiomatic use through "go run
// <path>/testdata/gendoc.go <dir> [args]".
//
// NOTE: The gendoc command itself is not based on the cmdline library
// to avoid non-trivial bootstrapping. In particular, if the
// compilation of gendoc requires GOPATH to contain the vanadium Go
// workspaces, then running the gendoc command requires the v23 tool,
// which in turn my depend on the gendoc command.
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	if got, want := len(os.Args[1:]), 1; got < want {
		return fmt.Errorf("gendoc requires at least one argument\nusage: gendoc <dir> [args]")
	}
	pkg := os.Args[1]

	// Build the gendoc binary in a temporary folder.
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("TempDir() failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	gendocBin := filepath.Join(tmpDir, "gendoc")
	args := []string{"go", "build", "-o", gendocBin}
	args = append(args, pkg)
	buildCmd := exec.Command("v23", args...)
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("%q failed: %v\n", strings.Join(buildCmd.Args, " "), err)
	}

	// Use it to generate the documentation.
	var out bytes.Buffer
	env := os.Environ()
	if len(os.Args) == 2 {
		args = []string{"help", "-style=godoc", "..."}
	} else {
		args = os.Args[2:]
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

/*
%s*/
package main
`, out.String())

	// Write the result to doc.go.
	path, perm := filepath.Join(pkg, "doc.go"), os.FileMode(0644)
	if err := ioutil.WriteFile(path, []byte(doc), perm); err != nil {
		return fmt.Errorf("WriteFile(%v, %v) failed: %v\n", path, perm, err)
	}
	return nil
}
