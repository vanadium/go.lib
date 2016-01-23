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

var (
	varsPrefix = []byte("<goshVars")
	varsSuffix = []byte("goshVars>")
)

// SendVars sends the given vars to the parent process. Writes a string of the
// form "<goshVars{ ... JSON-encoded vars ... }goshVars>\n" to stderr.
func SendVars(vars map[string]string) {
	data, err := json.Marshal(vars)
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(os.Stderr, "%s%s%s\n", varsPrefix, data, varsSuffix)
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
