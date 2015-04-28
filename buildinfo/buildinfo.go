// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package buildinfo implements a mechanism for injecting build-time
// metadata into executable binaries.
package buildinfo

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"time"
)

var (
	manifest  string
	platform  string
	pristine  string
	timestamp string
	username  string
)

// T describes binary metadata.
type T struct {
	// BuildPlatform records the target platform of the build.
	BuildPlatform string
	// BuildTimestamp records the time of the build.
	BuildTimestamp time.Time
	// BuildUser records the name of user who executed the build.
	BuildUser string
	// GoVersion records the Go version used for the build.
	GoVersion string
	// Manifest records the project manifest that identifies the state
	// of Vanadium projects used for the build.
	Manifest string
	// Pristine records whether the build was executed using pristine
	// master branches of Vanadium projects (or not).
	Pristine bool
}

var info T

func init() {
	info.BuildPlatform = platform
	if timestamp != "" {
		var err error
		info.BuildTimestamp, err = time.Parse(time.RFC3339, timestamp)
		if err != nil {
			panic(fmt.Sprintf("Parse(%v) failed: %v", timestamp, err))
		}
	}
	info.BuildUser = username
	info.GoVersion = runtime.Version()
	if manifest != "" {
		decodedBytes, err := base64.StdEncoding.DecodeString(manifest)
		if err != nil {
			panic(fmt.Sprintf("DecodeString() failed: %v", err))
		}
		info.Manifest = string(decodedBytes)
	}
	if pristine != "" {
		b, err := strconv.ParseBool(pristine)
		if err != nil {
			panic(fmt.Sprintf("ParseBool(%v) failed: %v", pristine, err))
		}
		info.Pristine = b
	}
}

// Info returns metadata about the current binary.
func Info() *T {
	return &info
}

// String returns the binary metadata as a JSON-encoded string, under the
// expectation that clients may want to parse it for specific bits of metadata.
func (t *T) String() string {
	jsonT, err := json.Marshal(t)
	if err != nil {
		return ""
	}
	return string(jsonT)
}
