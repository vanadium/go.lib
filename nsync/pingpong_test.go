// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nsync_test

import "sync"
import "testing"
import "time"

import "v.io/x/lib/nsync"

// The benchmarks in this file use various mechanisms to
// ping-pong back and forth between two threads as they count i from
// 0 to limit.
// The data structure contains multiple synchronization primitives,
// but each benchmark uses only those it needs.
//
// The setting of GOMAXPROCS, and the exact choices of the thread scheduler can
// have great effect on the timings.
type pingPong struct {
	mu nsync.Mu
	cv [2]nsync.CV

	mutex sync.Mutex
	cond  [2]*sync.Cond

	i     int
	limit int
}

// ---------------------------------------

// mutexCVPingPong() is run by each thread in BenchmarkPingPongMutexCV().
func (pp *pingPong) mutexCVPingPong(parity int) {
	pp.mutex.Lock()
	for pp.i < pp.limit {
		for (pp.i & 1) == parity {
			pp.cv[parity].Wait(&pp.mutex)
		}
		pp.i++
		pp.cv[1-parity].Signal()
	}
	pp.mutex.Unlock()
}

// BenchmarkPingPongMutexCV() measures the wakeup speed of
// sync.Mutex/nsync.CV used to ping-pong back and forth between two threads.
func BenchmarkPingPongMutexCV(b *testing.B) {
	pp := pingPong{limit: b.N}
	go pp.mutexCVPingPong(0)
	pp.mutexCVPingPong(1)
}

// ---------------------------------------

// muCVPingPong() is run by each thread in BenchmarkPingPongMuCV().
func (pp *pingPong) muCVPingPong(parity int) {
	pp.mu.Lock()
	for pp.i < pp.limit {
		for (pp.i & 1) == parity {
			pp.cv[parity].Wait(&pp.mu)
		}
		pp.i++
		pp.cv[1-parity].Signal()
	}
	pp.mu.Unlock()
}

// BenchmarkPingPongMuCV() measures the wakeup speed of nsync.Mu/nsync.CV used to
// ping-pong back and forth between two threads.
func BenchmarkPingPongMuCV(b *testing.B) {
	pp := pingPong{limit: b.N}
	go pp.muCVPingPong(0)
	pp.muCVPingPong(1)
}

// ---------------------------------------

// muCVUnexpiredDeadlinePingPong() is run by each thread in BenchmarkPingPongMuCVUnexpiredDeadline().
func (pp *pingPong) muCVUnexpiredDeadlinePingPong(parity int) {
	var deadlineIn1Hour time.Time = time.Now().Add(1 * time.Hour)
	pp.mu.Lock()
	for pp.i < pp.limit {
		for (pp.i & 1) == parity {
			pp.cv[parity].WaitWithDeadline(&pp.mu, deadlineIn1Hour, nil)
		}
		pp.i++
		pp.cv[1-parity].Signal()
	}
	pp.mu.Unlock()
}

// BenchmarkPingPongMuCVUnexpiredDeadline() measures the wakeup speed of nsync.Mu/nsync.CV used to
// ping-pong back and forth between two threads.
func BenchmarkPingPongMuCVUnexpiredDeadline(b *testing.B) {
	pp := pingPong{limit: b.N}
	go pp.muCVUnexpiredDeadlinePingPong(0)
	pp.muCVUnexpiredDeadlinePingPong(1)
}

// ---------------------------------------

// mutexCondPingPong() is run by each thread in BenchmarkPingPongMutexCond().
func (pp *pingPong) mutexCondPingPong(parity int) {
	pp.mutex.Lock()
	for pp.i < pp.limit {
		for (pp.i & 1) == parity {
			pp.cond[parity].Wait()
		}
		pp.i++
		pp.cond[1-parity].Signal()
	}
	pp.mutex.Unlock()
}

// BenchmarkPingPongMutexCond() measures the wakeup speed of
// sync.Mutex/sync.Cond used to ping-pong back and forth between two threads.
func BenchmarkPingPongMutexCond(b *testing.B) {
	pp := pingPong{limit: b.N}
	pp.cond[0] = sync.NewCond(&pp.mutex)
	pp.cond[1] = sync.NewCond(&pp.mutex)
	go pp.mutexCondPingPong(0)
	pp.mutexCondPingPong(1)
}

// ---------------------------------------

// muCondPingPong() is run by each thread in BenchmarkPingPongMuCond().
func (pp *pingPong) muCondPingPong(parity int) {
	pp.mu.Lock()
	for pp.i < pp.limit {
		for (pp.i & 1) == parity {
			pp.cond[parity].Wait()
		}
		pp.i++
		pp.cond[1-parity].Signal()
	}
	pp.mu.Unlock()
}

// BenchmarkPingPongMuCond() measures the wakeup speed of nsync.Mutex/sync.Cond
// used to ping-pong back and forth between two threads.
func BenchmarkPingPongMuCond(b *testing.B) {
	pp := pingPong{limit: b.N}
	pp.cond[0] = sync.NewCond(&pp.mu)
	pp.cond[1] = sync.NewCond(&pp.mu)
	go pp.muCondPingPong(0)
	pp.muCondPingPong(1)
}
