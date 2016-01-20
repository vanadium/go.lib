// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

// This file contains functions meant to be called from a child process.

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

const varsPrefix = "# gosh "

// SendVars sends the given vars to the parent process. Writes a line of the
// form "# gosh { ... JSON object ... }" to stderr.
func SendVars(vars map[string]string) {
	data, err := json.Marshal(vars)
	if err != nil {
		panic(err)
	}
	// TODO(sadovsky): Handle the case where the JSON object contains a newline
	// character.
	fmt.Fprintf(os.Stderr, "%s%s\n", varsPrefix, data)
}

// watchParent periodically checks whether the parent process has exited and, if
// so, kills the current process. Meant to be run in a goroutine.
func watchParent() {
	for {
		if os.Getppid() == 1 {
			log.Fatal("gosh: parent process has exited")
		}
		time.Sleep(time.Second)
	}
}

// exitAfter kills the current process once the given duration has elapsed.
// Meant to be run in a goroutine.
func exitAfter(d time.Duration) {
	time.Sleep(d)
	log.Fatalf("gosh: timed out after %v", d)
}

// InitChildMain must be called early on in main() of child processes. It spawns
// goroutines to kill the current process when certain conditions are met, per
// Cmd.IgnoreParentExit and Cmd.ExitAfter.
func InitChildMain() {
	if os.Getenv(envWatchParent) != "" {
		os.Unsetenv(envWatchParent)
		go watchParent()
	}
	if os.Getenv(envExitAfter) != "" {
		d, err := time.ParseDuration(envExitAfter)
		if err != nil {
			panic(err)
		}
		os.Unsetenv(envExitAfter)
		go exitAfter(d)
	}
}
