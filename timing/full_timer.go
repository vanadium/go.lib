// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package timing

import (
	"bytes"
	"time"
)

// FullInterval implements Interval with a direct data mapping; each
// FullInterval represents a node in the FullTimer tree.
type FullInterval struct {
	Label              string
	StartTime, EndTime time.Time
	Children           []FullInterval
}

// Name returns the name of the interval.
func (i FullInterval) Name() string { return i.Label }

// Start returns the start time of the interval.
func (i FullInterval) Start() time.Time { return i.StartTime }

// End returns the end time of the interval, or zero if the interval hasn't
// ended yet (i.e. it's still open).
func (i FullInterval) End() time.Time { return i.EndTime }

// NumChild returns the number of children contained in this interval.
func (i FullInterval) NumChild() int { return len(i.Children) }

// Child returns the child interval at the given index.  Valid children are in
// the range [0, NumChild).
func (i FullInterval) Child(index int) Interval { return i.Children[index] }

// String returns a formatted string describing the tree starting with the
// given interval.
func (i FullInterval) String() string {
	var buf bytes.Buffer
	IntervalPrinter{}.Print(&buf, i)
	return buf.String()
}

// FullTimer implements Timer, using FullInterval to represent the tree of
// intervals.  Timestamps are recorded on each call to Push, Pop and Finish.
//
// FullTimer performs more memory allocations and is slower than CompactTimer
// for push and pop, but collects actual start and end times for all intervals.
type FullTimer struct {
	// FullRoot holds the root of the time interval tree.
	FullRoot FullInterval

	// The stack holds the path through the interval tree leading to the current
	// interval.  This makes it easy to determine the current interval, as well as
	// pop up to the parent interval.  The root is never held in the stack.
	//
	// There is a subtlely here, since we're keeping a stack of pointers; we must
	// be careful not to invalidate any pointers.  E.g. given an Interval X with a
	// child Y, we will invalidate the pointer to Y when we append to X.Children,
	// if a new underlying array is created.  This never occurs since the stack
	// never contains a pointer to Y when appending to X.Children.
	stack []*FullInterval
}

// NewFullTimer returns a new FullTimer, with the root interval initialized to
// the given name.
func NewFullTimer(name string) *FullTimer {
	return &FullTimer{
		FullRoot: FullInterval{Label: name, StartTime: nowFunc()},
	}
}

// Push implements the Timer.Push method.
func (t *FullTimer) Push(name string) {
	var current *FullInterval
	if len(t.stack) == 0 {
		// Unset the root end time, to handle Push after Finish.
		t.FullRoot.EndTime = time.Time{}
		current = &t.FullRoot
	} else {
		current = t.stack[len(t.stack)-1]
	}
	push := FullInterval{Label: name, StartTime: nowFunc()}
	current.Children = append(current.Children, push)
	t.stack = append(t.stack, &current.Children[len(current.Children)-1])
}

// Pop implements the Timer.Pop method.
func (t *FullTimer) Pop() {
	if len(t.stack) == 0 {
		return // Pop on the root does nothing.
	}
	last := len(t.stack) - 1
	t.stack[last].EndTime = nowFunc()
	t.stack = t.stack[:last]
}

// Finish implements the Timer.Finish method.
func (t *FullTimer) Finish() {
	end := nowFunc()
	t.FullRoot.EndTime = end
	for _, interval := range t.stack {
		interval.EndTime = end
	}
	t.stack = t.stack[:0]
}

// Root returns the root interval.
func (t *FullTimer) Root() Interval {
	return t.FullRoot
}

// String returns a formatted string describing the tree of time intervals.
func (t *FullTimer) String() string {
	return t.FullRoot.String()
}
