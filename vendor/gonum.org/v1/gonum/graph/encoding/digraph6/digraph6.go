// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package digraph6 implements graphs specified by digraph6 strings.
package digraph6 // import "gonum.org/v1/gonum/graph/encoding/digraph6"

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

// Graph is a digraph6-represented directed graph.
//
// See https://users.cecs.anu.edu.au/~bdm/data/formats.txt for details.
//
// Note that the digraph6 format specifies that the first character of the graph
// string is a '&'. This character must be present for use in the digraph6 package.
// A Graph without this prefix is treated as the null graph.
type Graph string

var (
	d6 Graph

	_ graph.Graph    = d6
	_ graph.Directed = d6
)

// Encode returns a graph6 encoding of the topology of the given graph using a
// lexical ordering of the nodes by ID to map them to [0, n).
func Encode(g graph.Graph) Graph {
	nodes := graph.NodesOf(g.Nodes())
	n := len(nodes)
	sort.Sort(ordered.ByID(nodes))
	indexOf := make(map[int64]int, n)
	for i, n := range nodes {
		indexOf[n.ID()] = i
	}

	size := n * n
	var b big.Int
	for i, u := range nodes {
		it := g.From(u.ID())
		for it.Next() {
			vid := it.Node().ID()
			j := indexOf[vid]
			b.SetBit(&b, bitFor(int64(i), int64(j), int64(n)), 1)
		}
	}

	var buf strings.Builder
	buf.WriteByte('&')
	switch {
	case n < 63:
		buf.WriteByte(byte(n) + 63)
	case n < 258048:
		buf.Write([]byte{126, byte(n>>12) + 63, byte(n>>6) + 63, byte(n) + 63})
	case n < 68719476736:
		buf.Write([]byte{126, 126, byte(n>>30) + 63, byte(n>>24) + 63, byte(n>>18) + 63, byte(n>>12) + 63, byte(n>>6) + 63, byte(n) + 63})
	default:
		panic("digraph6: too large")
	}

	var c byte
	for i := 0; i < size; i++ {
		bit := i % 6
		c |= byte(b.Bit(i)) << uint(5-bit)
		if bit == 5 {
			buf.WriteByte(c + 63)
			c = 0
		}
	}
	if size%6 != 0 {
		buf.WriteByte(c + 63)
	}

	return Graph(buf.String())
}

// IsValid returns whether the graph is a valid digraph6 encoding. An invalid Graph
// behaves as the null graph.
func IsValid(g Graph) bool {
	n := int(numberOf(g))
	if n < 0 {
		return false
	}
	size := (n*n + 5) / 6 // ceil(n^2 / 6)
	g = g[1:]
	switch {
	case g[0] != 126:
		return len(g[1:]) == size
	case g[1] != 126:
		return len(g[4:]) == size
	default:
		return len(g[8:]) == size
	}
}

// Edge returns the edge from u to v, with IDs uid and vid, if such an edge
// exists and nil otherwise. The node v must be directly reachable from u as
// defined by the From method.
func (g Graph) Edge(uid, vid int64) graph.Edge {
	if !IsValid(g) {
		return nil
	}
	if !g.HasEdgeFromTo(uid, vid) {
		return nil
	}
	return simple.Edge{simple.Node(uid), simple.Node(vid)}
}

// From returns all nodes that can be reached directly from the node with the
// given ID.
func (g Graph) From(id int64) graph.Nodes {
	if !IsValid(g) {
		return graph.Empty
	}
	if g.Node(id) == nil {
		return nil
	}
	return &d6ForwardIterator{g: g, from: id, to: -1}
}

// HasEdgeBetween returns whether an edge exists between nodes with IDs xid
// and yid without considering direction.
func (g Graph) HasEdgeBetween(xid, yid int64) bool {
	if !IsValid(g) {
		return false
	}
	return g.HasEdgeFromTo(xid, yid) || g.HasEdgeFromTo(yid, xid)
}

// HasEdgeFromTo returns whether an edge exists in the graph from u to v with
// IDs uid and vid.
func (g Graph) HasEdgeFromTo(uid, vid int64) bool {
	if !IsValid(g) {
		return false
	}
	if uid == vid {
		return false
	}
	n := numberOf(g)
	if uid < 0 || n <= uid {
		return false
	}
	if vid < 0 || n <= vid {
		return false
	}
	return isSet(bitFor(uid, vid, n), g)
}

// Node returns the node with the given ID if it exists in the graph, and nil
// otherwise.
func (g Graph) Node(id int64) graph.Node {
	if !IsValid(g) {
		return nil
	}
	if id < 0 || numberOf(g) <= id {
		return nil
	}
	return simple.Node(id)
}

