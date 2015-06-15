// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package set

import (
	"testing"
)

func TestUint8(t *testing.T) {
	slice := []uint8{}

	slice = append(slice, 0)
	slice = append(slice, 1)

	// Test conversion from a slice.
	s1 := Uint8.FromSlice(slice)
	for i, want := range []bool{true, true} {
		if _, got := s1[slice[i]]; got != want {
			t.Errorf("index %d: got %v, want %v", i, got, want)
		}
	}
	if got, want := len(s1), len(slice); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Test conversion to a slice.
	slice2 := Uint8.ToSlice(s1)
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
		s1 := Uint8.FromSlice(slice)
		s2 := Uint8.FromSlice(slice[1:])
		Uint8.Difference(s1, s2)
		for i, want := range []bool{true, false} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := Uint8.FromSlice(slice[1:])
		s4 := Uint8.FromSlice(slice)
		Uint8.Difference(s3, s4)
		for i, want := range []bool{false, false} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := Uint8.FromSlice(slice[1:])
		s6 := Uint8.FromSlice(slice[:1])
		Uint8.Difference(s5, s6)
		for i, want := range []bool{false, true} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := Uint8.FromSlice(slice)
		s8 := Uint8.FromSlice(slice)
		Uint8.Difference(s7, s8)
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
		s1 := Uint8.FromSlice(slice)
		s2 := Uint8.FromSlice(slice[1:])
		Uint8.Intersection(s1, s2)
		for i, want := range []bool{false, true} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := Uint8.FromSlice(slice[1:])
		s4 := Uint8.FromSlice(slice)
		Uint8.Intersection(s3, s4)
		for i, want := range []bool{false, true} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := Uint8.FromSlice(slice[1:])
		s6 := Uint8.FromSlice(slice[:1])
		Uint8.Intersection(s5, s6)
		for i, want := range []bool{false, false} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := Uint8.FromSlice(slice)
		s8 := Uint8.FromSlice(slice)
		Uint8.Intersection(s7, s8)
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
		s1 := Uint8.FromSlice(slice[:1])
		s2 := Uint8.FromSlice(slice[1:])
		Uint8.Union(s1, s2)
		for i, want := range []bool{true, true} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := Uint8.FromSlice(slice[1:])
		s4 := Uint8.FromSlice(slice)
		Uint8.Union(s3, s4)
		for i, want := range []bool{true, true} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := Uint8.FromSlice(slice[1:])
		s6 := Uint8.FromSlice(slice[:1])
		Uint8.Union(s5, s6)
		for i, want := range []bool{true, true} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := Uint8.FromSlice(slice)
		s8 := Uint8.FromSlice(slice)
		Uint8.Union(s7, s8)
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
