// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package set

var String = StringT{}

type StringT struct{}

// FromSlice transforms the given slice to a set.
func (StringT) FromSlice(els []string) map[string]struct{} {
	if len(els) == 0 {
		return nil
	}
	result := map[string]struct{}{}
	for _, el := range els {
		result[el] = struct{}{}
	}
	return result
}

// ToSlice transforms the given set to a slice.
func (StringT) ToSlice(s map[string]struct{}) []string {
	var result []string
	for el, _ := range s {
		result = append(result, el)
	}
	return result
}

// Difference subtracts s2 from s1, storing the result in s1.
func (StringT) Difference(s1, s2 map[string]struct{}) {
	for el, _ := range s1 {
		if _, ok := s2[el]; ok {
			delete(s1, el)
		}
	}
}

// Intersection intersects s1 and s2, storing the result in s1.
func (StringT) Intersection(s1, s2 map[string]struct{}) {
	for el, _ := range s1 {
		if _, ok := s2[el]; !ok {
			delete(s1, el)
		}
	}
}

// Union merges s1 and s2, storing the result in s1.
func (StringT) Union(s1, s2 map[string]struct{}) {
	for el, _ := range s2 {
		s1[el] = struct{}{}
	}
}
