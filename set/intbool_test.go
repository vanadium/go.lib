// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package set

import (
	"testing"
)

// nolint: gocyclo
func TestIntBool(t *testing.T) {
	slice := []int{}

	slice = append(slice, -1)
	slice = append(slice, 1)

	// Test conversion from a slice.
	s1 := IntBool.FromSlice(slice)
	for i, want := range []bool{true, true} {
		if _, got := s1[slice[i]]; got != want {
			t.Errorf("index %d: got %v, want %v", i, got, want)
		}
	}
	if got, want := len(s1), len(slice); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Test conversion to a slice.
	slice2 := IntBool.ToSlice(s1)
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
		s1 := IntBool.FromSlice(slice)
		s2 := IntBool.FromSlice(slice[1:])
		IntBool.Difference(s1, s2)
		for i, want := range []bool{true, false} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := IntBool.FromSlice(slice[1:])
		s4 := IntBool.FromSlice(slice)
		IntBool.Difference(s3, s4)
		for i, want := range []bool{false, false} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := IntBool.FromSlice(slice[1:])
		s6 := IntBool.FromSlice(slice[:1])
		IntBool.Difference(s5, s6)
		for i, want := range []bool{false, true} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := IntBool.FromSlice(slice)
		s8 := IntBool.FromSlice(slice)
		IntBool.Difference(s7, s8)
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
		s1 := IntBool.FromSlice(slice)
		s2 := IntBool.FromSlice(slice[1:])
		IntBool.Intersection(s1, s2)
		for i, want := range []bool{false, true} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := IntBool.FromSlice(slice[1:])
		s4 := IntBool.FromSlice(slice)
		IntBool.Intersection(s3, s4)
		for i, want := range []bool{false, true} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := IntBool.FromSlice(slice[1:])
		s6 := IntBool.FromSlice(slice[:1])
		IntBool.Intersection(s5, s6)
		for i, want := range []bool{false, false} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := IntBool.FromSlice(slice)
		s8 := IntBool.FromSlice(slice)
		IntBool.Intersection(s7, s8)
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
		s1 := IntBool.FromSlice(slice[:1])
		s2 := IntBool.FromSlice(slice[1:])
		IntBool.Union(s1, s2)
		for i, want := range []bool{true, true} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := IntBool.FromSlice(slice[1:])
		s4 := IntBool.FromSlice(slice)
		IntBool.Union(s3, s4)
		for i, want := range []bool{true, true} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := IntBool.FromSlice(slice[1:])
		s6 := IntBool.FromSlice(slice[:1])
		IntBool.Union(s5, s6)
		for i, want := range []bool{true, true} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := IntBool.FromSlice(slice)
		s8 := IntBool.FromSlice(slice)
		IntBool.Union(s7, s8)
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
