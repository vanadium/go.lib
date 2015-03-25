// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"

	"v.io/x/lib/cmdline"
)

var (
	testFlag flag.Getter
)

func main() {
	os.Exit(root().Main())
}

func init() {
	os.Setenv("TEST", "HELLO")
	testFlag = cmdline.EnvFlag("${TEST}")
	cmdRoot.Flags.Var(testFlag, "test", "test flag")
}

// root returns a command that represents the root of the v23 tool.
func root() *cmdline.Command {
	return cmdRoot
}

// cmdRoot represents the root of the test command.
var cmdRoot = &cmdline.Command{
	Run:   runRoot,
	Name:  "test",
	Short: "test command",
	Long:  "test command.",
}

func runRoot(*cmdline.Command, []string) error {
	if got, want := testFlag.Get().(string), "HELLO"; got != want {
		return fmt.Errorf("unexpected value: got %v, want %v", got, want)
	}
	return nil
}
