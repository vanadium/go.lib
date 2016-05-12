// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nsync_test

import "runtime"
import "sync"
import "testing"

import "v.io/x/lib/nsync"

// A testData is the state shared between the threads in each of the tests below.
type testData struct {
	nThreads  int // Number of test threads; constant after init.
	loopCount int // Iteration count for each test thread; constant after init.

	mu nsync.Mu // Protects i, id, and finishedThreads.
	i  int      // Counter incremented by test loops.
	id int      // id of current lock-holding thread in some tests.

	mutex sync.Mutex // Protects i and id when in countingLoopMutex.

	done            nsync.CV // Signalled when finishedThread==nThreads.
	finishedThreads int      // Count of threads that have finished.
}

// threadFinished() indicates that a thread has finished its operations on testData
// by incrementing td.finishedThreads, and signalling td.done when it reaches td.nThreads.
// See waitForAllThreads().
// We could use sync.WaitGroup here, but this code exercises nsync more.
func (td *testData) threadFinished() {
	td.mu.Lock()
	td.finishedThreads++
	if td.finishedThreads == td.nThreads {
		td.done.Broadcast()
	}
	td.mu.Unlock()
}

// waitForAllThreads() waits until all td.nThreads have called threadFinished(),
// and then returns.
// We could use sync.WaitGroup here, but this code exercises nsync more.
func (td *testData) waitForAllThreads() {
	td.mu.Lock()
	for td.finishedThreads != td.nThreads {
		td.done.Wait(&td.mu)
	}
	td.mu.Unlock()
}

// ---------------------------------------

// countingLoopMu() is the body of each thread executed by TestMuNThread().
// *td represents the test data that the threads share, and id is an integer
// unique to each test thread.
func countingLoopMu(td *testData, id int) {
	var n int = td.loopCount
	for i := 0; i != n; i++ {
		td.mu.Lock()
		td.id = id
		td.i++
		if td.id != id {
			panic("td.id != id")
		}
		td.mu.Unlock()
	}
	td.threadFinished()
}

// TestMuNThread creates a few threads, each of which increment an
// integer a fixed number of times, using an nsync.Mu for mutual exclusion.
// It checks that the integer is incremented the correct number of times.
func TestMuNThread(t *testing.T) {
	td := testData{nThreads: 5, loopCount: 1000000}
	for i := 0; i != td.nThreads; i++ {
		go countingLoopMu(&td, i)
	}
	td.waitForAllThreads()
	if td.i != td.nThreads*td.loopCount {
		t.Fatalf("TestMuNThread final count inconsistent: want %d, got %d",
			td.nThreads*td.loopCount, td.i)
	}
}

// ---------------------------------------

// countingLoopMutex() is the body of each thread executed by TestMutexNThread().
// *td represents the test data that the threads share, and id is an integer
// unique to each test thread, here protected by a sync.Mutex.
func countingLoopMutex(td *testData, id int) {
	var n int = td.loopCount
	for i := 0; i != n; i++ {
		td.mutex.Lock()
		td.id = id
		td.i++
		if td.id != id {
			panic("td.id != id")
		}
		td.mutex.Unlock()
	}
	td.threadFinished()
}

// TestMutexNThread creates a few threads, each of which increment an
// integer a fixed number of times, using a sync.Mutex for mutual exclusion.
// It checks that the integer is incremented the correct number of times.
func TestMutexNThread(t *testing.T) {
	td := testData{nThreads: 5, loopCount: 1000000}
	for i := 0; i != td.nThreads; i++ {
		go countingLoopMutex(&td, i)
	}
	td.waitForAllThreads()
	if td.i != td.nThreads*td.loopCount {
		t.Fatalf("TestMutexNThread final count inconsistent: want %d, got %d",
			td.nThreads*td.loopCount, td.i)
	}
}

// ---------------------------------------

// countingLoopTryMu() is the body of each thread executed by TestTryMuNThread().
// *td represents the test data that the threads share, and id is an integer
// unique to each test thread.
func countingLoopTryMu(td *testData, id int) {
	var n int = td.loopCount
	for i := 0; i != n; i++ {
		for !td.mu.TryLock() {
			runtime.Gosched()
		}
		td.id = id
		td.i++
		if td.id != id {
			panic("td.id != id")
		}
		td.mu.Unlock()
	}
	td.threadFinished()
}

// TestTryMuNThread() tests that acquiring an nsync.Mu with TryLock()
// using several threads provides mutual exclusion.
func TestTryMuNThread(t *testing.T) {
	td := testData{nThreads: 5, loopCount: 100000}
	for i := 0; i != td.nThreads; i++ {
		go countingLoopTryMu(&td, i)
	}
	td.waitForAllThreads()
	if td.i != td.nThreads*td.loopCount {
		t.Fatalf("TestTryMuNThread final count inconsistent: want %d, got %d",
			td.nThreads*td.loopCount, td.i)
	}
}

// ---------------------------------------

// BenchmarkMuUncontended() measures the performance of an uncontended nsync.Mu.
func BenchmarkMuUncontended(b *testing.B) {
	var mu nsync.Mu
	for i := 0; i != b.N; i++ {
		mu.Lock()
		mu.Unlock()
	}
}

// BenchmarkMutexUncontended() measures the performance of an uncontended sync.Mutex.
func BenchmarkMutexUncontended(b *testing.B) {
	var mu sync.Mutex
	for i := 0; i != b.N; i++ {
		mu.Lock()
		mu.Unlock()
	}
}
