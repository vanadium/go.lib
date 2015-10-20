// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package timing

import (
	"bytes"
	"time"
)

// CompactTimer implements Timer with a memory-efficient implementation.
// Timestamps are only recorded on calls to Push and Finish; the assumption is
// that there is typically a very small delay between a call to Pop and a
// subsequent call to Push or Finish, so the Pop time doesn't need to be
// recorded.
//
// CompactTimer performs fewer memory allocations and is faster than FullTimer
// for push and pop, but doesn't collect actual end times.
type CompactTimer struct {
	points []compactPoint
	depth  int
	zero   time.Time
}

// compactPoint represents a single interval, taking advantage of the fact that
// all intervals are required to be disjoint, and also assuming that all
// intervals are adjacent to each other.  Thus instead of holding both a start
// and end time, it only holds a single NextStart time, represented as a delta
// from the zero time for further space savings.
//
// The NextStart time represents the time that the next interval starts.  If the
// next interval is at the same or smaller depth, this is also the end time for
// the current interval.
//
// The interesting logic is in Push; we rely on each Push call to update
// NextStart for the previous entry, and to create a new entry.  This also lets
// us keep a single slice of points in CompactTimer, which is more memory
// efficient than the tree of slices used in FullInterval.
//
// The trade-off we make for the memory efficiency and simple Push/Pop/Finish
// logic is that we need to do more work in compactInterval to piece-together
// the resulting time intervals.
type compactPoint struct {
	Label     string
	Depth     int
	NextStart time.Duration
}

const invalidNext = time.Duration(-1 << 63)

// NewCompactTimer returns a new CompactTimer, with the root interval
// initialized to the given name.
func NewCompactTimer(name string) *CompactTimer {
	return &CompactTimer{
		points: []compactPoint{{
			Label:     name,
			Depth:     0,
			NextStart: invalidNext,
		}},
		zero: nowFunc(),
	}
}

// Push implements the Timer.Push method.
func (t *CompactTimer) Push(name string) {
	t.depth++
	t.points[len(t.points)-1].NextStart = nowFunc().Sub(t.zero)
	t.points = append(t.points, compactPoint{
		Label:     name,
		Depth:     t.depth,
		NextStart: invalidNext,
	})
}

// Pop implements the Timer.Pop method.
func (t *CompactTimer) Pop() {
	if t.depth > 0 {
		t.depth--
	}
}

// Finish implements the Timer.Finish method.
func (t *CompactTimer) Finish() {
	t.depth = 0
	t.points[len(t.points)-1].NextStart = nowFunc().Sub(t.zero)
}

// String returns a formatted string describing the tree of time intervals.
func (t *CompactTimer) String() string {
	return t.Root().String()
}

// Root returns the root interval.
func (t *CompactTimer) Root() Interval {
	return compactInterval{
		points:   t.points,
		children: computeCompactChildren(t.points),
		zero:     t.zero,
		start:    t.zero,
	}
}

// compactInterval implements Interval using the underlying slice of points
// recorded by CompactTimer.  The interesting method is Child, where we
// re-compute the points to use for the next compactInterval.
type compactInterval struct {
	points      []compactPoint
	children    []int
	zero, start time.Time
}

// computeCompactChildren returns the indices in points that are immediate
// children of the first point.  Points must be a subtree rooted at the first
// point; the depth of every point in points[1:] must be greater than the depth
// of the first point.
func computeCompactChildren(points []compactPoint) (children []int) {
	if len(points) < 2 {
		return
	}
	target := points[0].Depth + 1
	for index := 1; index < len(points); index++ {
		if point := points[index]; point.Depth == target {
			children = append(children, index)
		}
	}
	return
}

// Name returns the name of the interval.
func (i compactInterval) Name() string { return i.points[0].Label }

// Start returns the start time of the interval.
func (i compactInterval) Start() time.Time { return i.start }

// End returns the end time of the interval, or zero if the interval hasn't
// ended yet (i.e. it's still open).
func (i compactInterval) End() time.Time {
	if next := i.points[len(i.points)-1].NextStart; next != invalidNext {
		return i.zero.Add(next)
	}
	return time.Time{}
}

// NumChild returns the number of children contained in this interval.
func (i compactInterval) NumChild() int { return len(i.children) }

// Child returns the child interval at the given index.  Valid children are in
// the range [0, NumChild).
func (i compactInterval) Child(index int) Interval {
	beg := i.children[index]
	var end int
	if index+1 < len(i.children) {
		end = i.children[index+1]
	} else {
		end = len(i.points)
	}
	points := i.points[beg:end]
	return compactInterval{
		points:   points,
		children: computeCompactChildren(points),
		zero:     i.zero,
		start:    i.zero.Add(i.points[beg-1].NextStart),
	}
}

// String returns a formatted string describing the tree starting with the
// given interval.
func (i compactInterval) String() string {
	var buf bytes.Buffer
	IntervalPrinter{}.Print(&buf, i)
	return buf.String()
}
