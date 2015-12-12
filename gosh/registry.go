// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

// Inspired by https://github.com/golang/appengine/blob/master/delay/delay.go.

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"reflect"
)

// Fn is a registered, callable function.
type Fn struct {
	name  string
	value reflect.Value
}

var (
	fns       = map[string]*Fn{}
	errorType = reflect.TypeOf((*error)(nil)).Elem()
)

// Register registers the given function with the given name. 'name' must be
// unique across the dependency graph; 'fni' must be a function that accepts
// gob-encodable arguments and returns an error or nothing.
func Register(name string, fni interface{}) *Fn {
	// TODO(sadovsky): Switch to using len(fns) as name, and maybe drop the name
	// argument, if it turns out that initialization order is deterministic.
	if _, ok := fns[name]; ok {
		panic(fmt.Errorf("%s: already registered", name))
	}
	v := reflect.ValueOf(fni)
	t := v.Type()
	if t.Kind() != reflect.Func {
		panic(fmt.Errorf("%s: not a function: %v", name, t.Kind()))
	}
	if t.NumOut() > 1 || t.NumOut() == 1 && t.Out(0) != errorType {
		panic(fmt.Errorf("%s: function must return an error or nothing: %v", name, t))
	}
	// Register the function's args with gob. Needed because Shell.Fn() takes
	// interface{} arguments.
	for i := 0; i < t.NumIn(); i++ {
		// Note: Clients are responsible for registering any concrete types stored
		// inside interface{} arguments.
		if t.In(i).Kind() == reflect.Interface {
			continue
		}
		gob.Register(reflect.Zero(t.In(i)).Interface())
	}
	fn := &Fn{name: name, value: v}
	fns[name] = fn
	return fn
}

// Call calls the named function, which must have been registered.
func Call(name string, args ...interface{}) error {
	if fn, ok := fns[name]; !ok {
		return fmt.Errorf("unknown function: %s", name)
	} else {
		return fn.Call(args...)
	}
}

// Call calls the function fn with the input arguments args.
func (fn *Fn) Call(args ...interface{}) error {
	t := fn.value.Type()
	in := []reflect.Value{}
	for i, arg := range args {
		var av reflect.Value
		if arg != nil {
			av = reflect.ValueOf(arg)
		} else {
			// Client passed nil; construct the zero value for this argument based on
			// the function signature.
			at := t.In(i)
			if t.IsVariadic() && i == t.NumIn()-1 {
				at = at.Elem()
			}
			av = reflect.Zero(at)
		}
		in = append(in, av)
	}
	out := fn.value.Call(in)
	if t.NumOut() == 1 && !out[0].IsNil() {
		return out[0].Interface().(error)
	}
	return nil
}

////////////////////////////////////////
// invocation

type invocation struct {
	Name string
	Args []interface{}
}

// encInvocation encodes an invocation.
func encInvocation(name string, args ...interface{}) (string, error) {
	inv := invocation{Name: name, Args: args}
	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(inv); err != nil {
		return "", fmt.Errorf("failed to encode invocation: %v", err)
	}
	// Base64-encode the gob-encoded bytes so that the result can be used as an
	// env var value.
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// decInvocation decodes an invocation.
func decInvocation(s string) (name string, args []interface{}, err error) {
	var inv invocation
	b, err := base64.StdEncoding.DecodeString(s)
	if err == nil {
		err = gob.NewDecoder(bytes.NewReader(b)).Decode(&inv)
	}
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode invocation: %v", err)
	}
	return inv.Name, inv.Args, nil
}
