// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nsync

import "sync"
import "sync/atomic"
import "time"

// See also the implementation notes at the top of mu.go.

// A CV is a condition variable in the style of Mesa, Java, POSIX, and Go's sync.Cond.
// It allows a thread to wait for a condition on state protected by a mutex,
// and to proceed with the mutex held and the condition true.
//
// When compared with sync.Cond:  (a) CV adds WaitWithDeadline() which allows
// timeouts and cancellation, (b) the mutex is an explicit argument of the wait
// calls to remind the reader that they have a side-effect on the mutex, and
// (c) (as a result of (b)), a zero-valued CV is a valid CV with no enqueued
// waiters, so there is no need of a call to construct a CV.
//
// Usage:
//
// After making the desired predicate true, call:
//     cv.Signal() // If at most one thread can make use of the predicate becoming true.
// or
//     cv.Broadcast() // If multiple threads can make use of the predicate becoming true.
//
// To wait for a predicate with no deadline (assuming cv.Broadcast() is called
// whenever the predicate becomes true):
//      mu.Lock()
//      for !some_predicate_protected_by_mu { // the for-loop is required.
//              cv.Wait(&mu)
//      }
//      // predicate is now true
//      mu.Unlock()
//
// To wait for a predicate with a deadline (assuming cv.Broadcast() is called
// whenever the predicate becomes true):
//      mu.Lock()
//      for !some_predicate_protected_by_mu && cv.WaitWithDeadline(&mu, absDeadline, cancelChan) == nsync.OK {
//      }
//      if some_predicate_protected_by_mu { // predicate is true
//      } else { // predicate is false, and deadline expired, or cancelChan was closed.
//      }
//      mu.Unlock()
// or, if the predicate is complex and you wish to write it just once and
// inline, you could use the following instead of the for-loop above:
//      mu.Lock()
//      var predIsTrue bool
//      for outcome := OK; ; outcome = cv.WaitWithDeadline(&mu, absDeadline, cancelChan) {
//              if predIsTrue = some_predicate_protected_by_mu; predIsTrue || outcome != nsync.OK {
//                      break
//              }
//      }
//      if predIsTrue { // predicate is true
//      } else { // predicate is false, and deadline expired, or cancelChan was closed.
//      }
//      mu.Unlock()
//
// As the examples show, Mesa-style condition variables require that waits use
// a loop that tests the predicate anew after each wait.  It may be surprising
// that these are preferred over the precise wakeups offered by the condition
// variables in Hoare monitors.  Imprecise wakeups make more efficient use of
// the critical section, because threads can enter it while a woken thread is
// still emerging from the scheduler, which may take thousands of cycles.
// Further, they make the programme easier to read and debug by making the
// predicate explicit locally at the wait, where the predicate is about to be
// assumed; the reader does not have to infer the predicate by examining all
// the places where wakeups may occur.
type CV struct {
	word    uint32 // see bits below; read and written atomically
	waiters dll    // Head of a doubly-linked list of enqueued waiters; under mu.
}

// Bits in CV.word
const (
	cvSpinlock = 1 << iota // protects waiters
	cvNonEmpty = 1 << iota // waiters list is non-empty
)

// Values returned by CV.WaitWithDeadline().
const (
	OK        = iota // Neither expired nor cancelled.
	Expired   = iota // absDeadline expired.
	Cancelled = iota // cancelChan was closed.
)

