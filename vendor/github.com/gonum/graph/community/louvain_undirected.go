// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package community

import (
	"math"
	"math/rand"
	"sort"

	"golang.org/x/tools/container/intsets"

	"github.com/gonum/graph"
	"github.com/gonum/graph/internal/ordered"
)

// qUndirected returns the modularity Q score of the graph g subdivided into the
// given communities at the given resolution. If communities is nil, the
// unclustered modularity score is returned. The resolution parameter
// is γ as defined in Reichardt and Bornholdt doi:10.1103/PhysRevE.74.016110.
// qUndirected will panic if g has any edge with negative edge weight.
//
//  Q = 1/2m \sum_{ij} [ A_{ij} - (\gamma k_i k_j)/2m ] \delta(c_i,c_j)
//
// graph.Undirect may be used as a shim to allow calculation of Q for
// directed graphs.
func qUndirected(g graph.Undirected, communities [][]graph.Node, resolution float64) float64 {
	nodes := g.Nodes()
	weight := positiveWeightFuncFor(g)

	// Calculate the total edge weight of the graph
	// and the table of penetrating edge weight sums.
	var m2 float64
	k := make(map[int]float64, len(nodes))
	for _, u := range nodes {
		w := weight(u, u)
		for _, v := range g.From(u) {
			w += weight(u, v)
		}
		m2 += w
		k[u.ID()] = w
	}

	if communities == nil {
		var q float64
		for _, u := range nodes {
			kU := k[u.ID()]
			q += weight(u, u) - resolution*kU*kU/m2
		}
		return q / m2
	}

	// Iterate over the communities, calculating
	// the non-self edge weights for the upper
	// triangle and adjust the diagonal.
	var q float64
	for _, c := range communities {
		for i, u := range c {
			kU := k[u.ID()]
			q += weight(u, u) - resolution*kU*kU/m2
			for _, v := range c[i+1:] {
				q += 2 * (weight(u, v) - resolution*kU*k[v.ID()]/m2)
			}
		}
	}
	return q / m2
}

// louvainUndirected returns the hierarchical modularization of g at the given
// resolution using the Louvain algorithm. If src is nil, rand.Intn is used as
// the random generator. louvainUndirected will panic if g has any edge with negative edge
// weight.
//
// graph.Undirect may be used as a shim to allow modularization of directed graphs.
func louvainUndirected(g graph.Undirected, resolution float64, src *rand.Rand) *ReducedUndirected {
	// See louvain.tex for a detailed description
	// of the algorithm used here.

	c := reduceUndirected(g, nil)
	rnd := rand.Intn
	if src != nil {
		rnd = src.Intn
	}
	for {
		l := newUndirectedLocalMover(c, c.communities, resolution)
		if l == nil {
			return c
		}
		if done := l.localMovingHeuristic(rnd); done {
			return c
		}
		c = reduceUndirected(c, l.communities)
	}
}

// ReducedUndirected is an undirected graph of communities derived from a
// parent graph by reduction.
type ReducedUndirected struct {
	// nodes is the set of nodes held
	// by the graph. In a ReducedUndirected
	// the node ID is the index into
	// nodes.
	nodes []community
	undirectedEdges

	// communities is the community
	// structure of the graph.
	communities [][]graph.Node

	parent *ReducedUndirected
}

var (
	_ graph.Undirected = (*ReducedUndirected)(nil)
	_ graph.Weighter   = (*ReducedUndirected)(nil)
	_ ReducedGraph     = (*ReducedUndirected)(nil)
)

// Communities returns the community memberships of the nodes in the
// graph used to generate the reduced graph.
func (g *ReducedUndirected) Communities() [][]graph.Node {
	communities := make([][]graph.Node, len(g.communities))
	if g.parent == nil {
		for i, members := range g.communities {
			comm := make([]graph.Node, len(members))
			for j, n := range members {
				nodes := g.nodes[n.ID()].nodes
				if len(nodes) != 1 {
					panic("community: unexpected number of nodes in base graph community")
				}
				comm[j] = nodes[0]
			}
			communities[i] = comm
		}
		return communities
	}
	sub := g.parent.Communities()
	for i, members := range g.communities {
		var comm []graph.Node
		for _, n := range members {
			comm = append(comm, sub[n.ID()]...)
		}
		communities[i] = comm
	}
	return communities
}

