// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package set

var Int64 = Int64T{}

type Int64T struct{}

// FromSlice transforms the given slice to a set.
func (Int64T) FromSlice(els []int64) map[int64]struct{} {
	if len(els) == 0 {
		return nil
	}
	result := map[int64]struct{}{}
	for _, el := range els {
		result[el] = struct{}{}
	}
	return result
}

// ToSlice transforms the given set to a slice.
func (Int64T) ToSlice(s map[int64]struct{}) []int64 {
	var result []int64
	for el := range s {
		result = append(result, el)
	}
	return result
}

// Difference subtracts s2 from s1, storing the result in s1.
func (Int64T) Difference(s1, s2 map[int64]struct{}) {
	for el := range s1 {
		if _, ok := s2[el]; ok {
			delete(s1, el)
		}
	}
}

// Intersection intersects s1 and s2, storing the result in s1.
func (Int64T) Intersection(s1, s2 map[int64]struct{}) {
	for el := range s1 {
		if _, ok := s2[el]; !ok {
			delete(s1, el)
		}
	}
}

// Union merges s1 and s2, storing the result in s1.
func (Int64T) Union(s1, s2 map[int64]struct{}) {
	for el := range s2 {
		s1[el] = struct{}{}
	}
}
