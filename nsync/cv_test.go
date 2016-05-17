// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nsync_test

import "testing"
import "time"

import "v.io/x/lib/nsync"

// ---------------------------

// A queue represents a FIFO queue with up to Limit elements.
// The storage for the queue expands as necessary up to Limit.
type queue struct {
	Limit    int           // max value of count---should not be changed after initialization
	nonEmpty nsync.CV      // signalled when count transitions from zero to non-zero
	nonFull  nsync.CV      // signalled when count transitions from Limit to less than Limit
	mu       nsync.Mu      // protects fields below
	data     []interface{} // in use elements are data[pos, ..., (pos+count-1)%len(data)]
	pos      int           // index of first in-use element
	count    int           // number of elements in use
}

// Put() adds v to the end of the FIFO *q and returns true, or if the FIFO already
// has Limit elements and continues to do so until absDeadline, do nothing and
// return false.
func (q *queue) Put(v interface{}, absDeadline time.Time) (added bool) {
	q.mu.Lock()
	for q.count == q.Limit && q.nonFull.WaitWithDeadline(&q.mu, absDeadline, nil) == nsync.OK {
	}
	if q.count != q.Limit {
		length := len(q.data)
		i := q.pos + q.count
		if q.count == length {
			newLength := length * 2
			if newLength == 0 {
				newLength = 16
			}
			if q.Limit < newLength {
				newLength = q.Limit
			}
			newData := make([]interface{}, newLength)
			if i <= length {
				copy(newData[:], q.data[q.pos:i])
			} else {
				n := copy(newData[:], q.data[q.pos:length])
				copy(newData[n:], q.data[:i-length])
			}
			q.pos = 0
			i = q.count
			q.data = newData
			length = newLength
		}
		if length <= i {
			i -= length
		}
		q.data[i] = v
		if q.count == 0 {
			q.nonEmpty.Broadcast()
		}
		q.count++
		added = true
	}
	q.mu.Unlock()
	return added
}

// Get() removes the first value from the front of the FIFO *q and returns it
// and true, or if the FIFO is empty and continues to be so until absDeadline,
// do nothing and return nil and false.
func (q *queue) Get(absDeadline time.Time) (v interface{}, ok bool) {
	q.mu.Lock()
	for q.count == 0 && q.nonEmpty.WaitWithDeadline(&q.mu, absDeadline, nil) == nsync.OK {
	}
	if q.count != 0 {
		v = q.data[q.pos]
		q.data[q.pos] = nil
		if q.count == q.Limit {
			q.nonFull.Broadcast()
		}
		q.pos++
		q.count--
		if q.pos == len(q.data) {
			q.pos = 0
		}
		ok = true
	}
	q.mu.Unlock()
	return v, ok
}

// ---------------------------

// producerN() Put()s count integers on *q, in the sequence start*3, (start+1)*3, (start+2)*3, ....
func producerN(t *testing.T, q *queue, start int, count int) {
	for i := 0; i != count; i++ {
		if !q.Put((start+i)*3, nsync.NoDeadline) {
			t.Fatalf("queue.Put() returned false with no deadline")
		}
	}
}

// consumerN() Get()s count integers from *q, and checks that they are in the
// sequence start*3, (start+1)*3, (start+2)*3, ....
func consumerN(t *testing.T, q *queue, start int, count int) {
	for i := 0; i != count; i++ {
		v, ok := q.Get(nsync.NoDeadline)
		if !ok {
			t.Fatalf("queue.Get() returned false with no deadline")
		}
		x, isInt := v.(int)
		if !isInt {
			t.Fatalf("queue.Get() returned non integer value; wanted int %d, got %#v", (start+i)*3, v)
		}
		if x != (start+i)*3 {
			t.Fatalf("queue.Get() returned bad value; want %d, got %d", (start+i)*3, x)
		}
	}
}

// producerConsumerN is the number of elements passed from producer to consumer in the
// TestCVProducerConsumerX() tests below.
const producerConsumerN = 300000

// TestCVProducerConsumer0() sends a stream of integers from a producer thread to
// a consumer thread via a queue with Limit 10**0.
func TestCVProducerConsumer0(t *testing.T) {
	q := queue{Limit: 1}
	go producerN(t, &q, 0, producerConsumerN)
	consumerN(t, &q, 0, producerConsumerN)
}

// TestCVProducerConsumer1() sends a stream of integers from a producer thread to
// a consumer thread via a queue with Limit 10**1.
func TestCVProducerConsumer1(t *testing.T) {
	q := queue{Limit: 10}
	go producerN(t, &q, 0, producerConsumerN)
	consumerN(t, &q, 0, producerConsumerN)
}

// TestCVProducerConsumer2() sends a stream of integers from a producer thread to
// a consumer thread via a queue with Limit 10**2.
func TestCVProducerConsumer2(t *testing.T) {
	q := queue{Limit: 100}
	go producerN(t, &q, 0, producerConsumerN)
	consumerN(t, &q, 0, producerConsumerN)
}

