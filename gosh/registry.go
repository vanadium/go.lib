// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

// Inspired by https://github.com/golang/appengine/blob/master/delay/delay.go.

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
)

// Func is a registered, callable function.
type Func struct {
	handle string
	value  reflect.Value
}

var (
	errorType = reflect.TypeOf((*error)(nil)).Elem()
	funcsMu   = sync.RWMutex{} // protects funcs
	funcs     = map[string]*Func{}
)

// RegisterFunc registers the given function with the given name. 'fi' must be a
// function that accepts gob-encodable arguments and returns an error or
// nothing.
func RegisterFunc(name string, fi interface{}) *Func {
	funcsMu.Lock()
	defer funcsMu.Unlock()
	_, file, line, _ := runtime.Caller(1)
	handle := fmt.Sprintf("%s:%d:%s", file, line, name)
	if _, ok := funcs[handle]; ok {
		panic(fmt.Errorf("gosh: %q is already registered", handle))
	}
	v := reflect.ValueOf(fi)
	t := v.Type()
	if t.Kind() != reflect.Func {
		panic(fmt.Errorf("gosh: %q is not a function: %v", name, t.Kind()))
	}
	if t.NumOut() > 1 || t.NumOut() == 1 && t.Out(0) != errorType {
		panic(fmt.Errorf("gosh: %q must return an error or nothing: %v", name, t))
	}
	// Register the function's args with gob. Needed because Shell.Func takes
	// interface{} arguments.
	for i := 0; i < t.NumIn(); i++ {
		// Note: Users are responsible for registering any concrete types stored
		// inside interface{} arguments.
		if t.In(i).Kind() == reflect.Interface {
			continue
		}
		gob.Register(reflect.Zero(t.In(i)).Interface())
	}
	f := &Func{handle: handle, value: v}
	funcs[handle] = f
	return f
}

// getFunc returns the referenced function.
func getFunc(handle string) (*Func, error) {
	funcsMu.RLock()
	f, ok := funcs[handle]
	funcsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("gosh: unknown function %q", handle)
	}
	return f, nil
}

// callFunc calls the referenced function, which must have been registered.
func callFunc(handle string, args ...interface{}) error {
	f, err := getFunc(handle)
	if err != nil {
		return err
	}
	return f.call(args...)
}

// call calls this Func with the given input arguments.
func (f *Func) call(args ...interface{}) error {
	t := f.value.Type()
	in := []reflect.Value{}
	for i, arg := range args {
		var av reflect.Value
		if arg != nil {
			av = reflect.ValueOf(arg)
		} else {
			// User passed nil; construct the zero value for this argument based on
			// the function signature.
			av = reflect.Zero(argType(t, i))
		}
		in = append(in, av)
	}
	out := f.value.Call(in)
	if t.NumOut() == 1 && !out[0].IsNil() {
		return out[0].Interface().(error)
	}
	return nil
}

// argType returns the type of the nth argument to a function of type t.
func argType(t reflect.Type, n int) reflect.Type {
	if !t.IsVariadic() || n < t.NumIn()-1 {
		return t.In(n)
	}
	return t.In(t.NumIn() - 1).Elem()
}

// checkCall checks that the referenced function exists and can be called with
// the given arguments. Modeled after the implementation of reflect.Value.call.
func checkCall(handle string, args ...interface{}) error {
	f, err := getFunc(handle)
	if err != nil {
		return err
	}
	t := f.value.Type()
	n := t.NumIn()
	if t.IsVariadic() {
		n--
	}
	if len(args) < n {
		return errors.New("gosh: too few input arguments")
	}
	if !t.IsVariadic() && len(args) > n {
		return errors.New("gosh: too many input arguments")
	}
	for i, arg := range args {
		if arg == nil {
			continue
		}
		if at, et := reflect.ValueOf(arg).Type(), argType(t, i); !at.AssignableTo(et) {
			return fmt.Errorf("gosh: cannot use %s as type %s", at, et)
		}
	}
	return nil
}

////////////////////////////////////////
// invocation

type invocation struct {
	Handle string
	Args   []interface{}
}

// encodeInvocation encodes an invocation.
func encodeInvocation(handle string, args ...interface{}) (string, error) {
	if err := checkCall(handle, args...); err != nil {
		return "", err
	}
	inv := invocation{Handle: handle, Args: args}
	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(inv); err != nil {
		return "", fmt.Errorf("gosh: failed to encode invocation: %v", err)
	}
	// Base64-encode the gob-encoded bytes so that the result can be used as an
	// env var value.
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// decodeInvocation decodes an invocation.
func decodeInvocation(s string) (handle string, args []interface{}, err error) {
	var inv invocation
	b, err := base64.StdEncoding.DecodeString(s)
	if err == nil {
		err = gob.NewDecoder(bytes.NewReader(b)).Decode(&inv)
	}
	if err != nil {
		return "", nil, fmt.Errorf("gosh: failed to decode invocation: %v", err)
	}
	return inv.Handle, inv.Args, nil
}
