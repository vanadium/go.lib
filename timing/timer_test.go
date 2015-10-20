// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package timing

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func sec(d int) time.Time {
	return time.Time{}.Add(time.Second * time.Duration(d))
}

// fakeNow is a simulated clock where now is set manually.
type fakeNow struct{ now int }

func (f *fakeNow) Now() time.Time { return sec(f.now) }

// stepNow is a simulated clock where now increments in 1 second steps.
type stepNow struct{ now int }

func (s *stepNow) Now() time.Time {
	s.now++
	return sec(s.now)
}

type (
	// op represents the operations that can be performed on a Timer, with a fake
	// clock.  This makes it easy to construct test cases.
	op interface {
		run(f *fakeNow, t Timer)
	}
	push struct {
		now  int
		name string
	}
	pop struct {
		now int
	}
	finish struct {
		now int
	}
)

func (x push) run(f *fakeNow, t Timer) {
	f.now = x.now
	t.Push(x.name)
}
func (x pop) run(f *fakeNow, t Timer) {
	f.now = x.now
	t.Pop()
}
func (x finish) run(f *fakeNow, t Timer) {
	f.now = x.now
	t.Finish()
}

type kind int

const (
	kindFull kind = iota
	kindCompact
)

func (k kind) NewTimer(name string) Timer {
	switch k {
	case kindFull:
		return NewFullTimer(name)
	default:
		return NewCompactTimer(name)
	}
}

func (k kind) String() string {
	switch k {
	case kindFull:
		return "FullTimer"
	default:
		return "CompactTimer"
	}
}

