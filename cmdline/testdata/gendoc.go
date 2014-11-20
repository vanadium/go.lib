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
// compilation of gendoc requires GOPATH to contain the veyron
// workspaces, then running the gendoc command requires the veyron
// tool, which in turn my depend on the gendoc command.
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
	if got, want := len(os.Args[1:]), 1; got < want {
		fmt.Fprintln(os.Stderr, "gendoc requires at least one argument")
		fmt.Fprintln(os.Stderr, "usage: gendoc <dir> [args]")
		os.Exit(1)
	}

	dir := os.Args[1]

	doc := `// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
`
	fileInfoList, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ReadDir(%v) failed: %v\n", dir, err)
		os.Exit(1)
	}
	goFiles := []string{}
	for _, fileInfo := range fileInfoList {
		if !fileInfo.Mode().IsRegular() {
			continue
		}
		if strings.HasSuffix(fileInfo.Name(), ".go") && !strings.HasSuffix(fileInfo.Name(), "_test.go") {
			goFiles = append(goFiles, fileInfo.Name())
		}
	}
	var out bytes.Buffer
	args := []string{"go", "run"}
	args = append(args, goFiles...)
	if len(os.Args) == 2 {
		args = append(args, "help", "-style=godoc", "...")
	} else {
		args = append(args, os.Args[2:]...)
	}
	runCmd := exec.Command("veyron", args...)
	runCmd.Stdout = &out
	if err := runCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%q failed: %v\n%v\n", strings.Join(runCmd.Args, " "), out.String(), err)
		os.Exit(1)
	}
	doc += out.String()
	doc += `*/
package main
`
	path, perm := filepath.Join(dir, "doc.go"), os.FileMode(0644)
	if err := ioutil.WriteFile(path, []byte(doc), perm); err != nil {
		fmt.Fprintf(os.Stderr, "WriteFile(%v, %v) failed: %v\n", path, perm, err)
		os.Exit(1)
	}
}