// WaitWithDeadline() atomically releases "mu" and blocks the calling thread on
// *cv.  It then waits until awakened by a call to Signal() or Broadcast() (or
// a spurious wakeup), or by the time reaching absDeadline, or by cancelChan
// being closed.  In all cases, it reacquires "mu", and returns the reason for
// the call returned (OK, Expired, or Cancelled).  Use
// absDeadline==nsync.NoDeadline for no deadline, and cancelChan==nil for no
// cancellation.  WaitWithDeadline() should be used in a loop, as with all
// Mesa-style condition variables.  See examples above.
//
// There are two reasons for using an absolute deadline, rather than a relative
// timeout---these are why pthread_cond_timedwait() also uses an absolute
// deadline.  First, condition variable waits have to be used in a loop; with
// an absolute times, the deadline does not have to be recomputed on each
// iteration.  Second, in most real programmes, some activity (such as an RPC
// to a server, or when guaranteeing response time in a UI), there is a
// deadline imposed by the specification or the caller/user; relative delays
// can shift arbitrarily with scheduling delays, and so after multiple waits
// might extend beyond the expected deadline.  Relative delays tend to be more
// convenient mostly in tests and trivial examples than they are in real
// programmes.
func (cv *CV) WaitWithDeadline(mu sync.Locker, absDeadline time.Time, cancelChan <-chan struct{}) (outcome int) {
	var w *waiter = newWaiter()
	atomic.StoreUint32(&w.waiting, 1)
	cvMu, _ := mu.(*Mu)
	w.cvMu = cvMu // If the Locker is an nsync.Mu, record its address, else record nil.

	oldWord := spinTestAndSet(&cv.word, cvSpinlock, cvSpinlock|cvNonEmpty) // acquire spinlock, set non-empty
	if (oldWord & cvNonEmpty) == 0 {
		cv.waiters.MakeEmpty() // initialize the waiter queue if it was empty.
	}
	w.q.InsertAfter(&cv.waiters)
	// Release the spin lock.
	atomic.StoreUint32(&cv.word, oldWord|cvNonEmpty) // release store

	mu.Unlock() // Release *mu.

	// Prepare a time.Timer for the deadline, if any.  We use a time.Timer
	// pre-allocated for the waiter, to avoid allocating and garbage
	// collecting one on each wait.
	var deadlineTimer *time.Timer
	if absDeadline != NoDeadline {
		deadlineTimer = w.deadlineTimer
		if deadlineTimer.Reset(absDeadline.Sub(time.Now())) {
			// w.deadlineTimer is guaranteed inactive and drained;
			// see "Stop any active timer" code below.
			panic("deadlineTimer was active")
		}
	}

	// Wait until awoken or a timeout.
	semOutcome := OK
	var attempts uint
	for atomic.LoadUint32(&w.waiting) != 0 { // acquire load
		if semOutcome == OK {
			semOutcome = w.sem.PWithDeadline(deadlineTimer, cancelChan)
		}
		if semOutcome != OK && atomic.LoadUint32(&w.waiting) != 0 { // acquire load
			// A timeout or cancellation occurred, and no wakeup.  Acquire the spinlock, and confirm.
			oldWord = spinTestAndSet(&cv.word, cvSpinlock, cvSpinlock)
			// Check that w wasn't removed from the queue after we
			// checked above, but before we acquired the spinlock.
			// The call to IsInList() confirms that the waiter *w is still governed
			// by *cv's spinlock; otherwise, some other thread is about to set w.waiting==0.
			if atomic.LoadUint32(&w.waiting) != 0 && w.q.IsInList(&cv.waiters) { // still in waiter queue
				// Not woken, so remove ourselves from queue, and declare a timeout or cancellation.
				outcome = semOutcome
				w.q.Remove()
				atomic.StoreUint32(&w.waiting, 0) // release store
				if cv.waiters.IsEmpty() {
					oldWord &^= cvNonEmpty
				}
			}
			// Release spinlock.
			atomic.StoreUint32(&cv.word, oldWord) // release store
			if atomic.LoadUint32(&w.waiting) != 0 {
				attempts = spinDelay(attempts) // so we will ultimately yield to scheduler.
			}
		}
	}

	// Stop any active timer, and drain its channel.
	if deadlineTimer != nil && semOutcome != Expired && !deadlineTimer.Stop() /*expired*/ {
		// This receive is synchonous because time.Timer's expire+send
		// is not atomic:  it may send after Stop() returns false!  The
		// "semOutcome != Expired" ensures that the value wasn't
		// consumed by the PWithDeadline() above.
		<-deadlineTimer.C
	}

	if cvMu != nil && w.cvMu == nil { // waiter was transferred to mu's queue, and woken.
		// Requeue mu using existing waiter struct; current thread is the designated waker.
		cvMu.lockSlow(w, muDesigWaker)
	} else {
		// Traditional case: We've woken from the CV, and need to reacquire mu.
		freeWaiter(w)
		mu.Lock()
	}
	return outcome
}

// Signal() wakes at least one thread currently enqueued on *cv.
func (cv *CV) Signal() {
	if (atomic.LoadUint32(&cv.word) & cvNonEmpty) != 0 { // acquire load
		var toWakeList *waiter                                      // waiters that we will wake
		oldWord := spinTestAndSet(&cv.word, cvSpinlock, cvSpinlock) // acquire spinlock
		if !cv.waiters.IsEmpty() {
			// Point to first waiter that enqueued itself, and detach it from all others.
			toWakeList = cv.waiters.prev.elem
			toWakeList.q.Remove()
			toWakeList.q.MakeEmpty()
			if cv.waiters.IsEmpty() {
				oldWord &^= cvNonEmpty
			}
		}
		// Release spinlock.
		atomic.StoreUint32(&cv.word, oldWord) // release store
		if toWakeList != nil {
			wakeWaiters(toWakeList)
		}
	}
}

// Broadcast() wakes all threads currently enqueued on *cv.
func (cv *CV) Broadcast() {
	if (atomic.LoadUint32(&cv.word) & cvNonEmpty) != 0 { // acquire load
		var toWakeList *waiter                           // waiters that we will wake
		spinTestAndSet(&cv.word, cvSpinlock, cvSpinlock) // acquire spinlock
		if !cv.waiters.IsEmpty() {
			// Point to last waiter that enqueued itself, still attached to all other waiters.
			toWakeList = cv.waiters.next.elem
			cv.waiters.Remove()
			cv.waiters.MakeEmpty()
		}
		// Release spinlock and mark queue empty.
		atomic.StoreUint32(&cv.word, 0) // release store
		if toWakeList != nil {
			wakeWaiters(toWakeList)
		}
	}
}

