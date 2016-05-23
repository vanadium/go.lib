// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The nsync package provides a mutex Mu and a Mesa-style condition variable CV.
//
// The nsync primitives differ from those in sync in that nsync provides
// timed/cancellable wait on CV, and try-lock on Mu; CV's wait primitives take
// the mutex as an explicit argument to remind the reader that they have a side
// effect on the mutex; the zero value CV can be used without further
// initialization; and Mu forbids a lock acquired by one thread to be released
// by another.
//
// As well as Mu and CV being usable with one another, an nsync,Mu can be used
// with a sync.Cond, and an nsync.CV can be used with a sync.Mutex.
package nsync

import "sync/atomic"

// Implementation notes
//
// The implementation of Mu and CV both use spinlocks to protect their waiter
// queues.  The spinlocks are implemented with atomic operations and a delay
// loop found in common.go.  They could use sync.Mutex, but I wished to have an
// implementation independent of the sync package (except for the trivial use
// of the sync.Locker interface.
//
// Mu and CV use the same type of doubly-linked list of waiters (see
// waiter.go).  This allows waiters to be transferred from the CV queue to the
// Mu queue when a thread is logically woken from the CV but would immediately
// go to sleep on the Mu.  See the wakeWaiters() call in cv.go.
//
// In Mu, the "designated waker" is a thread that was waiting on Mu, has been
// woken up, but as yet has neither acquired nor gone back to waiting.  The
// presence of such a thread is indicated by the muDesigWaker bit in the Mu
// word.  This bit allows the Unlock() code to avoid waking a second waiter
// when there's already one will wake the next thread when the time comes.
// This speeds things up when the lock is heavily contended, and the critical
// sections are small.
//
// The weasel words "with high probability" in the specification of TryLock()
// prevent clients from believing that they can determine with certainty
// whether another thread has given up a lock yet.  This, together with the
// requirement that a thread that acquired a mutex must release it (rather than
// it being released by another thread), prohibits clients from using Mu as a
// sort of semaphore.  The intent is that it be used only for traditional
// mutual exclusion.  This leaves room for certain future optimizations, and
// make it easier to apply detection of potential races via candidate lock-set
// algorithms, should that ever be desired.
//
// The condition variable has a wait with an absolute rather than a relative
// timeout.  This is less error prone, as described in the comment on
// CV.WaitWithDeadline().  Alas, relative timeouts are seductive in trivial
// examples (such as tests).  These are the first things that people try, so
// they are likely to be requested.  If enough people complain we could give
// them that particular piece of rope.
//
// The condition variable uses an internal binary semaphore that is passed a
// *time.Timer stored in a *waiter for its expirations.  It is as invariant
// that the timer is inactive and its channel drained when the waiter is on the
// waiter free list.

// A Mu is a mutex.  Its zero value is valid, and unlocked.
// It is similar to sync.Mutex, but implements TryLock().
//
// A Mu can be "free", or held by a single thread (aka goroutine).  A thread
// that acquires it should eventually release it.  It is not legal to acquire a
// Mu in one thread and release it in another.
//
// Example usage, where p.mu is an nsync.Mu protecting the invariant p.a+p.b==0
//      p.mu.Lock()
//      // The current thread now has exclusive access to p.a and p.b; invariant assumed true.
//      p.a++
//      p.b-- // restore invariant p.a+p.b==0 before releasing p.mu
//      p.mu.Unlock()
type Mu struct {
	word    uint32 // bits:  see below
	waiters dll    // Head of a doubly-linked list of enqueued waiters; under mu.
}

// Bits in Mu.word
const (
	muLock       = 1 << iota // lock is held.
	muSpinlock   = 1 << iota // spinlock is held (protects waiters).
	muWaiting    = 1 << iota // waiter list is non-empty.
	muDesigWaker = 1 << iota // a former waiter has been woken, and has not yet acquired or slept once more.
)

// TryLock() attempts to acquire *mu without blocking, and returns whether it is successful.
// It returns true with high probability if *mu was free on entry.
func (mu *Mu) TryLock() bool {
	if atomic.CompareAndSwapUint32(&mu.word, 0, muLock) { // acquire CAS
		return true
	}
	oldWord := atomic.LoadUint32(&mu.word)
	return (oldWord&muLock) == 0 && atomic.CompareAndSwapUint32(&mu.word, oldWord, oldWord|muLock) // acquire CAS
}

// Lock() blocks until *mu is free and then acquires it.
func (mu *Mu) Lock() {
	if !atomic.CompareAndSwapUint32(&mu.word, 0, muLock) { // acquire CAS
		oldWord := atomic.LoadUint32(&mu.word)
		if (oldWord&muLock) != 0 || !atomic.CompareAndSwapUint32(&mu.word, oldWord, oldWord|muLock) { // acquire CAS
			mu.lockSlow(newWaiter(), 0)
		}
	}
}

