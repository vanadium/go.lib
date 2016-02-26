// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"v.io/x/lib/gosh"
	"v.io/x/lib/gosh/internal/gosh_example_lib"
)

// Mirrors TestCmd in shell_test.go.
func ExampleCmd() {
	sh := gosh.NewShell(nil)
	defer sh.Cleanup()

	// Start server.
	binDir := sh.MakeTempDir()
	binPath := gosh.BuildGoPkg(sh, binDir, "v.io/x/lib/gosh/internal/gosh_example_server")
	c := sh.Cmd(binPath)
	c.Start()
	addr := c.AwaitVars("addr")["addr"]
	fmt.Println(addr)

	// Run client.
	binPath = gosh.BuildGoPkg(sh, binDir, "v.io/x/lib/gosh/internal/gosh_example_client")
	c = sh.Cmd(binPath, "-addr="+addr)
	fmt.Print(c.Stdout())
}

var (
	getFunc   = gosh.RegisterFunc("getFunc", lib.Get)
	serveFunc = gosh.RegisterFunc("serveFunc", lib.Serve)
)

// Mirrors TestFuncCmd in shell_test.go.
func ExampleFuncCmd() {
	sh := gosh.NewShell(nil)
	defer sh.Cleanup()

	// Start server.
	c := sh.FuncCmd(serveFunc)
	c.Start()
	addr := c.AwaitVars("addr")["addr"]
	fmt.Println(addr)

	// Run client.
	c = sh.FuncCmd(getFunc, addr)
	fmt.Print(c.Stdout())
}

func main() {
	gosh.InitMain()
	ExampleCmd()
	ExampleFuncCmd()
}