// Structure returns the community structure of the current level of
// the module clustering. The first index of the returned value
// corresponds to the index of the nodes in the next higher level if
// it exists. The returned value should not be mutated.
func (g *ReducedUndirected) Structure() [][]graph.Node {
	return g.communities
}

// Expanded returns the next lower level of the module clustering or nil
// if at the lowest level.
func (g *ReducedUndirected) Expanded() ReducedGraph {
	return g.parent
}

// reduceUndirected returns a reduced graph constructed from g divided
// into the given communities. The communities value is mutated
// by the call to reduceUndirected. If communities is nil and g is a
// ReducedUndirected, it is returned unaltered.
func reduceUndirected(g graph.Undirected, communities [][]graph.Node) *ReducedUndirected {
	if communities == nil {
		if r, ok := g.(*ReducedUndirected); ok {
			return r
		}

		nodes := g.Nodes()
		// TODO(kortschak) This sort is necessary really only
		// for testing. In practice we would not be using the
		// community provided by the user for a Q calculation.
		// Probably we should use a function to map the
		// communities in the test sets to the remapped order.
		sort.Sort(ordered.ByID(nodes))
		communities = make([][]graph.Node, len(nodes))
		for i := range nodes {
			communities[i] = []graph.Node{node(i)}
		}

		weight := positiveWeightFuncFor(g)
		r := ReducedUndirected{
			nodes: make([]community, len(nodes)),
			undirectedEdges: undirectedEdges{
				edges:   make([][]int, len(nodes)),
				weights: make(map[[2]int]float64),
			},
			communities: communities,
		}
		communityOf := make(map[int]int, len(nodes))
		for i, n := range nodes {
			r.nodes[i] = community{id: i, nodes: []graph.Node{n}}
			communityOf[n.ID()] = i
		}
		for _, u := range nodes {
			var out []int
			uid := communityOf[u.ID()]
			for _, v := range g.From(u) {
				vid := communityOf[v.ID()]
				if vid != uid {
					out = append(out, vid)
				}
				if uid < vid {
					// Only store the weight once.
					r.weights[[2]int{uid, vid}] = weight(u, v)
				}
			}
			r.edges[uid] = out
		}
		return &r
	}

	// Remove zero length communities destructively.
	var commNodes int
	for i := 0; i < len(communities); {
		comm := communities[i]
		if len(comm) == 0 {
			communities[i] = communities[len(communities)-1]
			communities[len(communities)-1] = nil
			communities = communities[:len(communities)-1]
		} else {
			commNodes += len(comm)
			i++
		}
	}

	r := ReducedUndirected{
		nodes: make([]community, len(communities)),
		undirectedEdges: undirectedEdges{
			edges:   make([][]int, len(communities)),
			weights: make(map[[2]int]float64),
		},
	}
	r.communities = make([][]graph.Node, len(communities))
	for i := range r.communities {
		r.communities[i] = []graph.Node{node(i)}
	}
	if g, ok := g.(*ReducedUndirected); ok {
		// Make sure we retain the truncated
		// community structure.
		g.communities = communities
		r.parent = g
	}
	weight := positiveWeightFuncFor(g)
	communityOf := make(map[int]int, commNodes)
	for i, comm := range communities {
		r.nodes[i] = community{id: i, nodes: comm}
		for _, n := range comm {
			communityOf[n.ID()] = i
		}
	}
	for uid, comm := range communities {
		var out []int
		for i, u := range comm {
			r.nodes[uid].weight += weight(u, u)
			for _, v := range comm[i+1:] {
				r.nodes[uid].weight += 2 * weight(u, v)
			}
			for _, v := range g.From(u) {
				vid := communityOf[v.ID()]
				found := false
				for _, e := range out {
					if e == vid {
						found = true
						break
					}
				}
				if !found && vid != uid {
					out = append(out, vid)
				}
				if uid < vid {
					// Only store the weight once.
					r.weights[[2]int{uid, vid}] += weight(u, v)
				}
			}
		}
		r.edges[uid] = out
	}
	return &r
}

// Has returns whether the node exists within the graph.
func (g *ReducedUndirected) Has(n graph.Node) bool {
	id := n.ID()
	return id >= 0 || id < len(g.nodes)
}