func TestTimer(t *testing.T) {
	tests := []struct {
		ops                 []op
		full, compact       *FullInterval
		fullStr, compactStr string
	}{
		{
			nil,
			&FullInterval{"root", sec(1), sec(0), nil}, nil,
			`
00:00:01.000 root 999.000s ---------now
`, ``,
		},
		{
			[]op{pop{123}},
			&FullInterval{"root", sec(1), sec(0), nil}, nil,
			`
00:00:01.000 root 999.000s ---------now
`, ``,
		},
		{
			[]op{finish{99}},
			&FullInterval{"root", sec(1), sec(99), nil}, nil,
			`
00:00:01.000 root 98.000s 00:01:39.000
`, ``,
		},
		{
			[]op{finish{99}, pop{123}},
			&FullInterval{"root", sec(1), sec(99), nil}, nil,
			`
00:00:01.000 root 98.000s 00:01:39.000
`, ``,
		},
		{
			[]op{push{10, "abc"}},
			&FullInterval{"root", sec(1), sec(0),
				[]FullInterval{{"abc", sec(10), sec(0), nil}}}, nil,
			`
00:00:01.000 root   999.000s    ---------now
00:00:01.000    *        9.000s 00:00:10.000
00:00:10.000    abc    990.000s ---------now
`, ``,
		},
		{
			[]op{push{10, "abc"}, finish{99}},
			&FullInterval{"root", sec(1), sec(99),
				[]FullInterval{{"abc", sec(10), sec(99), nil}}}, nil,
			`
00:00:01.000 root   98.000s    00:01:39.000
00:00:01.000    *       9.000s 00:00:10.000
00:00:10.000    abc    89.000s 00:01:39.000
`, ``,
		},
		{
			[]op{push{10, "abc"}, pop{20}, finish{99}},
			&FullInterval{"root", sec(1), sec(99),
				[]FullInterval{{"abc", sec(10), sec(20), nil}}},
			&FullInterval{"root", sec(1), sec(99),
				[]FullInterval{{"abc", sec(10), sec(99), nil}}},
			`
00:00:01.000 root   98.000s    00:01:39.000
00:00:01.000    *       9.000s 00:00:10.000
00:00:10.000    abc    10.000s 00:00:20.000
00:00:20.000    *      79.000s 00:01:39.000
`,
			`
00:00:01.000 root   98.000s    00:01:39.000
00:00:01.000    *       9.000s 00:00:10.000
00:00:10.000    abc    89.000s 00:01:39.000
`,
		},
		{
			[]op{push{10, "A1"}, push{20, "A1_1"}},
			&FullInterval{"root", sec(1), sec(0),
				[]FullInterval{{"A1", sec(10), sec(0),
					[]FullInterval{{"A1_1", sec(20), sec(0), nil}}}}}, nil,
			`
00:00:01.000 root       999.000s       ---------now
00:00:01.000    *            9.000s    00:00:10.000
00:00:10.000    A1         990.000s    ---------now
00:00:10.000       *           10.000s 00:00:20.000
00:00:20.000       A1_1       980.000s ---------now
`, ``,
		},
		{
			[]op{push{10, "A1"}, push{20, "A1_1"}, finish{99}},
			&FullInterval{"root", sec(1), sec(99),
				[]FullInterval{{"A1", sec(10), sec(99),
					[]FullInterval{{"A1_1", sec(20), sec(99), nil}}}}}, nil,
			`
00:00:01.000 root       98.000s       00:01:39.000
00:00:01.000    *           9.000s    00:00:10.000
00:00:10.000    A1         89.000s    00:01:39.000
00:00:10.000       *          10.000s 00:00:20.000
00:00:20.000       A1_1       79.000s 00:01:39.000
`, ``,
		},
		{
			[]op{push{10, "A1"}, push{20, "A1_1"}, pop{30}, finish{99}},
			&FullInterval{"root", sec(1), sec(99),
				[]FullInterval{{"A1", sec(10), sec(99),
					[]FullInterval{{"A1_1", sec(20), sec(30), nil}}}}},
			&FullInterval{"root", sec(1), sec(99),
				[]FullInterval{{"A1", sec(10), sec(99),
					[]FullInterval{{"A1_1", sec(20), sec(99), nil}}}}},
			`
00:00:01.000 root       98.000s       00:01:39.000
00:00:01.000    *           9.000s    00:00:10.000
00:00:10.000    A1         89.000s    00:01:39.000
00:00:10.000       *          10.000s 00:00:20.000
00:00:20.000       A1_1       10.000s 00:00:30.000
00:00:30.000       *          69.000s 00:01:39.000
`,
			`
00:00:01.000 root       98.000s       00:01:39.000
00:00:01.000    *           9.000s    00:00:10.000
00:00:10.000    A1         89.000s    00:01:39.000
00:00:10.000       *          10.000s 00:00:20.000
00:00:20.000       A1_1       79.000s 00:01:39.000
`,
		},
		{
			[]op{push{10, "A1"}, push{20, "A1_1"}, pop{30}, pop{40}, finish{99}},
			&FullInterval{"root", sec(1), sec(99),
				[]FullInterval{{"A1", sec(10), sec(40),
					[]FullInterval{{"A1_1", sec(20), sec(30), nil}}}}},
			&FullInterval{"root", sec(1), sec(99),
				[]FullInterval{{"A1", sec(10), sec(99),
					[]FullInterval{{"A1_1", sec(20), sec(99), nil}}}}},
			`
00:00:01.000 root       98.000s       00:01:39.000
00:00:01.000    *           9.000s    00:00:10.000
00:00:10.000    A1         30.000s    00:00:40.000
00:00:10.000       *          10.000s 00:00:20.000
00:00:20.000       A1_1       10.000s 00:00:30.000
00:00:30.000       *          10.000s 00:00:40.000
00:00:40.000    *          59.000s    00:01:39.000
`,
			`
00:00:01.000 root       98.000s       00:01:39.000
00:00:01.000    *           9.000s    00:00:10.000
00:00:10.000    A1         89.000s    00:01:39.000
00:00:10.000       *          10.000s 00:00:20.000
00:00:20.000       A1_1       79.000s 00:01:39.000
`,
		},
		{
			[]op{push{10, "A1"}, push{20, "A1_1"}, finish{30}, push{40, "B1"}},
			&FullInterval{"root", sec(1), sec(0), []FullInterval{
				{"A1", sec(10), sec(30), []FullInterval{
					{"A1_1", sec(20), sec(30), nil}}},
				{"B1", sec(40), sec(0), nil}}},
			&FullInterval{"root", sec(1), sec(0), []FullInterval{
				{"A1", sec(10), sec(40), []FullInterval{
					{"A1_1", sec(20), sec(40), nil}}},
				{"B1", sec(40), sec(0), nil}}},
			`
00:00:01.000 root       999.000s       ---------now
00:00:01.000    *            9.000s    00:00:10.000
00:00:10.000    A1          20.000s    00:00:30.000
00:00:10.000       *           10.000s 00:00:20.000
00:00:20.000       A1_1        10.000s 00:00:30.000
00:00:30.000    *           10.000s    00:00:40.000
00:00:40.000    B1         960.000s    ---------now
`,
			`
00:00:01.000 root       999.000s       ---------now
00:00:01.000    *            9.000s    00:00:10.000
00:00:10.000    A1          30.000s    00:00:40.000
00:00:10.000       *           10.000s 00:00:20.000
00:00:20.000       A1_1        20.000s 00:00:40.000
00:00:40.000    B1         960.000s    ---------now
`,
		},
		{
			[]op{push{10, "A1"}, push{20, "A1_1"}, finish{30}, push{40, "B1"}, finish{99}},
			&FullInterval{"root", sec(1), sec(99), []FullInterval{
				{"A1", sec(10), sec(30), []FullInterval{
					{"A1_1", sec(20), sec(30), nil}}},
				{"B1", sec(40), sec(99), nil}}},
			&FullInterval{"root", sec(1), sec(99), []FullInterval{
				{"A1", sec(10), sec(40), []FullInterval{
					{"A1_1", sec(20), sec(40), nil}}},
				{"B1", sec(40), sec(99), nil}}},
			`
00:00:01.000 root       98.000s       00:01:39.000
00:00:01.000    *           9.000s    00:00:10.000
00:00:10.000    A1         20.000s    00:00:30.000
00:00:10.000       *          10.000s 00:00:20.000
00:00:20.000       A1_1       10.000s 00:00:30.000
00:00:30.000    *          10.000s    00:00:40.000
00:00:40.000    B1         59.000s    00:01:39.000
`,
			`
00:00:01.000 root       98.000s       00:01:39.000
00:00:01.000    *           9.000s    00:00:10.000
00:00:10.000    A1         30.000s    00:00:40.000
00:00:10.000       *          10.000s 00:00:20.000
00:00:20.000       A1_1       20.000s 00:00:40.000
00:00:40.000    B1         59.000s    00:01:39.000
`,
		},
		{
			[]op{push{10, "foo"}, push{15, "foo1"}, pop{37}, push{37, "foo2"}, pop{55}, pop{55}, push{55, "bar"}, pop{80}, push{80, "baz"}, pop{99}, finish{99}},
			&FullInterval{
				"root", sec(1), sec(99), []FullInterval{
					{"foo", sec(10), sec(55), []FullInterval{
						{"foo1", sec(15), sec(37), nil},
						{"foo2", sec(37), sec(55), nil},
					}},
					{"bar", sec(55), sec(80), nil},
					{"baz", sec(80), sec(99), nil}}}, nil,
			`
00:00:01.000 root       98.000s       00:01:39.000
00:00:01.000    *           9.000s    00:00:10.000
00:00:10.000    foo        45.000s    00:00:55.000
00:00:10.000       *           5.000s 00:00:15.000
00:00:15.000       foo1       22.000s 00:00:37.000
00:00:37.000       foo2       18.000s 00:00:55.000
00:00:55.000    bar        25.000s    00:01:20.000
00:01:20.000    baz        19.000s    00:01:39.000
`, ``,
		},
		{
			[]op{push{10, "foo"}, push{15, "foo1"}, pop{30}, push{37, "foo2"}, pop{50}, pop{53}, push{55, "bar"}, pop{75}, push{80, "baz"}, pop{90}, finish{99}},
			&FullInterval{
				"root", sec(1), sec(99), []FullInterval{
					{"foo", sec(10), sec(53), []FullInterval{
						{"foo1", sec(15), sec(30), nil},
						{"foo2", sec(37), sec(50), nil},
					}},
					{"bar", sec(55), sec(75), nil},
					{"baz", sec(80), sec(90), nil}}},
			&FullInterval{
				"root", sec(1), sec(99), []FullInterval{
					{"foo", sec(10), sec(55), []FullInterval{
						{"foo1", sec(15), sec(37), nil},
						{"foo2", sec(37), sec(55), nil},
					}},
					{"bar", sec(55), sec(80), nil},
					{"baz", sec(80), sec(99), nil}}},
			`
00:00:01.000 root       98.000s       00:01:39.000
00:00:01.000    *           9.000s    00:00:10.000
00:00:10.000    foo        43.000s    00:00:53.000
00:00:10.000       *           5.000s 00:00:15.000
00:00:15.000       foo1       15.000s 00:00:30.000
00:00:30.000       *           7.000s 00:00:37.000
00:00:37.000       foo2       13.000s 00:00:50.000
00:00:50.000       *           3.000s 00:00:53.000
00:00:53.000    *           2.000s    00:00:55.000
00:00:55.000    bar        20.000s    00:01:15.000
00:01:15.000    *           5.000s    00:01:20.000
00:01:20.000    baz        10.000s    00:01:30.000
00:01:30.000    *           9.000s    00:01:39.000
`,
			`
00:00:01.000 root       98.000s       00:01:39.000
00:00:01.000    *           9.000s    00:00:10.000
00:00:10.000    foo        45.000s    00:00:55.000
00:00:10.000       *           5.000s 00:00:15.000
00:00:15.000       foo1       22.000s 00:00:37.000
00:00:37.000       foo2       18.000s 00:00:55.000
00:00:55.000    bar        25.000s    00:01:20.000
00:01:20.000    baz        19.000s    00:01:39.000
`,
		},
	}
	for _, test := range tests {
		for _, kind := range []kind{kindFull, kindCompact} {
			// Run all ops.
			now := &fakeNow{1}
			nowFunc = now.Now
			timer := kind.NewTimer("root")
			for _, op := range test.ops {
				op.run(now, timer)
			}
			// Check all intervals.
			wantRoot := test.full
			if kind == kindCompact && test.compact != nil {
				wantRoot = test.compact
			}
			name := fmt.Sprintf("%s %#v", kind.String(), test.ops)
			expectEqualIntervals(t, name, timer.Root(), *wantRoot)
			// Check string output.
			now.now = 1000
			wantStr := test.fullStr
			if kind == kindCompact && test.compactStr != "" {
				wantStr = test.compactStr
			}
			if got, want := timer.String(), strings.TrimLeft(wantStr, "\n"); got != want {
				t.Errorf("%s GOT STRING\n%sWANT\n%s", name, got, want)
			}
			// Check print output hiding all gaps.
			var buf bytes.Buffer
			printer := IntervalPrinter{MinGap: time.Hour}
			if err := printer.Print(&buf, timer.Root()); err != nil {
				t.Errorf("%s got printer error: %v", name, err)
			}
			if got, want := buf.String(), stripGaps(wantStr); got != want {
				t.Errorf("%s GOT PRINT\n%sWANT\n%s", name, got, want)
			}
		}
	}
	nowFunc = time.Now
}

