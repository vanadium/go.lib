// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows
// +build !windows

package lookpath

import (
	"os"
	"path/filepath"
	"strings"
)

func isExecutablePath(dir, base string) (string, bool) {
	file, err := filepath.Abs(filepath.Join(dir, base))
	if err != nil {
		return "", false
	}
	info, err := os.Stat(file)
	if err != nil {
		return "", false
	}
	if !isExecutable(info) {
		return "", false
	}
	return file, true
}

func isExecutable(info os.FileInfo) bool {
	mode := info.Mode()
	return !mode.IsDir() && mode&0111 != 0
}

// PathEnvVar is the system specific environment variable name for command
// paths; commonly PATH on UNIX systems.
const PathEnvVar = "PATH"

// ExecutableFilename returns a system specific filename for executable
// files. On UNIX systems the filename is unchanged.
func ExecutableFilename(name string) string {
	return name
}

// ExecutableBasename returns the system specific basename (i.e. without
// any executable suffix) for executable files.
// On UNIX systems the filename is unchanged.
func ExecutableBasename(name string) string {
	return strings.TrimSuffix(name, ".exe")
}

// translateEnv translates commonly used environment variables to their
// system specific equivalents, e.g. the commonly used PATH on UNIX
// systems to Path on Windows.
func translateEnv(env map[string]string) map[string]string {
	return env
}