// TestCVProducerConsumer3() sends a stream of integers from a producer thread to
// a consumer thread via a queue with Limit 10**3.
func TestCVProducerConsumer3(t *testing.T) {
	q := queue{Limit: 1000}
	go producerN(t, &q, 0, producerConsumerN)
	consumerN(t, &q, 0, producerConsumerN)
}

// TestCVProducerConsumer4() sends a stream of integers from a producer thread to
// a consumer thread via a queue with Limit 10**4.
func TestCVProducerConsumer4(t *testing.T) {
	q := queue{Limit: 10000}
	go producerN(t, &q, 0, producerConsumerN)
	consumerN(t, &q, 0, producerConsumerN)
}

// TestCVProducerConsumer5() sends a stream of integers from a producer thread to
// a consumer thread via a queue with Limit 10**5.
func TestCVProducerConsumer5(t *testing.T) {
	q := queue{Limit: 100000}
	go producerN(t, &q, 0, producerConsumerN)
	consumerN(t, &q, 0, producerConsumerN)
}

// TestCVProducerConsumer6() sends a stream of integers from a producer thread to
// a consumer thread via a queue with Limit 10**6.
func TestCVProducerConsumer6(t *testing.T) {
	q := queue{Limit: 1000000}
	go producerN(t, &q, 0, producerConsumerN)
	consumerN(t, &q, 0, producerConsumerN)
}

// TestCVDeadline() checks timeouts on a CV WaitWithDeadline().
func TestCVDeadline(t *testing.T) {
	var mu nsync.Mu
	var cv nsync.CV

	// The following two values control how aggressively we police the timeout.
	const tooEarly time.Duration = 1 * time.Millisecond
	const tooLate time.Duration = 35 * time.Millisecond // longer, to accommodate scheduling delays
	const tooLateAllowed int = 3                        // number of iterations permitted to violate tooLate

	var tooLateViolations int
	mu.Lock()
	for i := 0; i != 50; i++ {
		startTime := time.Now()
		expectedEndTime := startTime.Add(87 * time.Millisecond)
		if cv.WaitWithDeadline(&mu, expectedEndTime, nil) != nsync.Expired {
			t.Fatalf("cv.Wait() returns non-Expired for a timeout")
		}
		endTime := time.Now()
		if endTime.Before(expectedEndTime.Add(-tooEarly)) {
			t.Errorf("cvWait() returned %v too early", expectedEndTime.Sub(endTime))
		}
		if endTime.After(expectedEndTime.Add(tooLate)) {
			tooLateViolations++
		}
	}
	mu.Unlock()
	if tooLateViolations > tooLateAllowed {
		t.Errorf("cvWait() returned too late %d times", tooLateViolations)
	}
}

// TestCVCancel() checks cancellations on a CV WaitWithDeadline().
func TestCVCancel(t *testing.T) {
	var mu nsync.Mu
	var cv nsync.CV

	// The loops below cancel after 87 milliseconds, like the timeout tests above.

	// The following two values control how aggressively we police the timeout.
	const tooEarly time.Duration = 1 * time.Millisecond
	const tooLate time.Duration = 35 * time.Millisecond // longer, to accommodate scheduling delays
	const tooLateAllowed int = 3                        // number of iterations permitted to violate tooLate

	var futureTime time.Time = time.Now().Add(1 * time.Hour) // a future time, to test cancels with pending timeout

	var tooLateViolations int
	mu.Lock()
	for i := 0; i != 50; i++ {
		startTime := time.Now()
		expectedEndTime := startTime.Add(87 * time.Millisecond)

		cancel := make(chan struct{})
		time.AfterFunc(87*time.Millisecond, func() { close(cancel) })

		if cv.WaitWithDeadline(&mu, futureTime, cancel) != nsync.Cancelled {
			t.Fatalf("cv.Wait() return non-Cancelled for a cancellation")
		}
		endTime := time.Now()
		if endTime.Before(expectedEndTime.Add(-tooEarly)) {
			t.Errorf("cvWait() returned %v too early", expectedEndTime.Sub(endTime))
		}
		if endTime.After(expectedEndTime.Add(tooLate)) {
			tooLateViolations++
		}

		// Check that an already cancelled wait returns immediately.
		startTime = time.Now()
		if cv.WaitWithDeadline(&mu, nsync.NoDeadline, cancel) != nsync.Cancelled {
			t.Fatalf("cv.Wait() returns non-Cancelled for a cancellation")
		}
		endTime = time.Now()
		if endTime.Before(startTime) {
			t.Errorf("cvWait() returned %v too early", endTime.Sub(startTime))
		}
		if endTime.After(startTime.Add(tooLate)) {
			tooLateViolations++
		}
	}
	mu.Unlock()
	if tooLateViolations > tooLateAllowed {
		t.Errorf("cvWait() returned too late %d times", tooLateViolations)
	}
}
