// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lookpath implements utilities to find executables.
package lookpath

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func splitPath(env map[string]string) []string {
	var dirs []string
	for _, dir := range strings.Split(env[PathEnvVar], string(filepath.ListSeparator)) {
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// Look returns the absolute path of the executable with the given name.  If
// name only contains a single path component, the dirs in env["PATH"]
// or env["Path"] on windows (use lookpath.PathEnvVar to obtain the os specific
// value) are consulted, and the first match is returned.  Otherwise, for
// multi-component paths, the absolute path of the name is looked up directly.
//
// The behavior is the same as LookPath in the os/exec package, but allows the
// env to be passed in explicitly.
// On Windows systems PATH is copied to Path in env unless Path is already
// defined. Again, on Windows, the returned executable name does not include
// the .exe suffix.
func Look(env map[string]string, name string) (string, error) {
	env = translateEnv(env)
	var dirs []string
	base := filepath.Base(name)
	if base == name {
		dirs = splitPath(env)
	} else {
		dirs = []string{filepath.Dir(name)}
	}
	fmt.Printf("DIRS: %v: %v\n", dirs, name)
	for _, dir := range dirs {
		fmt.Printf("DIR: %v\n", dir)
		if file, ok := isExecutablePath(dir, base); ok {
			fmt.Printf("YES... File: %v: %v\n", file, ok)
			return ExecutableBasename(file), nil
		} else {
			fmt.Printf("File: %v: %v\n", file, ok)
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

// LookPrefix returns the absolute paths of all executables with the given name
// prefix.  If prefix only contains a single path component, the directories in
// env["PATH"] are consulted.  Otherwise, for multi-component prefixes, only the
// directory containing the prefix is consulted.  If multiple executables with
// the same base name match the prefix in different directories, the first match
// is returned.  Returns a list of paths sorted by base name.
//
// The names are filled in as the method runs, to ensure the first matching
// property.  As a consequence, you may pass in a pre-populated names map to
// prevent matching those names.  It is fine to pass in a nil names map.
func LookPrefix(env map[string]string, prefix string, names map[string]bool) ([]string, error) {
	env = translateEnv(env)
	if names == nil {
		names = make(map[string]bool)
	}
	var dirs []string
	if filepath.Base(prefix) == prefix {
		dirs = splitPath(env)
	} else {
		dirs = []string{filepath.Dir(prefix)}
	}
	var all []string
	for _, dir := range dirs {
		dir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		infos, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, info := range infos {
			fsinfo, err := info.Info()
			if err != nil {
				return nil, err
			}
			if !isExecutable(fsinfo) {
				continue
			}
			name := info.Name()
			bprefix := filepath.Base(prefix)
			if !strings.HasPrefix(name, bprefix) {
				continue
			}
			name = ExecutableBasename(name)
			if names[name] {
				continue
			}
			names[name] = true
			all = append(all, filepath.Join(dir, name))
		}
	}
	if len(all) > 0 {
		sort.Sort(byBase(all))
		return all, nil
	}
	return nil, &exec.Error{Name: prefix + "*", Err: exec.ErrNotFound}
}

type byBase []string

func (x byBase) Len() int           { return len(x) }
func (x byBase) Less(i, j int) bool { return filepath.Base(x[i]) < filepath.Base(x[j]) }
func (x byBase) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
