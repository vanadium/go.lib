// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package set

var UintBool = UintBoolT{}

type UintBoolT struct{}

// FromSlice transforms the given slice to a set.
func (UintBoolT) FromSlice(els []uint) map[uint]bool {
	if len(els) == 0 {
		return nil
	}
	result := map[uint]bool{}
	for _, el := range els {
		result[el] = true
	}
	return result
}

// ToSlice transforms the given set to a slice.
func (UintBoolT) ToSlice(s map[uint]bool) []uint {
	var result []uint
	for el := range s {
		result = append(result, el)
	}
	return result
}

// Difference subtracts s2 from s1, storing the result in s1.
func (UintBoolT) Difference(s1, s2 map[uint]bool) {
	for el := range s1 {
		if _, ok := s2[el]; ok {
			delete(s1, el)
		}
	}
}

// Intersection intersects s1 and s2, storing the result in s1.
func (UintBoolT) Intersection(s1, s2 map[uint]bool) {
	for el := range s1 {
		if _, ok := s2[el]; !ok {
			delete(s1, el)
		}
	}
}

// Union merges s1 and s2, storing the result in s1.
func (UintBoolT) Union(s1, s2 map[uint]bool) {
	for el := range s2 {
		s1[el] = true
	}
}
