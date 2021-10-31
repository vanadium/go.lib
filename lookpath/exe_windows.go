// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package lookpath

import (
	"os"
	"path/filepath"
	"strings"
)

func isExecutablePath(dir, base string) (string, bool) {
	if strings.HasSuffix(base, ".exe") {
		file, err := filepath.Abs(filepath.Join(dir, base))
		return file, err == nil
	}
	file, err := filepath.Abs(filepath.Join(dir, base+".exe"))
	if err != nil {
		return "", false
	}
	info, err := os.Stat(file)
	return file, err == nil && !info.Mode().IsDir()
}

func isExecutable(info os.FileInfo) bool {
	return strings.HasSuffix(info.Name(), ".exe")
}

// PathEnvVar is the system specific environment variable name for command
// paths; Path on Windows systems.
const PathEnvVar = "Path"

// ExecutableFilename returns a system specific filename for executable
// files. On Windows a '.exe' suffix is appended.
func ExecutableFilename(name string) string {
	return strings.TrimSuffix(name, ".exe") + ".exe"
}

// ExecutableBasename returns the system specific basename (i.e. without
// any executable suffix) for executable files.
// On Windows a '.exe' suffix is remove.
func ExecutableBasename(name string) string {
	return strings.TrimSuffix(name, ".exe")
}

// translateEnv translates commonly used environment variables to their
// system specific equivalents, e.g. the commonly used PATH on UNIX
// systems to Path on Windows.
func translateEnv(env map[string]string) map[string]string {
	if p, ok := env["PATH"]; ok {
		nenv := make(map[string]string, len(env))
		for k, v := range env {
			nenv[k] = v
		}
		nenv[PathEnvVar] = p
		return nenv
	}
	return env
}
