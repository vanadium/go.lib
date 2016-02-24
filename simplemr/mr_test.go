// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simplemr_test

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"testing"
	"time"

	"v.io/x/lib/simplemr"
)

func newChans(chanSize int) (chan *simplemr.Record, chan *simplemr.Record) {
	return make(chan *simplemr.Record, chanSize), make(chan *simplemr.Record, chanSize)
}

type termCount struct{}

func (tc *termCount) Map(mr *simplemr.MR, key string, val interface{}) error {
	text, ok := val.(string)
	if !ok {
		return fmt.Errorf("%T is the wrong type", val)
	}
	for _, token := range strings.Split(text, " ") {
		mr.MapOut(token, 1)
	}
	return nil
}

func (tc *termCount) Reduce(mr *simplemr.MR, key string, values []interface{}) error {
	count := 0
	for _, val := range values {
		c, ok := val.(int)
		if !ok {
			return fmt.Errorf("%T is the wrong type", val)
		}
		count += c
	}
	mr.ReduceOut(key, count)
	return nil
}

var (
	d1 = "a b c"
	d2 = "a b c d"
	d3 = "e f"
)

func expect(t *testing.T, out chan *simplemr.Record, key string, vals ...int) {
	rec := <-out
	if got, want := rec.Key, key; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := len(rec.Values), len(vals); got != want {
		t.Errorf("got %v, want %v", got, want)
		return
	}
	for i, v := range vals {
		if got, want := rec.Values[i], v; got != want {
			t.Errorf("%d: got %v, want %v", i, got, want)
		}
	}
}

func TestMR(t *testing.T) {
	mrt := &simplemr.MR{}
	in, out := newChans(10)
	tc := &termCount{}
	genInput := func() {
		in <- &simplemr.Record{"d1", []interface{}{d1, d2, d3}}
		in <- &simplemr.Record{"d2", []interface{}{d1, d2, d3}}
		close(in)
	}
	go genInput()
	if err := mrt.Run(in, out, tc, tc); err != nil {
		t.Fatal(err)
	}

	expect(t, out, "a", 4)
	expect(t, out, "b", 4)
	expect(t, out, "c", 4)
	expect(t, out, "d", 2)
	expect(t, out, "e", 2)
	expect(t, out, "f", 2)
	kvs := <-out
	if kvs != nil {
		t.Fatal("expected the channel to be closed")
	}
}

type slowReducer struct{}

func (sr *slowReducer) Reduce(mr *simplemr.MR, key string, values []interface{}) error {
	time.Sleep(time.Hour)
	return nil
}

func TestTimeout(t *testing.T) {
	in, out := newChans(1)
	mrt := &simplemr.MR{Timeout: 100 * time.Millisecond}
	identity := &simplemr.Identity{}
	mrt.Run(in, out, identity, identity)
	if err := mrt.Error(); err == nil || !strings.Contains(err.Error(), "timed out mappers") {
		t.Fatalf("missing or wrong error: %v", err)
	}
	mrt = &simplemr.MR{Timeout: 100 * time.Millisecond}
	in, out = newChans(1)
	in <- &simplemr.Record{"key", []interface{}{"value"}}
	close(in)
	mrt.Run(in, out, identity, &slowReducer{})
	if err := mrt.Error(); err == nil || !strings.Contains(err.Error(), "timed out reducers") {
		t.Fatalf("missing or wrong error: %v", err)
	}
}

type sleeper struct{}

const sleepTime = time.Millisecond * 100

func (sl *sleeper) Map(mr *simplemr.MR, key string, val interface{}) error {
	time.Sleep(sleepTime)
	mr.MapOut(key, val)
	return nil
}

func (sl *sleeper) Reduce(mr *simplemr.MR, key string, values []interface{}) error {
	mr.ReduceOut(key, values...)
	return nil
}

