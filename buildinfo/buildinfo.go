// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package buildinfo defines a mechanism to inject build-time metadata into
// binaries.
package buildinfo

import (
	"encoding/json"
	"runtime"
)

// These variables are filled in at link time, using:
//  -ldflags "-X v.io/x/lib/buildinfo.<varname> <value>"
var timestamp, username, platform string

// T describes binary metadata.
type T struct {
	GoVersion, BuildTimestamp, BuildUser, BuildPlatform string
}

var info T

func init() {
	info = T{
		GoVersion:      runtime.Version(),
		BuildTimestamp: timestamp,
		BuildUser:      username,
		BuildPlatform:  platform,
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
