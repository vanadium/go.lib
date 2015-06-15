// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package set implements utility functions for manipulating sets of
// primitive type elements represented as maps.
//
// For each primitive type "foo", the package provides global
// variables Foo and FooBool that implement utility functions for
// map[foo]struct{} and map[foo]bool respectively. For each such
// variable, the package provides:
//
//   1) methods for conversion between sets represented as maps and
//      slices: FromSlice(slice) and ToSlice(set)
//
//   2) methods for common set operations: Difference(s1, s2),
//      Intersection(s1, s2), and Union(s1, s2); note that these
//      functions store their result in the first argument
//
// For instance, one can use these functions as follows:
//
//   s1 := set.String.FromSlice([]string{"a", "b"})
//   s2 := set.String.FromSlice([]string{"b", "c"})
//
//   set.String.Difference(s1, s2)   // s1 == {"a"}
//   set.String.Intersection(s1, s2) // s1 == {}
//   set.String.Union(s1, s2)        // s1 == {"b", "c"}
package set

//go:generate go run ./gen.go
