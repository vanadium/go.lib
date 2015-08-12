// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vlog_test

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"v.io/x/lib/vlog"
)

func ExampleConfigure() {
	vlog.Configure()
}

func ExampleInfo() {
	vlog.Info("hello")
}

func ExampleError() {
	vlog.Errorf("%s", "error")
	if vlog.V(2) {
		vlog.Info("some spammy message")
	}
	vlog.VI(2).Infof("another spammy message")
}

func readLogFiles(dir string) ([]string, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var contents []string
	for _, fi := range files {
		// Skip symlinks to avoid double-counting log lines.
		if !fi.Mode().IsRegular() {
			continue
		}
		file, err := os.Open(filepath.Join(dir, fi.Name()))
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if line := scanner.Text(); len(line) > 0 && line[0] == 'I' {
				contents = append(contents, line)
			}
		}
	}
	return contents, nil
}

func TestHeaders(t *testing.T) {
	dir, err := ioutil.TempDir("", "logtest")
	defer os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	logger := vlog.NewLogger("testHeader")
	logger.Configure(vlog.LogDir(dir), vlog.Level(2))
	logger.Infof("abc\n")
	logger.Infof("wombats\n")
	logger.VI(1).Infof("wombats again\n")
	logger.FlushLog()
	contents, err := readLogFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	fileRE := regexp.MustCompile(`\S+ \S+\s+\S+ (.*):.*`)
	for _, line := range contents {
		name := fileRE.FindStringSubmatch(line)
		if len(name) < 2 {
			t.Errorf("failed to find file in %s", line)
			continue
		}
		if got, want := name[1], "log_test.go"; got != want {
			t.Errorf("unexpected file name: got %s, want %s\n%v", got, want, contents)
			continue
		}
	}
	if want, got := 3, len(contents); want != got {
		t.Errorf("Expected %d info lines, got %d instead", want, got)
	}
}

func TestDepth(t *testing.T) {
	dir, err := ioutil.TempDir("", "logtest")
	defer os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	logger := vlog.NewLogger("testHeader")
	logger.Configure(vlog.LogDir(dir), vlog.Level(2))

	logger.InfoDepth(0, "here\n")
	tmp := func() {
		logger.InfoDepth(1, "still here\n")
		logger.InfoDepth(2, "not here\n")
	}
	tmp()
	logger.FlushLog()
	contents, err := readLogFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	fileRE := regexp.MustCompile(`\S+ \S+\s+\S+ (.*):.*`)
	files := []string{}
	for _, line := range contents {
		name := fileRE.FindStringSubmatch(line)
		if len(name) < 2 {
			t.Errorf("failed to find file in %s", line)
			continue
		}
		files = append(files, name[1])
	}
	for i, want := range []string{"log_test.go", "log_test.go", "testing.go"} {
		if got := files[i]; got != want {
			t.Errorf("%d: got %v, want %v", i, got, want)
		}
	}
}

func TestVModule(t *testing.T) {
	dir, err := ioutil.TempDir("", "logtest")
	defer os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logger := vlog.NewLogger("testVmodule")
	logger.Configure(vlog.LogDir(dir))
	if logger.V(2) || logger.V(3) {
		t.Errorf("Logging should not be enabled at levels 2 & 3")
	}
	spec := vlog.ModuleSpec{}
	if err := spec.Set("*log_test=2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := logger.Configure(vlog.OverridePriorConfiguration(true), spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !logger.V(2) {
		t.Errorf("logger.V(2) should be true")
	}
	if logger.V(3) {
		t.Errorf("logger.V(3) should be false")
	}
	if vlog.V(2) || vlog.V(3) {
		t.Errorf("Logging should not be enabled at levels 2 & 3")
	}
	vlog.Log.Configure(vlog.OverridePriorConfiguration(true), spec)
	if !vlog.V(2) {
		t.Errorf("vlog.V(2) should be true")
	}
	if vlog.V(3) {
		t.Errorf("vlog.V(3) should be false")
	}
	if vlog.VI(2) != vlog.Log {
		t.Errorf("vlog.V(2) should be vlog.Log")
	}
	if vlog.VI(3) == vlog.Log {
		t.Errorf("vlog.V(3) should not be vlog.Log")
	}
	spec = vlog.ModuleSpec{}
	spec.Set("notlikelytobeinuse=2")
	vlog.Log.Configure(vlog.OverridePriorConfiguration(true), spec)
}

func TestVFilepath(t *testing.T) {
	dir, err := ioutil.TempDir("", "logtest")
	defer os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logger := vlog.NewLogger("testVmodule")
	logger.Configure(vlog.LogDir(dir))
	if logger.V(2) || logger.V(3) {
		t.Errorf("Logging should not be enabled at levels 2 & 3")
	}
	spec := vlog.FilepathSpec{}
	if err := spec.Set(".*log_test=2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := logger.Configure(vlog.OverridePriorConfiguration(true), spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !logger.V(2) {
		t.Errorf("logger.V(2) should be true")
	}
	if logger.V(3) {
		t.Errorf("logger.V(3) should be false")
	}
	if vlog.V(2) || vlog.V(3) {
		t.Errorf("Logging should not be enabled at levels 2 & 3")
	}
	vlog.Log.Configure(vlog.OverridePriorConfiguration(true), spec)
	if !vlog.V(2) {
		t.Errorf("vlog.V(2) should be true")
	}
	if vlog.V(3) {
		t.Errorf("vlog.V(3) should be false")
	}
	if vlog.VI(2) != vlog.Log {
		t.Errorf("vlog.V(2) should be vlog.Log")
	}
	if vlog.VI(3) == vlog.Log {
		t.Errorf("vlog.V(3) should not be vlog.Log")
	}
	spec = vlog.FilepathSpec{}
	spec.Set("notlikelytobeinuse=2")
	vlog.Log.Configure(vlog.OverridePriorConfiguration(true), spec)
}

func TestConfigure(t *testing.T) {
	dir, err := ioutil.TempDir("", "logtest")
	defer os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logger := vlog.NewLogger("testVmodule")
	if got, want := logger.Configure(vlog.LogDir(dir), vlog.AlsoLogToStderr(false)), error(nil); got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := logger.Configure(vlog.AlsoLogToStderr(true)), vlog.ErrConfigured; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := logger.Configure(vlog.OverridePriorConfiguration(true), vlog.AlsoLogToStderr(false)), error(nil); got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStats(t *testing.T) {
	dir, err := ioutil.TempDir("", "logtest")
	defer os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	logger := vlog.NewLogger("testStats")
	logger.Configure(vlog.LogDir(dir))
	logger.Info("line 1")
	logger.Info("line 2")
	logger.Error("error 1")

	infoStats, errorStats := logger.Stats()
	expected := []struct{ Lines, Bytes int64 }{
		{2, 12},
		{1, 7}}
	for i, stats := range []struct {
		Lines, Bytes int64
	}{infoStats, errorStats} {
		if got, want := stats.Lines, expected[i].Lines; got != want {
			t.Errorf("%d: got %v, want %v", i, got, want)
		}
		// Need to have written out at least as many bytes as the actual message.
		if got, want := stats.Bytes, expected[i].Bytes; got <= want {
			t.Errorf("%d: got %v, but not > %v", i, got, want)
		}
	}

}
