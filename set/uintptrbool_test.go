// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package set

import (
	"testing"
)

func TestUintptrBool(t *testing.T) {
	slice := []uintptr{}

	slice = append(slice, 0)
	slice = append(slice, 1)

	// Test conversion from a slice.
	s1 := UintptrBool.FromSlice(slice)
	for i, want := range []bool{true, true} {
		if _, got := s1[slice[i]]; got != want {
			t.Errorf("index %d: got %v, want %v", i, got, want)
		}
	}
	if got, want := len(s1), len(slice); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Test conversion to a slice.
	slice2 := UintptrBool.ToSlice(s1)
	for i, got := range []bool{true, true} {
		if _, want := s1[(slice2[i])]; got != want {
			t.Errorf("index %d: got %v, want %v", i, got, want)
		}
	}
	if got, want := len(slice2), len(s1); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Test set difference.
	{
		s1 := UintptrBool.FromSlice(slice)
		s2 := UintptrBool.FromSlice(slice[1:])
		UintptrBool.Difference(s1, s2)
		for i, want := range []bool{true, false} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := UintptrBool.FromSlice(slice[1:])
		s4 := UintptrBool.FromSlice(slice)
		UintptrBool.Difference(s3, s4)
		for i, want := range []bool{false, false} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := UintptrBool.FromSlice(slice[1:])
		s6 := UintptrBool.FromSlice(slice[:1])
		UintptrBool.Difference(s5, s6)
		for i, want := range []bool{false, true} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := UintptrBool.FromSlice(slice)
		s8 := UintptrBool.FromSlice(slice)
		UintptrBool.Difference(s7, s8)
		for i, want := range []bool{false, false} {
			if _, got := s7[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s7), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	// Test set intersection.
	{
		s1 := UintptrBool.FromSlice(slice)
		s2 := UintptrBool.FromSlice(slice[1:])
		UintptrBool.Intersection(s1, s2)
		for i, want := range []bool{false, true} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := UintptrBool.FromSlice(slice[1:])
		s4 := UintptrBool.FromSlice(slice)
		UintptrBool.Intersection(s3, s4)
		for i, want := range []bool{false, true} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := UintptrBool.FromSlice(slice[1:])
		s6 := UintptrBool.FromSlice(slice[:1])
		UintptrBool.Intersection(s5, s6)
		for i, want := range []bool{false, false} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := UintptrBool.FromSlice(slice)
		s8 := UintptrBool.FromSlice(slice)
		UintptrBool.Intersection(s7, s8)
		for i, want := range []bool{true, true} {
			if _, got := s7[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s7), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	// Test set union.
	{
		s1 := UintptrBool.FromSlice(slice[:1])
		s2 := UintptrBool.FromSlice(slice[1:])
		UintptrBool.Union(s1, s2)
		for i, want := range []bool{true, true} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := UintptrBool.FromSlice(slice[1:])
		s4 := UintptrBool.FromSlice(slice)
		UintptrBool.Union(s3, s4)
		for i, want := range []bool{true, true} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := UintptrBool.FromSlice(slice[1:])
		s6 := UintptrBool.FromSlice(slice[:1])
		UintptrBool.Union(s5, s6)
		for i, want := range []bool{true, true} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := UintptrBool.FromSlice(slice)
		s8 := UintptrBool.FromSlice(slice)
		UintptrBool.Union(s7, s8)
		for i, want := range []bool{true, true} {
			if _, got := s7[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s7), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}
