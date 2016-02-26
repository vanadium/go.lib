// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"v.io/x/lib/gosh"
)

func TestPipeline(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	echo := sh.FuncCmd(echoFunc)
	echo.Args = append(echo.Args, "foo")
	replace := sh.FuncCmd(replaceFunc, byte('f'), byte('Z'))
	cat := sh.FuncCmd(catFunc)
	p := gosh.NewPipeline(echo, replace, cat)
	eq(t, p.Stdout(), "Zoo\n")
	eq(t, p.Clone().Stdout(), "Zoo\n")

	// Try piping only stdout.
	p = gosh.NewPipeline(sh.FuncCmd(writeFunc, true, true))
	p.PipeStdout(sh.FuncCmd(replaceFunc, byte('A'), byte('Z')))
	eq(t, p.Stdout(), "ZZ")
	eq(t, p.Clone().Stdout(), "ZZ")

	// Try piping only stderr.
	p = gosh.NewPipeline(sh.FuncCmd(writeFunc, true, true))
	p.PipeStderr(sh.FuncCmd(replaceFunc, byte('B'), byte('Z')))
	eq(t, p.Stdout(), "ZZ")
	eq(t, p.Clone().Stdout(), "ZZ")

	// Try piping both stdout and stderr.
	p = gosh.NewPipeline(sh.FuncCmd(writeFunc, true, true))
	p.PipeCombinedOutput(sh.FuncCmd(catFunc))
	// Note, we can't assume any particular ordering of stdout and stderr, so we
	// simply check the length of the combined output.
	eq(t, len(p.Stdout()), 4)
	eq(t, len(p.Clone().Stdout()), 4)

	// Try piping combinations.
	p = gosh.NewPipeline(sh.FuncCmd(writeFunc, true, true))
	p.PipeStderr(sh.FuncCmd(replaceFunc, byte('B'), byte('x')))
	p.PipeStdout(sh.FuncCmd(replaceFunc, byte('x'), byte('Z')))
	p.PipeStdout(sh.FuncCmd(catFunc))
	eq(t, p.Stdout(), "ZZ")
	eq(t, p.Clone().Stdout(), "ZZ")
}

func TestPipelineDifferentShells(t *testing.T) {
	sh1 := gosh.NewShell(t)
	defer sh1.Cleanup()
	sh2 := gosh.NewShell(t)
	defer sh2.Cleanup()

	setsErr(t, sh1, func() { gosh.NewPipeline(sh1.FuncCmd(echoFunc), sh2.FuncCmd(catFunc)) })
	setsErr(t, sh2, func() { gosh.NewPipeline(sh2.FuncCmd(echoFunc), sh1.FuncCmd(catFunc)) })
	p := gosh.NewPipeline(sh1.FuncCmd(echoFunc))
	setsErr(t, sh1, func() { p.PipeStdout(sh2.FuncCmd(catFunc)) })
	p = gosh.NewPipeline(sh1.FuncCmd(echoFunc))
	setsErr(t, sh1, func() { p.PipeStderr(sh2.FuncCmd(catFunc)) })
	p = gosh.NewPipeline(sh1.FuncCmd(echoFunc))
	setsErr(t, sh1, func() { p.PipeCombinedOutput(sh2.FuncCmd(catFunc)) })
}

func TestPipelineClosedPipe(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()
	writeLoop, readLine := sh.FuncCmd(writeLoopFunc), sh.FuncCmd(readFunc)

	// WriteLoop finishes because it gets a closed pipe write error after readLine
	// finishes. Note that the closed pipe error is ignored.
	p := gosh.NewPipeline(writeLoop, readLine)
	eq(t, p.Stdout(), "")
	ok(t, p.Cmds()[0].Err)
	ok(t, p.Cmds()[1].Err)
	p = p.Clone()
	eq(t, p.Stdout(), "")
	ok(t, p.Cmds()[0].Err)
	ok(t, p.Cmds()[1].Err)
}

