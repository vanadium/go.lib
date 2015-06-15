// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package set

var Int16Bool = Int16BoolT{}

type Int16BoolT struct{}

// FromSlice transforms the given slice to a set.
func (Int16BoolT) FromSlice(els []int16) map[int16]bool {
	if len(els) == 0 {
		return nil
	}
	result := map[int16]bool{}
	for _, el := range els {
		result[el] = true
	}
	return result
}

// ToSlice transforms the given set to a slice.
func (Int16BoolT) ToSlice(s map[int16]bool) []int16 {
	var result []int16
	for el, _ := range s {
		result = append(result, el)
	}
	return result
}

// Difference subtracts s2 from s1, storing the result in s1.
func (Int16BoolT) Difference(s1, s2 map[int16]bool) {
	for el, _ := range s1 {
		if _, ok := s2[el]; ok {
			delete(s1, el)
		}
	}
}

// Intersection intersects s1 and s2, storing the result in s1.
func (Int16BoolT) Intersection(s1, s2 map[int16]bool) {
	for el, _ := range s1 {
		if _, ok := s2[el]; !ok {
			delete(s1, el)
		}
	}
}

// Union merges s1 and s2, storing the result in s1.
func (Int16BoolT) Union(s1, s2 map[int16]bool) {
	for el, _ := range s2 {
		s1[el] = true
	}
}
