// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package set

var Float32 = Float32T{}

type Float32T struct{}

// FromSlice transforms the given slice to a set.
func (Float32T) FromSlice(els []float32) map[float32]struct{} {
	if len(els) == 0 {
		return nil
	}
	result := map[float32]struct{}{}
	for _, el := range els {
		result[el] = struct{}{}
	}
	return result
}

// ToSlice transforms the given set to a slice.
func (Float32T) ToSlice(s map[float32]struct{}) []float32 {
	var result []float32
	for el, _ := range s {
		result = append(result, el)
	}
	return result
}

// Difference subtracts s2 from s1, storing the result in s1.
func (Float32T) Difference(s1, s2 map[float32]struct{}) {
	for el, _ := range s1 {
		if _, ok := s2[el]; ok {
			delete(s1, el)
		}
	}
}

// Intersection intersects s1 and s2, storing the result in s1.
func (Float32T) Intersection(s1, s2 map[float32]struct{}) {
	for el, _ := range s1 {
		if _, ok := s2[el]; !ok {
			delete(s1, el)
		}
	}
}

// Union merges s1 and s2, storing the result in s1.
func (Float32T) Union(s1, s2 map[float32]struct{}) {
	for el, _ := range s2 {
		s1[el] = struct{}{}
	}
}