func TestPipelineCmdFailure(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()
	cat := sh.FuncCmd(catFunc)
	exit1 := sh.FuncCmd(exitFunc, 1)
	writeLoop := sh.FuncCmd(writeLoopFunc)
	echoFoo := sh.FuncCmd(echoFunc)
	echoFoo.Args = append(echoFoo.Args, "foo")

	// Exit1 fails, and cat finishes with success since it sees an EOF.
	p := gosh.NewPipeline(exit1.Clone(), cat.Clone())
	setsErr(t, sh, p.Run)
	nok(t, p.Cmds()[0].Err)
	ok(t, p.Cmds()[1].Err)
	p = p.Clone()
	setsErr(t, sh, p.Run)
	nok(t, p.Cmds()[0].Err)
	ok(t, p.Cmds()[1].Err)
	ok(t, sh.Err)

	// Exit1 fails, and echoFoo finishes with success since it ignores stdin.
	p = gosh.NewPipeline(exit1.Clone(), echoFoo.Clone())
	setsErr(t, sh, p.Run)
	nok(t, p.Cmds()[0].Err)
	ok(t, p.Cmds()[1].Err)
	p = p.Clone()
	setsErr(t, sh, p.Run)
	nok(t, p.Cmds()[0].Err)
	ok(t, p.Cmds()[1].Err)
	ok(t, sh.Err)

	// Exit1 fails, causing writeLoop to finish and succeed.
	p = gosh.NewPipeline(writeLoop.Clone(), exit1.Clone())
	setsErr(t, sh, p.Run)
	ok(t, p.Cmds()[0].Err)
	nok(t, p.Cmds()[1].Err)
	p = p.Clone()
	setsErr(t, sh, p.Run)
	ok(t, p.Cmds()[0].Err)
	nok(t, p.Cmds()[1].Err)
	ok(t, sh.Err)

	// Same tests, but allowing the exit error from exit1.
	exit1.ExitErrorIsOk = true

	// Exit1 fails, and cat finishes with success since it sees an EOF.
	p = gosh.NewPipeline(exit1.Clone(), cat.Clone())
	eq(t, p.Stdout(), "")
	nok(t, p.Cmds()[0].Err)
	ok(t, p.Cmds()[1].Err)
	ok(t, sh.Err)
	p = p.Clone()
	eq(t, p.Stdout(), "")
	nok(t, p.Cmds()[0].Err)
	ok(t, p.Cmds()[1].Err)
	ok(t, sh.Err)

	// Exit1 fails, and echoFoo finishes with success since it ignores stdin.
	p = gosh.NewPipeline(exit1.Clone(), echoFoo.Clone())
	eq(t, p.Stdout(), "foo\n")
	nok(t, p.Cmds()[0].Err)
	ok(t, p.Cmds()[1].Err)
	ok(t, sh.Err)
	p = p.Clone()
	eq(t, p.Stdout(), "foo\n")
	nok(t, p.Cmds()[0].Err)
	ok(t, p.Cmds()[1].Err)
	ok(t, sh.Err)

	// Exit1 fails, causing writeLoop to finish and succeed.
	p = gosh.NewPipeline(writeLoop.Clone(), exit1.Clone())
	eq(t, p.Stdout(), "")
	ok(t, p.Cmds()[0].Err)
	nok(t, p.Cmds()[1].Err)
	ok(t, sh.Err)
	p = p.Clone()
	eq(t, p.Stdout(), "")
	ok(t, p.Cmds()[0].Err)
	nok(t, p.Cmds()[1].Err)
	ok(t, sh.Err)
}

func TestPipelineSignal(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	for _, d := range []time.Duration{0, time.Hour} {
		for _, s := range []os.Signal{os.Interrupt, os.Kill} {
			fmt.Println(d, s)
			p := gosh.NewPipeline(sh.FuncCmd(sleepFunc, d, 0), sh.FuncCmd(sleepFunc, d, 0))
			p.Start()
			p.Cmds()[0].AwaitVars("ready")
			p.Cmds()[1].AwaitVars("ready")
			// Wait for a bit to allow the zero-sleep commands to exit.
			time.Sleep(100 * time.Millisecond)
			p.Signal(s)
			switch {
			case s == os.Interrupt:
				// Wait should succeed as long as the exit code was 0, regardless of
				// whether the signal arrived or the processes had already exited.
				p.Wait()
			case d != 0:
				// Note: We don't call Wait in the {d: 0, s: os.Kill} case because doing
				// so makes the test flaky on slow systems.
				setsErr(t, sh, func() { p.Wait() })
			}
		}
	}

	// Signal should fail if Wait has been called.
	z := time.Duration(0)
	p := gosh.NewPipeline(sh.FuncCmd(sleepFunc, z, 0), sh.FuncCmd(sleepFunc, z, 0))
	p.Run()
	setsErr(t, sh, func() { p.Signal(os.Interrupt) })
}

func TestPipelineTerminate(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	for _, d := range []time.Duration{0, time.Hour} {
		for _, s := range []os.Signal{os.Interrupt, os.Kill} {
			fmt.Println(d, s)
			p := gosh.NewPipeline(sh.FuncCmd(sleepFunc, d, 0), sh.FuncCmd(sleepFunc, d, 0))
			p.Start()
			p.Cmds()[0].AwaitVars("ready")
			p.Cmds()[1].AwaitVars("ready")
			// Wait for a bit to allow the zero-sleep commands to exit.
			time.Sleep(100 * time.Millisecond)
			// Terminate should succeed regardless of the exit code, and regardless of
			// whether the signal arrived or the processes had already exited.
			p.Terminate(s)
		}
	}

	// Terminate should fail if Wait has been called.
	z := time.Duration(0)
	p := gosh.NewPipeline(sh.FuncCmd(sleepFunc, z, 0), sh.FuncCmd(sleepFunc, z, 0))
	p.Run()
	setsErr(t, sh, func() { p.Terminate(os.Interrupt) })
}
