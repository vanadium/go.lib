// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nsync

import "math"
import "runtime"
import "sync/atomic"
import "time"

// NoDeadline represents a time in the far future---a deadline that will not expire.
var NoDeadline time.Time

// init() initializes the variable NoDeadline.
// If done inline, the godoc output is even more ugly.
func init() {
	NoDeadline = time.Now().Add(time.Duration(math.MaxInt64)).Add(time.Duration(math.MaxInt64))
}

// spinDelay() is used in spinloops to delay resumption of the loop.
// Usage:
//     var attempts uint
//     for try_something {
//        attempts = spinDelay(attempts)
//     }
func spinDelay(attempts uint) uint {
	if attempts < 7 {
		for i := 0; i != 1<<attempts; i++ {
		}
		attempts++
	} else {
		runtime.Gosched()
	}
	return attempts
}

// spinTestAndSet() spins until (*w & test) == 0.  It then atomically performs
// *w |= set and returns the previous value of *w.  It performs an acquire
// barrier.
func spinTestAndSet(w *uint32, test uint32, set uint32) uint32 {
	var attempts uint // cvSpinlock retry count
	var old uint32 = atomic.LoadUint32(w)
	for (old&test) != 0 || !atomic.CompareAndSwapUint32(w, old, old|set) { // acquire CAS
		attempts = spinDelay(attempts)
		old = atomic.LoadUint32(w)
	}
	return old
}