// lockSlow() locks *mu, waiting on *w if it needs to wait.
// "clear" should be zero if the thread has not previously slept on *mu, and
// muDesigWaker if it has; this represents bits that lockSlow() must clear when
// it either acquires or sleeps on *mu.
func (mu *Mu) lockSlow(w *waiter, clear uint32) {
	var attempts uint // attempt count; used for spinloop backoff
	w.cvMu = nil      // not a CV wait
	for {
		oldWord := atomic.LoadUint32(&mu.word)
		if (oldWord & muLock) == 0 {
			// lock is not held; try to acquire, possibly clearing muDesigWaker
			if atomic.CompareAndSwapUint32(&mu.word, oldWord, (oldWord|muLock)&^clear) { // acquire CAS
				freeWaiter(w)
				return
			}
		} else if (oldWord&muSpinlock) == 0 &&
			atomic.CompareAndSwapUint32(&mu.word, oldWord, (oldWord|muSpinlock|muWaiting)&^clear) { // acquire CAS

			// Spinlock is now held, and lock is held by someone
			// else; muWaiting has also been set; queue ourselves.
			atomic.StoreUint32(&w.waiting, 1)
			if (oldWord & muWaiting) == 0 { // init the waiter list if it's empty.
				mu.waiters.MakeEmpty()
			}
			w.q.InsertAfter(&mu.waiters)

			// Release spinlock.  Cannot use a store here, because
			// the current thread does not hold the mutex.  If
			// another thread were a designated waker, the mutex
			// holder could be concurrently unlocking, even though
			// we hold the spinlock.
			oldWord = atomic.LoadUint32(&mu.word)
			for !atomic.CompareAndSwapUint32(&mu.word, oldWord, oldWord&^muSpinlock) { // release CAS
				oldWord = atomic.LoadUint32(&mu.word)
			}

			// Wait until awoken.
			for atomic.LoadUint32(&w.waiting) != 0 { // acquire load
				w.sem.P()
			}

			attempts = 0
			clear = muDesigWaker
		}
		attempts = spinDelay(attempts)
	}
}

// Unlock() unlocks *mu, and wakes a waiter if there is one.
func (mu *Mu) Unlock() {
	// Go is a garbage-collected language, so it's legal (and slightly
	// faster on x86) to release with an atomic add before checking for
	// waiters.  Without GC, this would not work because another thread
	// could acquire, decrement a reference count and deallocate the mutex
	// before the current thread touched the mutex word again to wake
	// waiters.
	newWord := atomic.AddUint32(&mu.word, ^uint32(muLock-1))
	if (newWord&(muLock|muWaiting)) == 0 || (newWord&(muLock|muDesigWaker)) == muDesigWaker {
		return
	}

	if (newWord & muLock) != 0 {
		panic("attempt to Unlock a free nsync.Mu")
	}

	var attempts uint // attempt count; used for backoff
	for {
		oldWord := atomic.LoadUint32(&mu.word)
		if (oldWord&muWaiting) == 0 || (oldWord&muDesigWaker) == muDesigWaker {
			return // no one to wake, or there's already a designated waker waking up.
		} else if (oldWord&muSpinlock) == 0 &&
			atomic.CompareAndSwapUint32(&mu.word, oldWord, oldWord|muSpinlock|muDesigWaker) { // acquire CAS
			// The spinlock is now held, and we've set the designated wake flag, since
			// we're likely to wake a thread that will become that designated waker.

			if mu.waiters.elem != nil {
				panic("non-nil mu.waiters.dll.elem")
			}

			// Remove a waiter from the queue, if possible.
			var wake *waiter = mu.waiters.prev.elem
			var clearOnRelease uint32 = muSpinlock
			if wake != nil {
				wake.q.Remove()
			} else {
				clearOnRelease |= muDesigWaker // not waking a waiter => no designated waker
			}
			if mu.waiters.IsEmpty() {
				clearOnRelease |= muWaiting // no waiters left
			}
			// Unset the spinlock bit; set the designated wake
			// bit---the thread we're waking is the new designated
			// waker.  Cannot use a store here because we no longer
			// hold the main lock.
			oldWord = atomic.LoadUint32(&mu.word)
			for !atomic.CompareAndSwapUint32(&mu.word, oldWord, (oldWord|muDesigWaker)&^clearOnRelease) { // release CAS
				oldWord = atomic.LoadUint32(&mu.word)
			}
			if wake != nil { // Wake the waiter
				atomic.StoreUint32(&wake.waiting, 0) // release store
				wake.sem.V()
			}
			return
		}
		attempts = spinDelay(attempts)
	}
}

// AssertHeld() panics if *mu is not held.
func (mu *Mu) AssertHeld() {
	if (atomic.LoadUint32(&mu.word) & muLock) == 0 {
		panic("nsync.Mu not held")
	}
}