// Nodes returns all the nodes in the graph.
func (g *ReducedUndirected) Nodes() []graph.Node {
	nodes := make([]graph.Node, len(g.nodes))
	for i := range g.nodes {
		nodes[i] = node(i)
	}
	return nodes
}

// From returns all nodes in g that can be reached directly from u.
func (g *ReducedUndirected) From(u graph.Node) []graph.Node {
	out := g.edges[u.ID()]
	nodes := make([]graph.Node, len(out))
	for i, vid := range out {
		nodes[i] = g.nodes[vid]
	}
	return nodes
}

// HasEdgeBetween returns whether an edge exists between nodes x and y.
func (g *ReducedUndirected) HasEdgeBetween(x, y graph.Node) bool {
	xid := x.ID()
	yid := y.ID()
	if xid == yid {
		return false
	}
	if xid > yid {
		xid, yid = yid, xid
	}
	_, ok := g.weights[[2]int{xid, yid}]
	return ok
}

// Edge returns the edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
func (g *ReducedUndirected) Edge(u, v graph.Node) graph.Edge {
	uid := u.ID()
	vid := v.ID()
	if vid < uid {
		uid, vid = vid, uid
	}
	w, ok := g.weights[[2]int{uid, vid}]
	if !ok {
		return nil
	}
	return edge{from: g.nodes[u.ID()], to: g.nodes[v.ID()], weight: w}
}

// EdgeBetween returns the edge between nodes x and y.
func (g *ReducedUndirected) EdgeBetween(x, y graph.Node) graph.Edge {
	return g.Edge(x, y)
}

// Weight returns the weight for the edge between x and y if Edge(x, y) returns a non-nil Edge.
// If x and y are the same node the internal node weight is returned. If there is no joining
// edge between the two nodes the weight value returned is zero. Weight returns true if an edge
// exists between x and y or if x and y have the same ID, false otherwise.
func (g *ReducedUndirected) Weight(x, y graph.Node) (w float64, ok bool) {
	xid := x.ID()
	yid := y.ID()
	if xid == yid {
		return g.nodes[xid].weight, true
	}
	if xid > yid {
		xid, yid = yid, xid
	}
	w, ok = g.weights[[2]int{xid, yid}]
	return w, ok
}

// undirectedLocalMover is a step in graph modularity optimization.
type undirectedLocalMover struct {
	g *ReducedUndirected

	// nodes is the set of working nodes.
	nodes []graph.Node
	// edgeWeightOf is the weighted degree
	// of each node indexed by ID.
	edgeWeightOf []float64

	// m2 is the total sum of
	// edge weights in g.
	m2 float64

	// weight is the weight function
	// provided by g or a function
	// that returns the Weight value
	// of the non-nil edge between x
	// and y.
	weight func(x, y graph.Node) float64

	// communities is the current
	// division of g.
	communities [][]graph.Node
	// memberships is a mapping between
	// node ID and community membership.
	memberships []int

	// resolution is the Reichardt and
	// Bornholdt γ parameter as defined
	// in doi:10.1103/PhysRevE.74.016110.
	resolution float64

	// moved indicates that a call to
	// move has been made since the last
	// call to shuffle.
	moved bool

	// changed indicates that a move
	// has been made since the creation
	// of the local mover.
	changed bool
}

// newUndirectedLocalMover returns a new undirectedLocalMover initialized with
// the graph g, a set of communities and a modularity resolution parameter. The
// node IDs of g must be contiguous in [0,n) where n is the number of nodes.
// If g has a zero edge weight sum, nil is returned.
func newUndirectedLocalMover(g *ReducedUndirected, communities [][]graph.Node, resolution float64) *undirectedLocalMover {
	nodes := g.Nodes()
	l := undirectedLocalMover{
		g:            g,
		nodes:        nodes,
		edgeWeightOf: make([]float64, len(nodes)),
		communities:  communities,
		memberships:  make([]int, len(nodes)),
		resolution:   resolution,
		weight:       positiveWeightFuncFor(g),
	}

	// Calculate the total edge weight of the graph
	// and degree weights for each node.
	for _, u := range l.nodes {
		w := l.weight(u, u)
		for _, v := range g.From(u) {
			w += l.weight(u, v)
		}
		l.edgeWeightOf[u.ID()] = w
		l.m2 += w
	}
	if l.m2 == 0 {
		return nil
	}

	// Assign membership mappings.
	for i, c := range communities {
		for _, u := range c {
			l.memberships[u.ID()] = i
		}
	}

	return &l
}