// Nodes returns all the nodes in the graph.
func (g Graph) Nodes() graph.Nodes {
	if !IsValid(g) {
		return graph.Empty
	}
	return iterator.NewImplicitNodes(0, int(numberOf(g)), func(id int) graph.Node { return simple.Node(id) })
}

// To returns all nodes that can reach directly to the node with the given ID.
func (g Graph) To(id int64) graph.Nodes {
	if !IsValid(g) || g.Node(id) == nil {
		return graph.Empty
	}
	return &d6ReverseIterator{g: g, from: -1, to: id}
}

// d6ForwardIterator is a graph.Nodes for digraph6 graph edges for forward hops.
type d6ForwardIterator struct {
	g    Graph
	from int64
	to   int64
}

var _ graph.Nodes = (*d6ForwardIterator)(nil)

func (i *d6ForwardIterator) Next() bool {
	n := numberOf(i.g)
	for i.to < n-1 {
		i.to++
		if i.to != i.from && isSet(bitFor(i.from, i.to, n), i.g) {
			return true
		}
	}
	return false
}

func (i *d6ForwardIterator) Len() int {
	var cnt int
	n := numberOf(i.g)
	for to := i.to; to < n-1; {
		to++
		if to != i.from && isSet(bitFor(i.from, to, n), i.g) {
			cnt++
		}
	}
	return cnt
}

func (i *d6ForwardIterator) Reset() { i.to = -1 }

func (i *d6ForwardIterator) Node() graph.Node { return simple.Node(i.to) }

// d6ReverseIterator is a graph.Nodes for digraph6 graph edges for reverse hops.
type d6ReverseIterator struct {
	g    Graph
	from int64
	to   int64
}

var _ graph.Nodes = (*d6ReverseIterator)(nil)

func (i *d6ReverseIterator) Next() bool {
	n := numberOf(i.g)
	for i.from < n-1 {
		i.from++
		if i.to != i.from && isSet(bitFor(i.from, i.to, n), i.g) {
			return true
		}
	}
	return false
}

func (i *d6ReverseIterator) Len() int {
	var cnt int
	n := numberOf(i.g)
	for from := i.from; from < n-1; {
		from++
		if from != i.to && isSet(bitFor(from, i.to, n), i.g) {
			cnt++
		}
	}
	return cnt
}

func (i *d6ReverseIterator) Reset() { i.from = -1 }

func (i *d6ReverseIterator) Node() graph.Node { return simple.Node(i.from) }

// numberOf returns the digraph6-encoded number corresponding to g.
func numberOf(g Graph) int64 {
	if len(g) < 2 {
		return -1
	}
	if g[0] != '&' {
		return -1
	}
	g = g[1:]
	if g[0] != 126 {
		return int64(g[0] - 63)
	}
	if len(g) < 4 {
		return -1
	}
	if g[1] != 126 {
		return int64(g[1]-63)<<12 | int64(g[2]-63)<<6 | int64(g[3]-63)
	}
	if len(g) < 8 {
		return -1
	}
	return int64(g[2]-63)<<30 | int64(g[3]-63)<<24 | int64(g[4]-63)<<18 | int64(g[5]-63)<<12 | int64(g[6]-63)<<6 | int64(g[7]-63)
}

// bitFor returns the index into the digraph6 adjacency matrix for uid->vid in a graph
// order n.
func bitFor(uid, vid, n int64) int {
	return int(uid*n + vid)
}

// isSet returns whether the given bit of the adjacency matrix is set.
func isSet(bit int, g Graph) bool {
	g = g[1:]
	switch {
	case g[0] != 126:
		g = g[1:]
	case g[1] != 126:
		g = g[4:]
	default:
		g = g[8:]
	}
	if bit/6 >= len(g) {
		panic("digraph6: index out of range")
	}
	return (g[bit/6]-63)&(1<<uint(5-bit%6)) != 0
}

func (g Graph) GoString() string {
	if !IsValid(g) {
		return ""
	}
	bin, m6 := binary(g)
	format := fmt.Sprintf("%%d:%%0%db", m6)
	return fmt.Sprintf(format, numberOf(g), bin)
}

func binary(g Graph) (b *big.Int, l int) {
	n := int(numberOf(g))
	g = g[1:]
	switch {
	case g[0] != 126:
		g = g[1:]
	case g[1] != 126:
		g = g[4:]
	default:
		g = g[8:]
	}
	b = &big.Int{}
	var c big.Int
	for i := range g {
		c.SetUint64(uint64(g[len(g)-i-1] - 63))
		c.Lsh(&c, uint(6*i))
		b.Or(b, &c)
	}

	// Truncate to only the relevant parts of the bit vector.
	b.Rsh(b, uint(len(g)*6-(n*n)))

	return b, n * n
}
