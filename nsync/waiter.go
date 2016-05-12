// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nsync

import "math"
import "sync/atomic"
import "time"

// --------------------------------

// A dll represents a doubly-linked list of waiters.
type dll struct {
	next *dll
	prev *dll
	elem *waiter // points to the waiter struct this dll struct is embedded in, or nil if none.
}

// MakeEmpty() makes list *l empty.
// Requires that *l is currently not part of a non-empty list.
func (l *dll) MakeEmpty() {
	l.next = l
	l.prev = l
}

// IsEmpty() returns whether list *l is empty.
// Requires that *l is currently part of a list, or the zero dll element.
func (l *dll) IsEmpty() bool {
	return l.next == l
}

// InsertAfter() inserts element *e into the list after position *p.
// Requires that *e is currently not part of a list and that *p is part of a list.
func (e *dll) InsertAfter(p *dll) {
	e.next = p.next
	e.prev = p
	e.next.prev = e
	e.prev.next = e
}

// Remove() removes *e from the list it is currently in.
// Requires that *e is currently part of a list.
func (e *dll) Remove() {
	e.next.prev = e.prev
	e.prev.next = e.next
}

// IsInList() returns whether element e can be found in list l.
func (e *dll) IsInList(l *dll) bool {
	p := l.next
	for p != e && p != l {
		p = p.next
	}
	return p == e
}

// --------------------------------

// A waiter represents a single waiter on a CV or a Mu.
//
// To wait:
// Allocate a waiter struct *w with newWaiter(), set w.waiting=1, and
// w.cvMu=nil or to the associated Mu if waiting on a condition variable, then
// queue w.dll on some queue, and then wait using:
//    for atomic.LoadUint32(&w.waiting) != 0 { w.sem.P() }
// Return *w to the freepool by calling freeWaiter(w).
//
// To wakeup:
// Remove *w from the relevant queue then:
//  atomic.Store(&w.waiting, 0)
//  w.sem.V()
type waiter struct {
	q             dll             // Doubly-linked list element.
	sem           binarySemaphore // Thread waits on this semaphore.
	deadlineTimer *time.Timer     // Used for waits with deadlines.

	// If this waiter is waiting on a CV associated with a Mu, mu is a
	// pointer to that Mu, otherwise nil
	cvMu *Mu

	// non-zero <=> the waiter is waiting (read and written atomically)
	waiting uint32
}

var freeWaiters dll      // freeWaiters is a doubly-linked list of free waiter structs.
var freeWaitersMu uint32 // spinlock protects freeWaiters

// newWaiter() returns a pointer to an unused waiter struct.
// Ensures that the enclosed timer is stopped and its channel drained.
func newWaiter() (w *waiter) {
	spinTestAndSet(&freeWaitersMu, 1, 1)
	if freeWaiters.next == nil { // first time through, initialize the free list.
		freeWaiters.MakeEmpty()
	}
	if !freeWaiters.IsEmpty() { // If free list is non-empty, dequeue an item.
		var q *dll = freeWaiters.next
		q.Remove()
		w = q.elem
	}
	atomic.StoreUint32(&freeWaitersMu, 0) // release store
	if w == nil {                         // If free list was empty, allocate an item.
		w = new(waiter)
		w.sem.Init()
		w.deadlineTimer = time.NewTimer(time.Duration(math.MaxInt64))
		w.deadlineTimer.Stop()
		w.q.elem = w
	}
	return w
}

// freeWaiter() returns an unused waiter struct *w to the free pool.
func freeWaiter(w *waiter) {
	spinTestAndSet(&freeWaitersMu, 1, 1)
	w.q.InsertAfter(&freeWaiters)
	atomic.StoreUint32(&freeWaitersMu, 0) // release store
}
