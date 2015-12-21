// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"v.io/x/lib/gosh"
	"v.io/x/lib/gosh/internal/gosh_example_lib"
)

func ExampleCmds() {
	sh := gosh.NewShell(gosh.Opts{})
	defer sh.Cleanup()

	// Start server.
	binPath := sh.BuildGoPkg("v.io/x/lib/gosh/internal/gosh_example_server")
	c := sh.Cmd(binPath)
	c.Start()
	c.AwaitReady()
	addr := c.AwaitVars("Addr")["Addr"]
	fmt.Println(addr)

	// Run client.
	binPath = sh.BuildGoPkg("v.io/x/lib/gosh/internal/gosh_example_client")
	c = sh.Cmd(binPath, "-addr="+addr)
	fmt.Print(c.Stdout())
}

var (
	getFn   = gosh.Register("get", lib.Get)
	serveFn = gosh.Register("serve", lib.Serve)
)

func ExampleFns() {
	sh := gosh.NewShell(gosh.Opts{})
	defer sh.Cleanup()

	// Start server.
	c := sh.Fn(serveFn)
	c.Start()
	c.AwaitReady()
	addr := c.AwaitVars("Addr")["Addr"]
	fmt.Println(addr)

	// Run client.
	c = sh.Fn(getFn, addr)
	fmt.Print(c.Stdout())
}

func ExampleShellMain() {
	sh := gosh.NewShell(gosh.Opts{})
	defer sh.Cleanup()

	c := sh.Main(lib.HelloWorldMain)
	fmt.Print(c.Stdout())
}

func main() {
	gosh.MaybeRunFnAndExit()
	ExampleCmds()
	ExampleFns()
	ExampleShellMain()
}
