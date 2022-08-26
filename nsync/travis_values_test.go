// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build travis
// +build travis

package nsync_test

// Tune this value for running travis to reduce flakiness
const expectedTimeoutsDelta = 300
