// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows
// +build !windows

package lookpath

import (
	"os"
)

func ExecutableFileNameForTests(filename string, perm os.FileMode) string {
	return filename
}
