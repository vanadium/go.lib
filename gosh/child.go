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

const (
	msgPrefix = "#! "
	typeReady = "ready"
	typeVars  = "vars"
)

type msg struct {
	Type string
	Vars map[string]string // nil if Type is typeReady
}

func send(m msg) {
	data, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s%s\n", msgPrefix, data)
}

// SendReady tells the parent process that this child process is "ready", e.g.
// ready to serve requests.
func SendReady() {
	send(msg{Type: typeReady})
}

// SendVars sends the given vars to the parent process.
func SendVars(vars map[string]string) {
	send(msg{Type: typeVars, Vars: vars})
}

// WatchParent starts a goroutine that periodically checks whether the parent
// process has exited and, if so, kills the current process.
func WatchParent() {
	go func() {
		for {
			if os.Getppid() == 1 {
				log.Fatal("parent process has exited")
			}
			time.Sleep(time.Second)
		}
	}()
}

// MaybeWatchParent calls WatchParent iff this process was spawned by a
// gosh.Shell in the parent process.
func MaybeWatchParent() {
	if os.Getenv(envSpawnedByShell) != "" {
		// Our child processes should see envSpawnedByShell iff they were spawned by
		// a gosh.Shell in this process.
		os.Unsetenv(envSpawnedByShell)
		WatchParent()
	}
}
