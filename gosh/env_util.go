// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosh

import (
	"sort"
	"strings"
)

func splitKeyValue(kv string) (string, string) {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		panic(kv)
	}
	return parts[0], parts[1]
}

func joinKeyValue(k, v string) string {
	return k + "=" + v
}

func sortByKey(vars []string) {
	sort.Sort(keySorter(vars))
}

type keySorter []string

func (s keySorter) Len() int      { return len(s) }
func (s keySorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s keySorter) Less(i, j int) bool {
	ki, _ := splitKeyValue(s[i])
	kj, _ := splitKeyValue(s[j])
	return ki < kj
}

// sliceToMap converts a slice of "key=value" entries to a map, preferring later
// values over earlier ones.
func sliceToMap(s []string) map[string]string {
	m := make(map[string]string, len(s))
	for _, kv := range s {
		k, v := splitKeyValue(kv)
		m[k] = v
	}
	return m
}

// mapToSlice converts a map to a slice of "key=value" entries, sorted by key.
func mapToSlice(m map[string]string) []string {
	s := make([]string, 0, len(m))
	for k, v := range m {
		s = append(s, joinKeyValue(k, v))
	}
	sortByKey(s)
	return s
}

// mergeMaps merges the given maps into a new map, preferring values from later
// maps over those from earlier maps.
func mergeMaps(maps ...map[string]string) map[string]string {
	res := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}

// copyMap returns a copy of the given map.
func copyMap(m map[string]string) map[string]string {
	return mergeMaps(m)
}