func expectEqualIntervals(t *testing.T, name string, a, b Interval) {
	if got, want := a.Name(), b.Name(); got != want {
		t.Errorf("%s got name %q, want %q", name, got, want)
	}
	if got, want := a.Start(), b.Start(); !got.Equal(want) {
		t.Errorf("%s got start %v, want %v", name, got, want)
	}
	if got, want := a.End(), b.End(); !got.Equal(want) {
		t.Errorf("%s got end %v, want %v", name, got, want)
	}
	if got, want := a.NumChild(), b.NumChild(); got != want {
		t.Errorf("%s got num child %v, want %v", name, got, want)
		return
	}
	for child := 0; child < a.NumChild(); child++ {
		expectEqualIntervals(t, name, a.Child(child), b.Child(child))
	}
}

// stripGaps strips out leading newlines, and also strips any line with an
// asterisk (*).  Asterisks appear in lines with gaps, as shown here:
//    00:00:01.000 root   98.000s    00:01:39.000
//    00:00:01.000    *       9.000s 00:00:10.000
//    00:00:10.000    abc    89.000s 00:01:39.000
func stripGaps(out string) string {
	out = strings.TrimLeft(out, "\n")
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if !strings.ContainsRune(line, '*') {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func BenchmarkFullTimerPush(b *testing.B) {
	t := NewFullTimer("root")
	for i := 0; i < b.N; i++ {
		t.Push("child")
	}
}
func BenchmarkCompactTimerPush(b *testing.B) {
	t := NewCompactTimer("root")
	for i := 0; i < b.N; i++ {
		t.Push("child")
	}
}

func BenchmarkFullTimerPushPop(b *testing.B) {
	t := NewFullTimer("root")
	for i := 0; i < b.N; i++ {
		timerPushPop(t)
	}
}
func BenchmarkCompactTimerPushPop(b *testing.B) {
	t := NewCompactTimer("root")
	for i := 0; i < b.N; i++ {
		timerPushPop(t)
	}
}
func timerPushPop(t Timer) {
	t.Push("child1")
	t.Pop()
	t.Push("child2")
	t.Push("child2_1")
	t.Pop()
	t.Push("child2_2")
	t.Pop()
	t.Pop()
	t.Push("child3")
	t.Pop()
}

var randSource = rand.NewSource(123)

func BenchmarkFullTimerRandom(b *testing.B) {
	t, rng := NewFullTimer("root"), rand.New(randSource)
	for i := 0; i < b.N; i++ {
		timerRandom(rng, t)
	}
}
func BenchmarkCompactTimerRandom(b *testing.B) {
	t, rng := NewCompactTimer("root"), rand.New(randSource)
	for i := 0; i < b.N; i++ {
		timerRandom(rng, t)
	}
}
func timerRandom(rng *rand.Rand, t Timer) {
	switch pct := rng.Intn(100); {
	case pct < 60:
		timerPushPop(t)
	case pct < 90:
		t.Push("foo")
	case pct < 95:
		t.Pop()
	default:
		t.Finish()
	}
}

func BenchmarkFullTimerPrint(b *testing.B) {
	now := &stepNow{0}
	nowFunc = now.Now
	benchTimerPrint(b, NewFullTimer("root"))
	nowFunc = time.Now
}
func BenchmarkCompactTimerPrint(b *testing.B) {
	now := &stepNow{0}
	nowFunc = now.Now
	benchTimerPrint(b, NewCompactTimer("root"))
	nowFunc = time.Now
}
func benchTimerPrint(b *testing.B, t Timer) {
	rng := rand.New(randSource)
	for i := 0; i < 1000; i++ {
		timerRandom(rng, t)
	}
	t.Finish() // Make sure all intervals are closed.
	want := t.String()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := t.String(); got != want {
			b.Fatalf("GOT\n%sWANT\n%s\nTIMER\n%#v", got, want, t)
		}
	}
}