func runMappers(t *testing.T, bufsize, numMappers int) time.Duration {
	mrt := &simplemr.MR{NumMappers: numMappers}
	in, out := newChans(bufsize)
	sl := &sleeper{}
	go func() {
		for i := 0; i < bufsize; i++ {
			in <- &simplemr.Record{Key: fmt.Sprintf("%d", i), Values: []interface{}{i}}
		}
		close(in)
	}()
	then := time.Now()
	if err := mrt.Run(in, out, sl, sl); err != nil {
		t.Fatal(err)
	}
	return time.Since(then)
}

func TestOneMappers(t *testing.T) {
	bufsize := 5
	runtime := runMappers(t, bufsize, 1)
	if got, want := runtime, time.Duration(int64(sleepTime)*int64(bufsize)); got < want {
		t.Errorf("took %s which is too fast, should be at least %s", got, want)
	}
}

func TestMultipleMappers(t *testing.T) {
	numCPUs := runtime.NumCPU()
	if numCPUs == 1 {
		t.Skip("can't test concurrency with only one CPU")
	}
	bufsize := 5
	runtime := runMappers(t, bufsize, numCPUs)
	if got, want := runtime, time.Duration(int64(sleepTime)*int64(bufsize)); got > want {
		t.Errorf("took %s which is too slow, should take no longer than %s", got, want)
	}
}

type adder struct{}

func (a *adder) Map(mr *simplemr.MR, key string, val interface{}) error {
	i := val.(int)
	i++
	mr.MapOut(key, i)
	return nil
}

func (a *adder) Reduce(mr *simplemr.MR, key string, values []interface{}) error {
	mr.ReduceOut(key, values...)
	return nil
}

func TestChainedMR(t *testing.T) {
	chanSize := 5
	in, middle, out := make(chan *simplemr.Record, chanSize), make(chan *simplemr.Record, chanSize), make(chan *simplemr.Record, chanSize)
	mrt1 := &simplemr.MR{}
	mrt2 := &simplemr.MR{}
	adder := &adder{}
	go mrt1.Run(in, middle, adder, adder)
	go mrt2.Run(middle, out, adder, adder)
	in <- &simplemr.Record{"1", []interface{}{1}}
	in <- &simplemr.Record{"2", []interface{}{2}}
	close(in)
	expect(t, out, "1", 3)
	expect(t, out, "2", 4)
	if err := mrt1.Error(); err != nil {
		t.Fatal(err)
	}
	if err := mrt2.Error(); err != nil {
		t.Fatal(err)
	}
}

type cancelMR struct{ cancelMapper bool }

var (
	errMapperCancelled  = errors.New("mapper cancelled")
	errReducerCancelled = errors.New("reducer cancelled")
)

func cancelEg(mr *simplemr.MR) error {
	delay := rand.Int63n(1000) * int64(time.Millisecond)
	select {
	case <-mr.CancelCh():
		return nil
	case <-time.After(time.Duration(delay)):
		mr.Cancel()
		return nil
	case <-time.After(time.Hour):
	}
	return fmt.Errorf("timeout")
}

func (c *cancelMR) Map(mr *simplemr.MR, key string, val interface{}) error {
	if c.cancelMapper {
		return cancelEg(mr)
	}
	mr.MapOut(key, val)
	return nil
}

func (c *cancelMR) Reduce(mr *simplemr.MR, key string, values []interface{}) error {
	if !c.cancelMapper {
		return cancelEg(mr)
	}
	panic("should never get here")
	return nil
}

func testCancel(t *testing.T, mapper bool) {
	mrt := &simplemr.MR{}
	in, out := newChans(10)
	cancel := &cancelMR{true}
	genInput := func() {
		in <- &simplemr.Record{"d1", []interface{}{d1, d2, d3}}
		in <- &simplemr.Record{"d2", []interface{}{d1, d2, d3}}
		close(in)
	}
	go genInput()
	if got, want := mrt.Run(in, out, cancel, cancel), simplemr.ErrMRCancelled; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCancelMappers(t *testing.T) {
	testCancel(t, true)
}

func TestCancelReducers(t *testing.T) {
	testCancel(t, false)
}