// Wait() atomically releases "mu" and blocks the caller on *cv.  It waits
// until it is awakened by a call to Signal() or Broadcast(), or a spurious
// wakeup.  It then reacquires "mu", and returns.  It is equivalent to a call
// to WaitWithDeadline() with absDeadline==NoDeadline, and a nil cancelChan.
// It should be used in a loop, as with all standard Mesa-style condition
// variables.  See examples above.
func (cv *CV) Wait(mu sync.Locker) {
	cv.WaitWithDeadline(mu, NoDeadline, nil)
}

// ------------------------------------------

// wakeWaiters() wakes the CV waiters in the circular list pointed to by toWakeList,
// which may not be nil.  If the waiter is associated with an nsync.Mu (as
// opposed to another implementation of sync.Locker), the "wakeup" may consist
// of transferring the waiters to the nsync.Mu's queue.  Requires:
// - Every element of the list pointed to by toWakeList is a waiter---there is
//   no head/sentinel.
// - Every waiter is associated with the same mutex.
func wakeWaiters(toWakeList *waiter) {
	var firstWaiter *waiter = toWakeList.q.prev.elem
	var mu *Mu = firstWaiter.cvMu
	if mu != nil { // waiter is associated with the nsync.Mu *mu.
		// We will transfer elements of toWakeList to *mu if all of:
		//  - mu's spinlock is not held, and
		//  - either mu is locked, or there's more than one thread on toWakeList, and
		//  - we acquire the spinlock on the first try.
		// The spinlock acquisition also marks mu as having waiters.
		var oldMuWord uint32 = atomic.LoadUint32(&mu.word)
		var locked bool = (oldMuWord & muLock) != 0
		var setDesigWaker uint32 // set to muDesigWaker if a thread is to be woken rather than transferred
		if !locked {
			setDesigWaker = muDesigWaker
		}
		if (oldMuWord&muSpinlock) == 0 &&
			(locked || firstWaiter != toWakeList) &&
			atomic.CompareAndSwapUint32(&mu.word, oldMuWord, (oldMuWord|muSpinlock|muWaiting|setDesigWaker)) { // acquire CAS

			// Choose which waiters to transfer, and which to wake.
			toTransferList := toWakeList
			if locked { // *mu is held; all the threads get transferred.
				toWakeList = nil
			} else { // *mu is not held; we transfer all but the first thread, which will be woken.
				toWakeList = firstWaiter
				toWakeList.q.Remove()
				toWakeList.q.MakeEmpty()
			}

			// Transfer the waiters on toTransferList to *mu's
			// waiter queue.  We've acquired *mu's spinlock.  Queue
			// the threads there instead of waking them.
			for toTransferList != nil {
				var toTransfer *waiter = toTransferList.q.prev.elem
				if toTransfer == toTransferList { // *toTransferList was singleton; *toTransfer is last waiter
					toTransferList = nil
				} else {
					toTransfer.q.Remove()
				}
				if toTransfer.cvMu != mu {
					panic("multiple mutexes used with condition variable")
				}
				toTransfer.cvMu = nil // tell WaitWithDeadline() that we moved the waiter to *mu's queue.
				// toTransfer.waiting is already 1, from being on CV's waiter queue.
				if (oldMuWord & muWaiting) == 0 { // if there were previously no waiters, initialize.
					mu.waiters.MakeEmpty()
					oldMuWord |= muWaiting // so next iteration won't initialize again.
				}
				toTransfer.q.InsertAfter(&mu.waiters)
			}

			// release *mu's spinlock  (muWaiting was set by CAS above)
			oldMuWord = atomic.LoadUint32(&mu.word)
			for !atomic.CompareAndSwapUint32(&mu.word, oldMuWord, oldMuWord&^muSpinlock) { // release CAS
				oldMuWord = atomic.LoadUint32(&mu.word)
			}
		} else if (oldMuWord & (muSpinlock | muLock | muDesigWaker)) == 0 {
			// If spinlock and lock are not held, try to set muDesigWaker because
			// at least one thread is to be woken.
			atomic.CompareAndSwapUint32(&mu.word, oldMuWord, oldMuWord|muDesigWaker)
		}
	}

	// Wake any waiters we didn't manage to enqueue on the Mu.
	for toWakeList != nil {
		// Take one waiter from the toWakeList.
		var toWake *waiter = toWakeList.q.prev.elem
		if toWake == toWakeList { // *toWakeList was a singleton; *toWake is the last waiter
			toWakeList = nil // tell the loop to exit
		} else {
			toWake.q.Remove() // get the waiter out of the list
		}

		// Wake the waiter.
		atomic.StoreUint32(&toWake.waiting, 0) // release store
		toWake.sem.V()
	}
}
