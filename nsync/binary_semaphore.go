// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nsync

import "time"

// A binarySemaphore is a binary semaphore; it can have values 0 and 1.
type binarySemaphore struct {
	ch chan struct{}
}

// Init() initializes binarySemaphore *s; the initial value is 0.
func (s *binarySemaphore) Init() {
	s.ch = make(chan struct{}, 1)
}

// P() waits until the count of semaphore *s is 1 and decrements the
// count to 0.
func (s *binarySemaphore) P() {
	<-s.ch
}

// PWithDeadline() waits until one of:
// the count of semaphore *s is 1, in which case the semaphore is decremented to 0, then OK is returned;
// or deadlineTimer!=nil and *deadlineTimer expires, then Expired is returned;
// or cancelChan != nil and cancelChan becomes readable or closed, then Cancelled is returned.
// The channel "v.io/v23/context".T.Done() is a suitable cancelChan.
func (s *binarySemaphore) PWithDeadline(deadlineTimer *time.Timer, cancelChan <-chan struct{}) (res int) {
	var deadlineChan <-chan time.Time
	if deadlineTimer != nil {
		deadlineChan = deadlineTimer.C
	}
	// Avoid select if possible---it's slow.
	if deadlineTimer != nil || cancelChan != nil {
		select {
		case <-s.ch:
			res = OK
		case <-deadlineChan:
			res = Expired
		case <-cancelChan:
			res = Cancelled
		}
	} else {
		<-s.ch
		res = OK
	}
	return res
}

// V() ensures that the semaphore count of *s is 1.
func (s *binarySemaphore) V() {
	select {
	case s.ch <- struct{}{}:
	default: // Don't block if the semaphore count is already 1.
	}
}
