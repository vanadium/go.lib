// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package envvar

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func mkdir(t *testing.T, d ...string) string {
	path := filepath.Join(d...)
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func mkfile(t *testing.T, dir, file string, perm os.FileMode) string {
	path := filepath.Join(dir, file)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func initTmpDir(t *testing.T) (string, func()) {
	tmpDir, err := ioutil.TempDir("", "envvar_lookpath")
	if err != nil {
		t.Fatal(err)
	}
	return tmpDir, func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Error(err)
		}
	}
}

func TestLookPath(t *testing.T) {
	tmpDir, cleanup := initTmpDir(t)
	defer cleanup()
	dirA, dirB := mkdir(t, tmpDir, "a"), mkdir(t, tmpDir, "b")
	aFoo, aBar := mkfile(t, dirA, "foo", 0755), mkfile(t, dirA, "bar", 0755)
	bBar, bBaz := mkfile(t, dirB, "bar", 0755), mkfile(t, dirB, "baz", 0755)
	_, bExe := mkfile(t, dirA, "exe", 0644), mkfile(t, dirB, "exe", 0755)
	tests := []struct {
		Dirs []string
		Name string
		Want string
	}{
		{nil, "", ""},
		{nil, "foo", ""},
		{[]string{dirA}, "foo", aFoo},
		{[]string{dirA}, "bar", aBar},
		{[]string{dirA}, "baz", ""},
		{[]string{dirB}, "foo", ""},
		{[]string{dirB}, "bar", bBar},
		{[]string{dirB}, "baz", bBaz},
		{[]string{dirA, dirB}, "foo", aFoo},
		{[]string{dirA, dirB}, "bar", aBar},
		{[]string{dirA, dirB}, "baz", bBaz},
		// Make sure we find bExe, since aExe isn't executable
		{[]string{dirA, dirB}, "exe", bExe},
	}
	for _, test := range tests {
		if got, want := LookPath(test.Dirs, test.Name), test.Want; got != want {
			t.Errorf("dirs=%v name=%v got %v, want %v", test.Dirs, test.Name, got, want)
		}
	}
}

func TestLookPathAll(t *testing.T) {
	tmpDir, cleanup := initTmpDir(t)
	defer cleanup()
	dirA, dirB := mkdir(t, tmpDir, "a"), mkdir(t, tmpDir, "b")
	aFoo, aBar := mkfile(t, dirA, "foo", 0755), mkfile(t, dirA, "bar", 0755)
	bBar, bBaz := mkfile(t, dirB, "bar", 0755), mkfile(t, dirB, "baz", 0755)
	aBzz, bBaa := mkfile(t, dirA, "bzz", 0755), mkfile(t, dirB, "baa", 0755)
	_, bExe := mkfile(t, dirA, "exe", 0644), mkfile(t, dirB, "exe", 0755)
	tests := []struct {
		Dirs   []string
		Prefix string
		Names  map[string]bool
		Want   []string
	}{
		{nil, "", nil, nil},
		{nil, "foo", nil, nil},
		{[]string{dirA}, "foo", nil, []string{aFoo}},
		{[]string{dirA}, "bar", nil, []string{aBar}},
		{[]string{dirA}, "baz", nil, nil},
		{[]string{dirA}, "f", nil, []string{aFoo}},
		{[]string{dirA}, "b", nil, []string{aBar, aBzz}},
		{[]string{dirB}, "foo", nil, nil},
		{[]string{dirB}, "bar", nil, []string{bBar}},
		{[]string{dirB}, "baz", nil, []string{bBaz}},
		{[]string{dirB}, "f", nil, nil},
		{[]string{dirB}, "b", nil, []string{bBaa, bBar, bBaz}},
		{[]string{dirA, dirB}, "foo", nil, []string{aFoo}},
		{[]string{dirA, dirB}, "bar", nil, []string{aBar}},
		{[]string{dirA, dirB}, "baz", nil, []string{bBaz}},
		{[]string{dirA, dirB}, "f", nil, []string{aFoo}},
		{[]string{dirA, dirB}, "b", nil, []string{bBaa, aBar, bBaz, aBzz}},
		// Don't find baz, since it's already provided.
		{[]string{dirA, dirB}, "b", map[string]bool{"baz": true}, []string{bBaa, aBar, aBzz}},
		// Make sure we find bExe, since aExe isn't executable
		{[]string{dirA, dirB}, "exe", nil, []string{bExe}},
		{[]string{dirA, dirB}, "e", nil, []string{bExe}},
	}
	for _, test := range tests {
		if got, want := LookPathAll(test.Dirs, test.Prefix, test.Names), test.Want; !reflect.DeepEqual(got, want) {
			t.Errorf("dirs=%v prefix=%v names=%v got %v, want %v", test.Dirs, test.Prefix, test.Names, got, want)
		}
	}
}
