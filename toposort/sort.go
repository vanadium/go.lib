// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package toposort implements topological sort.  For details see:
// http://en.wikipedia.org/wiki/Topological_sorting
package toposort

// Sorter implements a topological sorter.  Add nodes and edges to the sorter to
// describe the graph, and call Sort to retrieve topologically-sorted nodes.
// The zero Sorter describes an empty graph.
type Sorter struct {
	values map[interface{}]int // maps from user-provided value to index in nodes
	nodes  []*node             // the graph to sort
}

// node represents a node in the graph.
type node struct {
	value    interface{}
	children []*node
}

func (s *Sorter) getOrAddNode(value interface{}) *node {
	if s.values == nil {
		s.values = make(map[interface{}]int)
	}
	if index, ok := s.values[value]; ok {
		return s.nodes[index]
	}
	s.values[value] = len(s.nodes)
	newNode := &node{value: value}
	s.nodes = append(s.nodes, newNode)
	return newNode
}

// AddNode adds a node.  Arbitrary value types are supported, but the values
// must be comparable; they'll be used as map keys.  Typically this is only used
// to add nodes with no incoming or outgoing edges.
func (s *Sorter) AddNode(value interface{}) {
	s.getOrAddNode(value)
}

// AddEdge adds nodes from and to, and adds an edge from -> to.  You don't need
// to call AddNode first; the nodes will be implicitly added if they don't
// already exist.  The direction means that from depends on to; i.e. to will
// appear before from in the sorted output.  Cycles are allowed.
func (s *Sorter) AddEdge(from interface{}, to interface{}) {
	fromN, toN := s.getOrAddNode(from), s.getOrAddNode(to)
	fromN.children = append(fromN.children, toN)
}

// Sort returns the topologically sorted nodes, along with some of the cycles
// (if any) that were encountered.  You're guaranteed that len(cycles)==0 iff
// there are no cycles in the graph, otherwise an arbitrary (but non-empty) list
// of cycles is returned.
//
// If there are cycles the sorting is best-effort; portions of the graph that
// are acyclic will still be ordered correctly, and the cyclic portions have an
// arbitrary ordering.
//
// Sort is deterministic; given the same sequence of inputs it always returns
// the same output, even if the inputs are only partially ordered.
func (s *Sorter) Sort() (sorted []interface{}, cycles [][]interface{}) {
	// The strategy is the standard simple approach of performing DFS on the
	// graph.  Details are outlined in the above wikipedia article.
	done := make(map[*node]bool)
	for _, n := range s.nodes {
		cycles = appendCycles(cycles, n.visit(done, make(map[*node]bool), &sorted))
	}
	return
}

// visit performs DFS on the graph, and fills in sorted and cycles as it
// traverses.  We use done to indicate a node has been fully explored, and
// visiting to indicate a node is currently being explored.
//
// The cycle collection strategy is to wait until we've hit a repeated node in
// visiting, and add that node to cycles and return.  Thereafter as the
// recursive stack is unwound, nodes append themselves to the end of each cycle,
// until we're back at the repeated node.  This guarantees that if the graph is
// cyclic we'll return at least one of the cycles.
func (n *node) visit(done, visiting map[*node]bool, sorted *[]interface{}) (cycles [][]interface{}) {
	if done[n] {
		return
	}
	if visiting[n] {
		cycles = [][]interface{}{{n.value}}
		return
	}
	visiting[n] = true
	for _, child := range n.children {
		cycles = appendCycles(cycles, child.visit(done, visiting, sorted))
	}
	done[n] = true
	*sorted = append(*sorted, n.value)
	// Update cycles.  If it's empty none of our children detected any cycles, and
	// there's nothing to do.  Otherwise we append ourselves to the cycle, iff the
	// cycle hasn't completed yet.  We know the cycle has completed if the first
	// and last item in the cycle are the same, with an exception for the single
	// item case; self-cycles are represented as the same node appearing twice.
	for cx := range cycles {
		len := len(cycles[cx])
		if len == 1 || cycles[cx][0] != cycles[cx][len-1] {
			cycles[cx] = append(cycles[cx], n.value)
		}
	}
	return
}

// appendCycles returns the combined cycles in a and b.
func appendCycles(a [][]interface{}, b [][]interface{}) [][]interface{} {
	for _, bcycle := range b {
		a = append(a, bcycle)
	}
	return a
}

// DumpCycles dumps the cycles returned from Sorter.Sort, using toString to
// convert each node into a string.
func DumpCycles(cycles [][]interface{}, toString func(n interface{}) string) string {
	var str string
	for cyclex, cycle := range cycles {
		if cyclex > 0 {
			str += " "
		}
		str += "["
		for nodex, node := range cycle {
			if nodex > 0 {
				str += " <= "
			}
			str += toString(node)
		}
		str += "]"
	}
	return str
}
