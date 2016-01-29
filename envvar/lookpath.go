// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package envvar

import (
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
)

// LookPath returns the absolute path of the executable with the given name,
// based on the given dirs.  If multiple exectuables match the name, the first
// match in dirs is returned.  Invalid dirs are silently ignored.
func LookPath(dirs []string, name string) string {
	if strings.Contains(name, string(filepath.Separator)) {
		return ""
	}
	for _, dir := range dirs {
		fileInfos, err := ioutil.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, fileInfo := range fileInfos {
			if m := fileInfo.Mode(); !m.IsRegular() || (m&0111 == 0) {
				continue
			}
			if fileInfo.Name() == name {
				return filepath.Join(dir, name)
			}
		}
	}
	return ""
}

// LookPathAll returns the absolute paths of all executables with the given name
// prefix, based on the given dirs.  If multiple exectuables match the prefix
// with the same name, the first match in dirs is returned.  Invalid dirs are
// silently ignored.  Returns a list of paths sorted by name.
//
// The names are filled in as the method runs, to ensure the first matching
// property.  As a consequence, you may pass in a pre-populated names map to
// prevent matching those names.  It is fine to pass in a nil names map.
func LookPathAll(dirs []string, prefix string, names map[string]bool) []string {
	if strings.Contains(prefix, string(filepath.Separator)) {
		return nil
	}
	if names == nil {
		names = make(map[string]bool)
	}
	var all []string
	for _, dir := range dirs {
		fileInfos, err := ioutil.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, fileInfo := range fileInfos {
			if m := fileInfo.Mode(); !m.IsRegular() || (m&0111 == 0) {
				continue
			}
			name, prefixLen := fileInfo.Name(), len(prefix)
			if len(name) < prefixLen || name[:prefixLen] != prefix {
				continue
			}
			if names[name] {
				continue
			}
			names[name] = true
			all = append(all, filepath.Join(dir, name))
		}
	}
	sort.Sort(byBase(all))
	return all
}

type byBase []string

func (x byBase) Len() int           { return len(x) }
func (x byBase) Less(i, j int) bool { return filepath.Base(x[i]) < filepath.Base(x[j]) }
func (x byBase) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
