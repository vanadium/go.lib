// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package toposort

import (
	"reflect"
	"testing"
)

func toStringSlice(input []interface{}) (output []string) {
	output = make([]string, len(input))
	for ix, ival := range input {
		output[ix] = ival.(string)
	}
	return
}

func toStringCycles(input [][]interface{}) (output [][]string) {
	output = make([][]string, len(input))
	for ix, islice := range input {
		output[ix] = toStringSlice(islice)
	}
	return
}

type orderChecker struct {
	t        *testing.T
	original []string
	orderMap map[string]int
}

func makeOrderChecker(t *testing.T, slice []interface{}) orderChecker {
	result := orderChecker{t, toStringSlice(slice), make(map[string]int)}
	for ix, val := range result.original {
		result.orderMap[val] = ix
	}
	return result
}

func (oc *orderChecker) findValue(val string) int {
	if index, ok := oc.orderMap[val]; ok {
		return index
	}
	oc.t.Errorf("Couldn't find val %v in slice %v", val, oc.original)
	return -1
}

func (oc *orderChecker) expectOrder(before, after string) {
	if oc.findValue(before) >= oc.findValue(after) {
		oc.t.Errorf("Expected %v before %v, slice %v", before, after, oc.original)
	}
}

// Since sort is deterministic we can expect a particular total order, in
// addition to the partial order checks.
func (oc *orderChecker) expectTotalOrder(expect ...string) {
	if !reflect.DeepEqual(oc.original, expect) {
		oc.t.Errorf("Expected order %v, actual %v", expect, oc.original)
	}
}

func expectCycles(t *testing.T, actual [][]interface{}, expect [][]string) {
	actualStr := toStringCycles(actual)
	if !reflect.DeepEqual(actualStr, expect) {
		t.Errorf("Expected cycles %v, actual %v", expect, actualStr)
	}
}

func TestSortDag(t *testing.T) {
	// This is the graph:
	// ,-->B
	// |
	// A-->C---->D
	// |    \
	// |     `-->E--.
	// `-------------`-->F
	var sorter Sorter
	sorter.AddEdge("A", "B")
	sorter.AddEdge("A", "C")
	sorter.AddEdge("A", "F")
	sorter.AddEdge("C", "D")
	sorter.AddEdge("C", "E")
	sorter.AddEdge("E", "F")
	sorted, cycles := sorter.Sort()
	oc := makeOrderChecker(t, sorted)
	oc.expectOrder("B", "A")
	oc.expectOrder("C", "A")
	oc.expectOrder("D", "A")
	oc.expectOrder("E", "A")
	oc.expectOrder("F", "A")
	oc.expectOrder("D", "C")
	oc.expectOrder("E", "C")
	oc.expectOrder("F", "C")
	oc.expectOrder("F", "E")
	oc.expectTotalOrder("B", "D", "F", "E", "C", "A")
	expectCycles(t, cycles, [][]string{})
}

func TestSortSelfCycle(t *testing.T) {
	// This is the graph:
	// ,---.
	// |   |
	// A<--'
	var sorter Sorter
	sorter.AddEdge("A", "A")
	sorted, cycles := sorter.Sort()
	oc := makeOrderChecker(t, sorted)
	oc.expectTotalOrder("A")
	expectCycles(t, cycles, [][]string{{"A", "A"}})
}

func TestSortCycle(t *testing.T) {
	// This is the graph:
	// ,-->B-->C
	// |       |
	// A<------'
	var sorter Sorter
	sorter.AddEdge("A", "B")
	sorter.AddEdge("B", "C")
	sorter.AddEdge("C", "A")
	sorted, cycles := sorter.Sort()
	oc := makeOrderChecker(t, sorted)
	oc.expectTotalOrder("C", "B", "A")
	expectCycles(t, cycles, [][]string{{"A", "C", "B", "A"}})
}

func TestSortContainsCycle1(t *testing.T) {
	// This is the graph:
	// ,-->B
	// |   ,-----.
	// |   v     |
	// A-->C---->D
	// |    \
	// |     `-->E--.
	// `-------------`-->F
	var sorter Sorter
	sorter.AddEdge("A", "B")
	sorter.AddEdge("A", "C")
	sorter.AddEdge("A", "F")
	sorter.AddEdge("C", "D")
	sorter.AddEdge("C", "E")
	sorter.AddEdge("D", "C") // creates the cycle
	sorter.AddEdge("E", "F")
	sorted, cycles := sorter.Sort()
	oc := makeOrderChecker(t, sorted)
	oc.expectOrder("B", "A")
	oc.expectOrder("C", "A")
	oc.expectOrder("D", "A")
	oc.expectOrder("E", "A")
	oc.expectOrder("F", "A")
	// The difference with the dag is C, D may be in either order.
	oc.expectOrder("E", "C")
	oc.expectOrder("F", "C")
	oc.expectOrder("F", "E")
	oc.expectTotalOrder("B", "D", "F", "E", "C", "A")
	expectCycles(t, cycles, [][]string{{"C", "D", "C"}})
}

func TestSortContainsCycle2(t *testing.T) {
	// This is the graph:
	// ,-->B
	// |   ,-------------.
	// |   v             |
	// A-->C---->D       |
	// |    \            |
	// |     `-->E--.    |
	// `-------------`-->F
	var sorter Sorter
	sorter.AddEdge("A", "B")
	sorter.AddEdge("A", "C")
	sorter.AddEdge("A", "F")
	sorter.AddEdge("C", "D")
	sorter.AddEdge("C", "E")
	sorter.AddEdge("E", "F")
	sorter.AddEdge("F", "C") // creates the cycle
	sorted, cycles := sorter.Sort()
	oc := makeOrderChecker(t, sorted)
	oc.expectOrder("B", "A")
	oc.expectOrder("C", "A")
	oc.expectOrder("D", "A")
	oc.expectOrder("E", "A")
	oc.expectOrder("F", "A")
	oc.expectOrder("D", "C")
	// The difference with the dag is C, E, F may be in any order.
	oc.expectTotalOrder("B", "D", "F", "E", "C", "A")
	expectCycles(t, cycles, [][]string{{"C", "F", "E", "C"}})
}

func TestSortMultiCycles(t *testing.T) {
	// This is the graph:
	//    ,-->B
	//    |   ,------------.
	//    |   v            |
	// .--A-->C---->D      |
	// |  ^    \           |
	// |  |     `-->E--.   |
	// |  |         |  |   |
	// |  `---------'  |   |
	// `---------------`-->F
	var sorter Sorter
	sorter.AddEdge("A", "B")
	sorter.AddEdge("A", "C")
	sorter.AddEdge("A", "F")
	sorter.AddEdge("C", "D")
	sorter.AddEdge("C", "E")
	sorter.AddEdge("E", "A") // creates a cycle
	sorter.AddEdge("E", "F")
	sorter.AddEdge("F", "C") // creates a cycle
	sorted, cycles := sorter.Sort()
	oc := makeOrderChecker(t, sorted)
	oc.expectOrder("B", "A")
	oc.expectOrder("D", "A")
	oc.expectOrder("F", "A")
	oc.expectOrder("D", "C")
	oc.expectTotalOrder("B", "D", "F", "E", "C", "A")
	expectCycles(t, cycles, [][]string{{"A", "E", "C", "A"}, {"C", "F", "E", "C"}})
}
