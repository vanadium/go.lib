// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package vlog_test

import (
	"os"
	"testing"

	"v.io/x/ref/test"
	"v.io/x/ref/test/modules"
)

func TestMain(m *testing.M) {
	test.Init()
	modules.DispatchAndExitIfChild()
	os.Exit(m.Run())
}