// localMovingHeuristic performs the Louvain local moving heuristic until
// no further moves can be made. It returns a boolean indicating that the
// undirectedLocalMover has not made any improvement to the community
// structure and so the Louvain algorithm is done.
func (l *undirectedLocalMover) localMovingHeuristic(rnd func(int) int) (done bool) {
	for {
		l.shuffle(rnd)
		for _, n := range l.nodes {
			dQ, dst, src := l.deltaQ(n)
			if dQ <= 0 {
				continue
			}
			l.move(dst, src)
		}
		if !l.moved {
			return !l.changed
		}
	}
}

// shuffle performs a Fisher-Yates shuffle on the nodes held by the
// undirectedLocalMover using the random source rnd which should return
// an integer in the range [0,n).
func (l *undirectedLocalMover) shuffle(rnd func(n int) int) {
	l.moved = false
	for i := range l.nodes[:len(l.nodes)-1] {
		j := i + rnd(len(l.nodes)-i)
		l.nodes[i], l.nodes[j] = l.nodes[j], l.nodes[i]
	}
}

// move moves the node at src to the community at dst.
func (l *undirectedLocalMover) move(dst int, src commIdx) {
	l.moved = true
	l.changed = true

	srcComm := l.communities[src.community]
	n := srcComm[src.node]

	l.memberships[n.ID()] = dst

	l.communities[dst] = append(l.communities[dst], n)
	srcComm[src.node], srcComm[len(srcComm)-1] = srcComm[len(srcComm)-1], nil
	l.communities[src.community] = srcComm[:len(srcComm)-1]
}

// deltaQ returns the highest gain in modularity attainable by moving
// n from its current community to another connected community and
// the index of the chosen destination. The index into the
// undirectedLocalMover's communities field is returned in src if n
// is in communities.
func (l *undirectedLocalMover) deltaQ(n graph.Node) (deltaQ float64, dst int, src commIdx) {
	id := n.ID()
	a_aa := l.weight(n, n)
	k_a := l.edgeWeightOf[id]
	m2 := l.m2
	gamma := l.resolution

	// Find communites connected to n.
	var connected intsets.Sparse
	// The following for loop is equivalent to:
	//
	//  for _, v := range l.g.From(n) {
	//  	connected.Insert(l.memberships[v.ID()])
	//  }
	//
	// This is done to avoid an allocation.
	for _, vid := range l.g.edges[id] {
		connected.Insert(l.memberships[vid])
	}
	// Insert the node's own community.
	connected.Insert(l.memberships[id])

	// Calculate the highest modularity gain
	// from moving into another community and
	// keep the index of that community.
	var dQremove float64
	dQadd, dst, src := math.Inf(-1), -1, commIdx{-1, -1}
	var i int
	for connected.TakeMin(&i) {
		c := l.communities[i]
		var k_aC, sigma_totC float64 // C is a substitution for ^𝛼 or ^𝛽.
		var removal bool
		for j, u := range c {
			uid := u.ID()
			if uid == id {
				if src.community != -1 {
					panic("community: multiple sources")
				}
				src = commIdx{i, j}
				removal = true
			}

			k_aC += l.weight(n, u)
			// sigma_totC could be kept for each community
			// and updated for moves, changing the calculation
			// of sigma_totC here from O(n_c) to O(1), but
			// in practice the time savings do not appear
			// to be compelling and do not make up for the
			// increase in code complexity and space required.
			sigma_totC += l.edgeWeightOf[uid]
		}

		// See louvain.tex for a derivation of these equations.
		switch {
		case removal:
			// The community c was the current community,
			// so calculate the change due to removal.
			dQremove = k_aC /*^𝛼*/ - a_aa - gamma*k_a*(sigma_totC /*^𝛼*/ -k_a)/m2

		default:
			// Otherwise calculate the change due to an addition
			// to c and retain if it is the current best.
			dQ := k_aC /*^𝛽*/ - gamma*k_a*sigma_totC /*^𝛽*/ /m2
			if dQ > dQadd {
				dQadd = dQ
				dst = i
			}
		}
	}

	return 2 * (dQadd - dQremove) / m2, dst, src
}
