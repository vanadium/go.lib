// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows
// +build !windows

package flagvar

import (
	"os"
	"strings"
)

// ExpandEnv is like os.ExpandEnv but supports 'pseudo' environment
// variables that have OS specific handling as follows:
//
// $USERHOME is replaced by $HOME.
func ExpandEnv(e string) string {
	e = strings.ReplaceAll(e, "$USERHOME", "$HOME")
	return os.ExpandEnv(e)
}
