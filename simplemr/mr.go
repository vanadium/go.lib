// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package simplemr provides a simple map reduce framework for use by
// commandline and other tools and consequently can only be used from
// within a single process. It is specifically not intended to support
// large datasets, but mappers are run concurrently so that long running
// tasks (e.g. external shell commands will be run in parallel). The
// current implementation supoorts only a single reducer however future
// implementations are likely to run multiple reducers and hence reducers
// should be coded accordingly.
package simplemr

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"
)

var ErrMRCancelled = errors.New("MR cancelled")

// Mapper is in the interface that must be implemented by all mappers.
type Mapper interface {
	// Map is called by the framework for every key, value pair read
	// from the specified input.
	Map(mr *MR, key string, value interface{}) error
}

// Reducer is the interface that must be implemented by the reducer.
type Reducer interface {
	// Reduce is called by the framework for every key and associated
	// values that are emitted by the Mappers.
	Reduce(mr *MR, key string, values []interface{}) error
}

// Record represents all input and output data.
type Record struct {
	Key    string
	Values []interface{}
}

type store struct {
	sync.Mutex
	data map[string][]interface{}
}

func newStore() *store {
	return &store{data: make(map[string][]interface{})}
}

func (s *store) sortedKeys() []string {
	s.Lock()
	defer s.Unlock()
	keys := make([]string, 0, len(s.data))
	for k, _ := range s.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *store) insert(k string, v ...interface{}) {
	s.Lock()
	defer s.Unlock()
	s.data[k] = append(s.data[k], v...)
}

func (s *store) lookup(k string) []interface{} {
	s.Lock()
	defer s.Unlock()
	return s.data[k]
}

// MR represents the Map Reduction.
type MR struct {
	input        <-chan *Record
	output       chan<- *Record
	cancel       chan struct{}
	cancelled    bool
	cancelled_mu sync.RWMutex // guards cancelled
	err          error
	err_mu       sync.RWMutex // guards err
	data         *store

	// The number of conccurent mappers to use. A value of 0 instructs
	// the implementation to use an appropriate number, such as the number
	// of available CPUs.
	NumMappers int
	// The time to wait for the map reduce to complete. A value of 0 implies
	// no timeout - i.e. an infinite wait.
	Timeout time.Duration
}

// Error returns any error that was returned by the Run method. It is
// safe to read its value once the output channel passed to Run has been
// closed.
func (mr *MR) Error() error {
	mr.err_mu.RLock()
	defer mr.err_mu.RUnlock()
	return mr.err
}

// MapOut outputs the key and associated values for subsequent
// processing by a Reducer. It should only be called from a mapper.
func (mr *MR) MapOut(key string, values ...interface{}) {
	mr.data.insert(key, values...)
}

// ReduceOut outputs the key and associated values to the specified output
// stream. It should only be called from a reducer.
func (mr *MR) ReduceOut(key string, values ...interface{}) {
	mr.output <- &Record{key, values}
}

// CancelCh returns a channel that will be closed when the Cancel
// method is called. It should only be called by a mapper or reducer.
func (mr *MR) CancelCh() <-chan struct{} {
	return mr.cancel
}

// Cancel closes the channel intended to be used for monitoring
// cancellation requests. If Cancel is called before any reducers
// have been run then no reducers will be run. It can only be called
// after mr.Run has been called, generally by a mapper or a reducer.
func (mr *MR) Cancel() {
	mr.cancelled_mu.Lock()
	defer mr.cancelled_mu.Unlock()
	if mr.cancelled {
		return
	}
	close(mr.cancel)
	mr.cancelled = true
}

// IsCancelled returns true if this MR has been cancelled.
func (mr *MR) IsCancelled() bool {
	mr.cancelled_mu.RLock()
	defer mr.cancelled_mu.RUnlock()
	return mr.cancelled
}

func (mr *MR) runMapper(ch chan error, mapper Mapper) {
	for {
		rec := <-mr.input
		if rec == nil {
			ch <- nil
			return
		}
		for _, v := range rec.Values {
			if err := mapper.Map(mr, rec.Key, v); err != nil {
				ch <- err
				return
			}
		}
	}
}

func (mr *MR) runMappers(mapper Mapper, timeout <-chan time.Time) error {
	ch := make(chan error, mr.NumMappers)
	for i := 0; i < mr.NumMappers; i++ {
		go mr.runMapper(ch, mapper)
	}
	done := 0
	for {
		select {
		case err := <-ch:
			if err != nil {
				// We should probably drain the channel.
				return err
			}
			done++
			if done == mr.NumMappers {
				return nil
			}
		case <-mr.cancel:
			return ErrMRCancelled
		case <-timeout:
			return fmt.Errorf("timed out mappers after %s", mr.Timeout)
		}
	}
}

func (mr *MR) runReducers(reducer Reducer, timeout <-chan time.Time) error {
	ch := make(chan error, 1)
	go func() {
		for _, k := range mr.data.sortedKeys() {
			v := mr.data.lookup(k)
			if err := reducer.Reduce(mr, k, v); err != nil {
				ch <- err
			}
		}
		close(ch)
	}()
	var err error
	select {
	case err = <-ch:
	case <-timeout:
		err = fmt.Errorf("timed out reducers after %s", mr.Timeout)
	}
	return err
}

// Run runs the map reduction using the supplied mapper and reducer reading
// from input and writing to output. The caller must close the input channel
// when there is no more input data. The implementation of Run will close
// the output channel when the Reducer has processed all intermediate data.
// Run may only be called once per MR receiver.
func (mr *MR) Run(input <-chan *Record, output chan<- *Record, mapper Mapper, reducer Reducer) error {
	mr.input, mr.output, mr.data = input, output, newStore()
	mr.cancel = make(chan struct{})
	if mr.NumMappers == 0 {
		// TODO(cnicolaou,toddw): consider using a new goroutine
		// for every input record rather than fixing concurrency like
		// this. Maybe an another option is to use the capacity of the
		// input channel.
		mr.NumMappers = runtime.NumCPU()
	}
	var timeout <-chan time.Time
	if mr.Timeout > 0 {
		timeout = time.After(mr.Timeout)
	}
	defer close(mr.output)
	if err := mr.runMappers(mapper, timeout); err != nil {
		mr.err_mu.Lock()
		mr.err = err
		mr.err_mu.Unlock()
		return err
	}
	if mr.IsCancelled() {
		return ErrMRCancelled
	}
	err := mr.runReducers(reducer, timeout)
	mr.err_mu.Lock()
	mr.err = err
	mr.err_mu.Unlock()
	if mr.IsCancelled() {
		return ErrMRCancelled
	}
	return err
}
