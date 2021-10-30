// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package flagvar

import (
	"os"
	"strings"
)

// ExpandEnv is like os.ExpandEnv but supports 'pseudo' environment
// variables that have OS specific handling as follows:
//
// $USERHOME and $HOME are replaced by $HOMEDRIVE:\\$HOMEPATH on Windows.
// All /'s are replaced with \'s.
func ExpandEnv(e string) string {
	e = strings.ReplaceAll(e, "$HOME", `$HOMEDRIVE$HOMEPATH`)
	e = strings.ReplaceAll(e, "$USERHOME", `$HOMEDRIVE$HOMEPATH`)
	return strings.ReplaceAll(os.ExpandEnv(e), `/`, `\`)
}
