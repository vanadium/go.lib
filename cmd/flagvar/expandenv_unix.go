// Copyright 2021 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build !windows
// +build !windows

package flagvar

import (
	"os"
)

// ExpandEnv is like os.ExpandEnv but supports 'pseudo' environment
// variables that have OS specific handling as follows:
//
// On Windows $HOME and $PATH are replaced by and $HOMEDRIVE:\\$HOMEPATH
// and $Path respectively.
// On Windows /'s are replaced with \'s.
func ExpandEnv(e string) string {
	return os.ExpandEnv(e)
}
