// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Example use of Mu.Wait():  A priority queue of strings whose
// RemoveWithDeadline() operation has a deadline.

package nsync_test

import "container/heap"
import "fmt"
import "time"

import "v.io/x/lib/nsync"

// ---------------------------------------

// A priQueue implements heap.Interface and holds strings.
type priQueue []string

func (pq priQueue) Len() int               { return len(pq) }
func (pq priQueue) Less(i int, j int) bool { return pq[i] < pq[j] }
func (pq priQueue) Swap(i int, j int)      { pq[i], pq[j] = pq[j], pq[i] }
func (pq *priQueue) Push(x interface{})    { *pq = append(*pq, x.(string)) }
func (pq *priQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	s := old[n-1]
	*pq = old[0 : n-1]
	return s
}

// ---------------------------------------

// A StringPriorityQueue is a priority queue of strings, which emits the
// lexicographically least string available.
type StringPriorityQueue struct {
	nonEmpty nsync.CV // signalled when heap becomes non-empty
	mu       nsync.Mu // protects priQueue
	heap     priQueue
}

// Add() adds "s" to the queue *q.
func (q *StringPriorityQueue) Add(s string) {
	q.mu.Lock()
	if q.heap.Len() == 0 {
		q.nonEmpty.Broadcast()
	}
	heap.Push(&q.heap, s)
	q.mu.Unlock()
}

// RemoveWithDeadline() waits until queue *q is non-empty, then removes a string from its
// beginning, and returns it with true; or if absDeadline is reached before the
// queue becomes non-empty, returns the empty string and false.
func (q *StringPriorityQueue) RemoveWithDeadline(absDeadline time.Time) (s string, ok bool) {
	q.mu.Lock()
	for q.heap.Len() == 0 && q.nonEmpty.WaitWithDeadline(&q.mu, absDeadline, nil) == nsync.OK {
	}
	if q.heap.Len() != 0 {
		s = heap.Pop(&q.heap).(string)
		ok = true
	}
	q.mu.Unlock()
	return s, ok
}

// ---------------------------------------

// addAndWait() adds strings s[0, ...] to *q, with the specified delay between additions.
func addAndWait(q *StringPriorityQueue, delay time.Duration, s ...string) {
	for i := range s {
		q.Add(s[i])
		time.Sleep(delay)
	}
}

// removeAndPrint() removes the first item from *q and outputs it on stdout,
// or outputs "timeout: <delay>" if no value can be found before "delay" elapses.
func removeAndPrint(q *StringPriorityQueue, delay time.Duration) {
	if s, ok := q.RemoveWithDeadline(time.Now().Add(delay)); ok {
		fmt.Printf("%s\n", s)
	} else {
		fmt.Printf("timeout %v\n", delay)
	}
}

// ExampleMuWait() demonstrates the use of nsync.Mu's Wait() via a priority queue of strings.
// See the routine RemoveWithDeadline(), above.
func ExampleCVWait() {
	var q StringPriorityQueue

	go addAndWait(&q, 500*time.Millisecond, "one", "two", "three", "four", "five")

	time.Sleep(1100 * time.Millisecond) // delay while "one", "two" and "three" are queued, but not yet "four"

	removeAndPrint(&q, 1*time.Second)        // should get "one"
	removeAndPrint(&q, 1*time.Second)        // should get "three" (it's lexicographically less than "two")
	removeAndPrint(&q, 1*time.Second)        // should get "two"
	removeAndPrint(&q, 100*time.Millisecond) // should time out because 1.1 < 0.5*3
	removeAndPrint(&q, 1*time.Second)        // should get "four"
	removeAndPrint(&q, 100*time.Millisecond) // should time out because 0.1 < 0.5
	removeAndPrint(&q, 1*time.Second)        // should get "five"
	removeAndPrint(&q, 1*time.Second)        // should time out because there's no more to fetch
	// Output:
	// one
	// three
	// two
	// timeout 100ms
	// four
	// timeout 100ms
	// five
	// timeout 1s
}
