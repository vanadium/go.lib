// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package timing implements utilities for tracking timing information.
package timing

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// nowFunc is used rather than direct calls to time.Now to allow tests to inject
// different clock functions.
var nowFunc = time.Now

// Interval represents a named time interval and its children.  The children are
// non-overlapping and ordered from earliest to latest.  The start and end time
// of a given interval always completely cover all of its children.  Put another
// way, intervals form a strictly hierarchical tree.
type Interval interface {
	// Name returns the name of the interval.
	Name() string

	// Start returns the start time of the interval.
	Start() time.Time

	// End returns the end time of the interval, or zero if the interval hasn't
	// ended yet (i.e. it's still open).
	End() time.Time

	// NumChild returns the number of children contained in this interval.
	NumChild() int

	// Child returns the child interval at the given index.  Valid children are in
	// the range [0, NumChild).
	Child(index int) Interval

	// String returns a formatted string describing the tree starting with the
	// given interval.
	String() string
}

// Timer provides support for tracking a tree of hierarchical time intervals.
// If you need to track overlapping time intervals, simply use separate Timers.
//
// Timer maintains a notion of a current interval, initialized to the root.  The
// tree of intervals is constructed by push and pop operations, which add and
// update intervals to the tree, while updating the currently referenced
// interval.  Finish should be called to finish all timing.
type Timer interface {
	// Push appends a child with the given name and an open interval to current,
	// and updates the current interval to refer to the newly created child.
	Push(name string)

	// Pop closes the current interval, and updates the current interval to refer
	// to its parent.  Pop does nothing if the current interval is the root.
	Pop()

	// Finish finishes all timing, closing all intervals including the root.
	Finish()

	// Root returns the root interval.
	Root() Interval

	// String returns a formatted string describing the tree of time intervals.
	String() string
}

// IntervalDuration returns the elapsed time between i.start and i.end if i.end
// is nonzero, otherwise it returns the elapsed time between i.start and now.
// Now is passed in explicitly to allow the caller to use the same now time when
// computing the duration of many intervals.  E.g. the same now time is used by
// the IntervalPrinter to compute the duration of all intervals, to ensure
// consistent output.
func IntervalDuration(i Interval, now time.Time) time.Duration {
	start, end := i.Start(), i.End()
	if end.IsZero() {
		return now.Sub(start)
	}
	return end.Sub(start)
}

// IntervalPrinter is a pretty-printer for Intervals.  Example output:
//
//    00:00:01.000 root       98.000s       00:01:39.000
//    00:00:01.000    *           9.000s    00:00:10.000
//    00:00:10.000    foo        45.000s    00:00:55.000
//    00:00:10.000       *           5.000s 00:00:15.000
//    00:00:15.000       foo1       22.000s 00:00:37.000
//    00:00:37.000       foo2       18.000s 00:00:55.000
//    00:00:55.000    bar        25.000s    00:01:20.000
//    00:01:20.000    baz        19.000s    00:01:39.000
type IntervalPrinter struct {
	// TimeFormat is passed to time.Format to format the start and end times.
	// Defaults to "15:04:05.000" if the value is empty.
	TimeFormat string
	// Indent is the number of spaces to indent each successive depth in the tree.
	// Defaults to 3 spaces if the value is 0; set to a negative value for no
	// indent.
	Indent int
	// MinGap is the minimum duration for gaps to be shown between successive
	// entries; only gaps that are larger than this threshold will be shown.
	// Defaults to 1 millisecond if the value is 0; set to a negative duration to
	// show all gaps.
	MinGap time.Duration
}

// Print writes formatted output to w representing the tree rooted at i.
func (p IntervalPrinter) Print(w io.Writer, i Interval) error {
	// Set default options for zero fields.
	if p.TimeFormat == "" {
		p.TimeFormat = "15:04:05.000"
	}
	switch {
	case p.Indent < 0:
		p.Indent = 0
	case p.Indent == 0:
		p.Indent = 3
	}
	switch {
	case p.MinGap < 0:
		p.MinGap = 0
	case p.MinGap == 0:
		p.MinGap = time.Millisecond
	}
	return p.print(w, i, p.collectPrintStats(i), i.Start(), 0)
}

func (p IntervalPrinter) print(w io.Writer, i Interval, stats *printStats, prevEnd time.Time, depth int) error {
	// Print gap before children, if a gap exists.
	if gap := i.Start().Sub(prevEnd); gap >= p.MinGap {
		if err := p.row(w, "*", prevEnd, i.Start(), gap, stats, depth); err != nil {
			return err
		}
	}
	// Print the current interval.
	if err := p.row(w, i.Name(), i.Start(), i.End(), IntervalDuration(i, stats.Now), stats, depth); err != nil {
		return err
	}
	// Print children recursively.
	for child := 0; child < i.NumChild(); child++ {
		prevEnd = i.Start()
		if child > 0 {
			prevEnd = i.Child(child - 1).End()
		}
		if err := p.print(w, i.Child(child), stats, prevEnd, depth+1); err != nil {
			return err
		}
	}
	// Print gap after children, if a gap exists.
	if last := i.NumChild() - 1; last >= 0 {
		lastChild := i.Child(last)
		if gap := i.End().Sub(lastChild.End()); gap >= p.MinGap {
			if err := p.row(w, "*", lastChild.End(), i.End(), gap, stats, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p IntervalPrinter) row(w io.Writer, name string, start, end time.Time, dur time.Duration, stats *printStats, depth int) error {
	pad := strings.Repeat(" ", p.Indent*depth)
	pad2 := strings.Repeat(" ", p.Indent*(stats.MaxDepth-depth))
	endStr := stats.NowLabel
	if !end.IsZero() {
		endStr = end.Format(p.TimeFormat)
	}
	_, err := fmt.Fprintf(w, "%s %-*s %s%*.3fs%s %s\n", start.Format(p.TimeFormat), stats.NameWidth, pad+name, pad, stats.DurationWidth, float64(dur)/float64(time.Second), pad2, endStr)
	return err
}

// collectPrintStats performs a walk through the tree rooted at i, collecting
// statistics along the way, to help align columns in the output.
func (p IntervalPrinter) collectPrintStats(i Interval) *printStats {
	stats := &printStats{
		Now:       nowFunc(),
		NameWidth: 1,
		NowLabel:  strings.Repeat("-", len(p.TimeFormat)-3) + "now",
	}
	stats.collect(i, p.Indent, i.Start(), 0)
	dur := fmt.Sprintf("%.3f", float64(stats.MaxDuration)/float64(time.Second))
	stats.DurationWidth = len(dur)
	return stats
}

type printStats struct {
	Now           time.Time
	NowLabel      string
	NameWidth     int
	MaxDuration   time.Duration
	DurationWidth int
	MaxDepth      int
}

func (s *printStats) collect(i Interval, indent int, prevEnd time.Time, depth int) {
	if x := len(i.Name()) + indent*depth; x > s.NameWidth {
		s.NameWidth = x
	}
	if x := i.Start().Sub(prevEnd); x > s.MaxDuration {
		s.MaxDuration = x
	}
	if x := IntervalDuration(i, s.Now); x > s.MaxDuration {
		s.MaxDuration = x
	}
	if x := depth; x > s.MaxDepth {
		s.MaxDepth = x
	}
	for child := 0; child < i.NumChild(); child++ {
		prevEnd = i.Start()
		if child > 0 {
			prevEnd = i.Child(child - 1).End()
		}
		s.collect(i.Child(child), indent, prevEnd, depth+1)
	}
}
