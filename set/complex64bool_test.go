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
func TestComplex64Bool(t *testing.T) {
	slice := []complex64{}

	slice = append(slice, complex(-1.0, 1.0))
	slice = append(slice, complex(1.0, -1.0))

	// Test conversion from a slice.
	s1 := Complex64Bool.FromSlice(slice)
	for i, want := range []bool{true, true} {
		if _, got := s1[slice[i]]; got != want {
			t.Errorf("index %d: got %v, want %v", i, got, want)
		}
	}
	if got, want := len(s1), len(slice); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Test conversion to a slice.
	slice2 := Complex64Bool.ToSlice(s1)
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
		s1 := Complex64Bool.FromSlice(slice)
		s2 := Complex64Bool.FromSlice(slice[1:])
		Complex64Bool.Difference(s1, s2)
		for i, want := range []bool{true, false} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := Complex64Bool.FromSlice(slice[1:])
		s4 := Complex64Bool.FromSlice(slice)
		Complex64Bool.Difference(s3, s4)
		for i, want := range []bool{false, false} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := Complex64Bool.FromSlice(slice[1:])
		s6 := Complex64Bool.FromSlice(slice[:1])
		Complex64Bool.Difference(s5, s6)
		for i, want := range []bool{false, true} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := Complex64Bool.FromSlice(slice)
		s8 := Complex64Bool.FromSlice(slice)
		Complex64Bool.Difference(s7, s8)
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
		s1 := Complex64Bool.FromSlice(slice)
		s2 := Complex64Bool.FromSlice(slice[1:])
		Complex64Bool.Intersection(s1, s2)
		for i, want := range []bool{false, true} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := Complex64Bool.FromSlice(slice[1:])
		s4 := Complex64Bool.FromSlice(slice)
		Complex64Bool.Intersection(s3, s4)
		for i, want := range []bool{false, true} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := Complex64Bool.FromSlice(slice[1:])
		s6 := Complex64Bool.FromSlice(slice[:1])
		Complex64Bool.Intersection(s5, s6)
		for i, want := range []bool{false, false} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := Complex64Bool.FromSlice(slice)
		s8 := Complex64Bool.FromSlice(slice)
		Complex64Bool.Intersection(s7, s8)
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
		s1 := Complex64Bool.FromSlice(slice[:1])
		s2 := Complex64Bool.FromSlice(slice[1:])
		Complex64Bool.Union(s1, s2)
		for i, want := range []bool{true, true} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := Complex64Bool.FromSlice(slice[1:])
		s4 := Complex64Bool.FromSlice(slice)
		Complex64Bool.Union(s3, s4)
		for i, want := range []bool{true, true} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := Complex64Bool.FromSlice(slice[1:])
		s6 := Complex64Bool.FromSlice(slice[:1])
		Complex64Bool.Union(s5, s6)
		for i, want := range []bool{true, true} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := Complex64Bool.FromSlice(slice)
		s8 := Complex64Bool.FromSlice(slice)
		Complex64Bool.Union(s7, s8)
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
