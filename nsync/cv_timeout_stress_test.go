// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This test runs too slowly under the race detector.
// +build !race

package nsync_test

import "fmt"
import "math/rand"
import "testing"
import "time"

import "v.io/x/lib/nsync"

// ---------------------------

// A cvStressData represents the data used by the threads of TestCVWaitStress.
type cvStressData struct {
	mu       nsync.Mu // protects fields below
	count    uint64   // incremented by the various threads
	timeouts uint64   // incremented on each timeout

	refs uint // reference count: one per test thread, decremented when it exits

	countIsIMod4 [4]nsync.CV // element i signalled when count==i mod 4
	refsIsZero   nsync.CV    // signalled when refs==0
}

// The delay in cvStressIncLoop() is uniformly distributed from 0 to
// cvMaxDelayMicros-1 microseconds.
const cvMaxDelayMicros = 1000                                // maximum delay
const cvMeanDelayMicros = cvMaxDelayMicros / 2               // mean delay
const cvExpectedTimeoutsPerSec = 1000000 / cvMeanDelayMicros // number of timeouts expected per second

// cvStressIncLoop() acquires s.mu, then increments s.count n times, each time
// waiting until condition is true.  A random delay between 0us and 999us is
// used for each wait; if the timeout expires, s.timeouts is incremented, and
// the wait is retried.  s.refs is decremented before the routine returns.
func cvStressIncLoop(s *cvStressData, countImod4 uint64, n uint64) {
	s.mu.Lock()
	s.mu.AssertHeld()
	for i := uint64(0); i != n; i++ {
		s.mu.AssertHeld()
		for (s.count & 3) != countImod4 {
			var absDeadline time.Time = time.Now().Add(time.Duration(rand.Int31n(cvMaxDelayMicros)) * time.Microsecond)
			for s.countIsIMod4[countImod4].WaitWithDeadline(&s.mu, absDeadline, nil) != nsync.OK && (s.count&3) != countImod4 {
				s.mu.AssertHeld()
				s.timeouts++
				s.mu.AssertHeld()
				absDeadline = time.Now().Add(time.Duration(rand.Int31n(cvMaxDelayMicros)) * time.Microsecond)
			}
		}
		s.mu.AssertHeld()
		s.count++
		s.countIsIMod4[s.count&3].Signal()
	}
	s.refs--
	if s.refs == 0 {
		s.refsIsZero.Signal()
	}
	s.mu.AssertHeld()
	s.mu.Unlock()
}

// TestCVTimeoutStress() tests many threads using a single lock, using
// WaitConditions and timeouts.
//
// It creates a cvStressData s, and then creates several threads using
// cvStressIncLoop() trying to increment s.count from 1 to 2 mod 4, from 2 to 3
// mod 4, and from 3 to 0 mod 4, using random delays.  It sleeps a few seconds, ensuring
// many random timeouts by threads in cvStressIncLoop, because there is no thread
// incrementing s.count from 0 (which is 0 mod 4).  It then creates several
// threads using cvStressIncLoop() trying to increment s.count from 0 to 1 mod 4.
// This allows all the threads to run to completion, since there are equal
// numbers for each condition.
// Finally, it waits for all threads to exit.
func TestCVTimeoutStress(t *testing.T) {
	const loopCount = 50000
	const threadsPerValue = 5
	var s cvStressData

	s.mu.Lock()
	s.mu.AssertHeld()
	// Create threads trying to increment from 1, 2, and 3 mod 4.
	// They will continually hit their timeouts because s.count==0
	for i := 0; i != threadsPerValue; i++ {
		s.mu.AssertHeld()
		s.refs++
		go cvStressIncLoop(&s, 1, loopCount)
		s.refs++
		go cvStressIncLoop(&s, 2, loopCount)
		s.refs++
		go cvStressIncLoop(&s, 3, loopCount)
	}
	s.mu.AssertHeld()
	s.mu.Unlock()

	// Sleep a few seconds to cause many timeouts.
	const sleepSeconds = 3
	time.Sleep(sleepSeconds * time.Second)

	s.mu.Lock()
	s.mu.AssertHeld()

	// Check that approximately the right number of timeouts have occurred.
	// The 3 below is the three classes of thread produced before the Sleep().
	// The factor of 1/4 is to allow for randomness and slow test machines.
	expectedTimeouts := uint64(threadsPerValue * 3 * sleepSeconds * cvExpectedTimeoutsPerSec / 4)
	timeoutsSeen := s.timeouts
	if timeoutsSeen < expectedTimeouts {
		t.Errorf("expected more than %d timeouts, got %d", expectedTimeouts, timeoutsSeen)
	}

	// Now create the threads that increment from 0 mod 4.   s.count will then be incremented.
	for i := 0; i != threadsPerValue; i++ {
		s.mu.AssertHeld()
		s.refs++
		go cvStressIncLoop(&s, 0, loopCount)
	}

	// Wait for threads to exit.
	s.mu.AssertHeld()
	for s.refs != 0 {
		s.refsIsZero.Wait(&s.mu)
	}
	s.mu.AssertHeld()
	if s.refs != 0 {
		t.Fatalf(fmt.Sprintf("s.refs == %d; expected 0 at end of TestCVWaitStress", s.refs))
	}

	s.mu.AssertHeld()
	s.mu.Unlock()

	// Check that s.count has the right value.
	expectedCount := uint64(loopCount * threadsPerValue * 4)
	if s.count != expectedCount {
		t.Errorf("expected to increment s.count to %d, got %d", expectedCount, s.count)
	}

	// Some timeouts shoud have happened while the counts were being incremented.
	expectedTimeouts = timeoutsSeen + 1000
	if s.timeouts < expectedTimeouts {
		t.Errorf("expected more than %d timeouts, got %d", expectedTimeouts, s.timeouts)
	}
}
