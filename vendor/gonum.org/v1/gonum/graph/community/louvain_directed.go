// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package community

import (
	"math"
	"sort"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/internal/set"
	"gonum.org/v1/gonum/graph/iterator"
)

// qDirected returns the modularity Q score of the graph g subdivided into the
// given communities at the given resolution. If communities is nil, the
// unclustered modularity score is returned. The resolution parameter
// is γ as defined in Reichardt and Bornholdt doi:10.1103/PhysRevE.74.016110.
// qDirected will panic if g has any edge with negative edge weight.
//
//  Q = 1/m \sum_{ij} [ A_{ij} - (\gamma k_i^in k_j^out)/m ] \delta(c_i,c_j)
//
func qDirected(g graph.Directed, communities [][]graph.Node, resolution float64) float64 {
	nodes := graph.NodesOf(g.Nodes())
	weight := positiveWeightFuncFor(g)

	// Calculate the total edge weight of the graph
	// and the table of penetrating edge weight sums.
	var m float64
	k := make(map[int64]directedWeights, len(nodes))
	for _, n := range nodes {
		var wOut float64
		u := n
		uid := u.ID()
		to := g.From(uid)
		for to.Next() {
			wOut += weight(uid, to.Node().ID())
		}
		var wIn float64
		v := n
		vid := v.ID()
		from := g.To(vid)
		for from.Next() {
			wIn += weight(from.Node().ID(), vid)
		}
		id := n.ID()
		w := weight(id, id)
		m += w + wOut // We only need to count edges once.
		k[id] = directedWeights{out: w + wOut, in: w + wIn}
	}

	if communities == nil {
		var q float64
		for _, u := range nodes {
			uid := u.ID()
			kU := k[uid]
			q += weight(uid, uid) - resolution*kU.out*kU.in/m
		}
		return q / m
	}

	var q float64
	for _, c := range communities {
		for _, u := range c {
			uid := u.ID()
			kU := k[uid]
			for _, v := range c {
				vid := v.ID()
				kV := k[vid]
				q += weight(uid, vid) - resolution*kU.out*kV.in/m
			}
		}
	}
	return q / m
}

// louvainDirected returns the hierarchical modularization of g at the given
// resolution using the Louvain algorithm. If src is nil, rand.Intn is used
// as the random generator. louvainDirected will panic if g has any edge with negative
// edge weight.
func louvainDirected(g graph.Directed, resolution float64, src rand.Source) ReducedGraph {
	// See louvain.tex for a detailed description
	// of the algorithm used here.

	c := reduceDirected(g, nil)
	rnd := rand.Intn
	if src != nil {
		rnd = rand.New(src).Intn
	}
	for {
		l := newDirectedLocalMover(c, c.communities, resolution)
		if l == nil {
			return c
		}
		if done := l.localMovingHeuristic(rnd); done {
			return c
		}
		c = reduceDirected(c, l.communities)
	}
}

// ReducedDirected is a directed graph of communities derived from a
// parent graph by reduction.
type ReducedDirected struct {
	// nodes is the set of nodes held
	// by the graph. In a ReducedDirected
	// the node ID is the index into
	// nodes.
	nodes []community
	directedEdges

	// communities is the community
	// structure of the graph.
	communities [][]graph.Node

	parent *ReducedDirected
}

var (
	reducedDirected = (*ReducedDirected)(nil)

	_ graph.WeightedDirected = reducedDirected
	_ ReducedGraph           = reducedDirected
)

// Communities returns the community memberships of the nodes in the
// graph used to generate the reduced graph.
func (g *ReducedDirected) Communities() [][]graph.Node {
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
func (g *ReducedDirected) Structure() [][]graph.Node {
	return g.communities
}

// Expanded returns the next lower level of the module clustering or nil
// if at the lowest level.
func (g *ReducedDirected) Expanded() ReducedGraph {
	return g.parent
}

// reduceDirected returns a reduced graph constructed from g divided
// into the given communities. The communities value is mutated
// by the call to reduceDirected. If communities is nil and g is a
// ReducedDirected, it is returned unaltered.
func reduceDirected(g graph.Directed, communities [][]graph.Node) *ReducedDirected {
	if communities == nil {
		if r, ok := g.(*ReducedDirected); ok {
			return r
		}

		nodes := graph.NodesOf(g.Nodes())
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
		r := ReducedDirected{
			nodes: make([]community, len(nodes)),
			directedEdges: directedEdges{
				edgesFrom: make([][]int, len(nodes)),
				edgesTo:   make([][]int, len(nodes)),
				weights:   make(map[[2]int]float64),
			},
			communities: communities,
		}
		communityOf := make(map[int64]int, len(nodes))
		for i, n := range nodes {
			r.nodes[i] = community{id: i, nodes: []graph.Node{n}}
			communityOf[n.ID()] = i
		}
		for _, n := range nodes {
			id := communityOf[n.ID()]

			var out []int
			u := n
			uid := u.ID()
			to := g.From(uid)
			for to.Next() {
				vid := to.Node().ID()
				vcid := communityOf[vid]
				if vcid != id {
					out = append(out, vcid)
				}
				r.weights[[2]int{id, vcid}] = weight(uid, vid)
			}
			r.edgesFrom[id] = out

			var in []int
			v := n
			vid := v.ID()
			from := g.To(vid)
			for from.Next() {
				uid := from.Node().ID()
				ucid := communityOf[uid]
				if ucid != id {
					in = append(in, ucid)
				}
				r.weights[[2]int{ucid, id}] = weight(uid, vid)
			}
			r.edgesTo[id] = in
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

	r := ReducedDirected{
		nodes: make([]community, len(communities)),
		directedEdges: directedEdges{
			edgesFrom: make([][]int, len(communities)),
			edgesTo:   make([][]int, len(communities)),
			weights:   make(map[[2]int]float64),
		},
	}
	r.communities = make([][]graph.Node, len(communities))
	for i := range r.communities {
		r.communities[i] = []graph.Node{node(i)}
	}
	if g, ok := g.(*ReducedDirected); ok {
		// Make sure we retain the truncated
		// community structure.
		g.communities = communities
		r.parent = g
	}
	weight := positiveWeightFuncFor(g)
	communityOf := make(map[int64]int, commNodes)
	for i, comm := range communities {
		r.nodes[i] = community{id: i, nodes: comm}
		for _, n := range comm {
			communityOf[n.ID()] = i
		}
	}
	for id, comm := range communities {
		var out, in []int
		for _, n := range comm {
			u := n
			uid := u.ID()
			for _, v := range comm {
				r.nodes[id].weight += weight(uid, v.ID())
			}

			to := g.From(uid)
			for to.Next() {
				vid := to.Node().ID()
				vcid := communityOf[vid]
				found := false
				for _, e := range out {
					if e == vcid {
						found = true
						break
					}
				}
				if !found && vcid != id {
					out = append(out, vcid)
				}
				// Add half weights because the other
				// ends of edges are also counted.
				r.weights[[2]int{id, vcid}] += weight(uid, vid) / 2
			}

			v := n
			vid := v.ID()
			from := g.To(vid)
			for from.Next() {
				uid := from.Node().ID()
				ucid := communityOf[uid]
				found := false
				for _, e := range in {
					if e == ucid {
						found = true
						break
					}
				}
				if !found && ucid != id {
					in = append(in, ucid)
				}
				// Add half weights because the other
				// ends of edges are also counted.
				r.weights[[2]int{ucid, id}] += weight(uid, vid) / 2
			}
		}
		r.edgesFrom[id] = out
		r.edgesTo[id] = in
	}
	return &r
}

// Node returns the node with the given ID if it exists in the graph,
// and nil otherwise.
func (g *ReducedDirected) Node(id int64) graph.Node {
	if g.has(id) {
		return g.nodes[id]
	}
	return nil
}

// has returns whether the node exists within the graph.
func (g *ReducedDirected) has(id int64) bool {
	return 0 <= id && id < int64(len(g.nodes))
}

// Nodes returns all the nodes in the graph.
func (g *ReducedDirected) Nodes() graph.Nodes {
	nodes := make([]graph.Node, len(g.nodes))
	for i := range g.nodes {
		nodes[i] = node(i)
	}
	return iterator.NewOrderedNodes(nodes)
}

// From returns all nodes in g that can be reached directly from u.
func (g *ReducedDirected) From(uid int64) graph.Nodes {
	out := g.edgesFrom[uid]
	nodes := make([]graph.Node, len(out))
	for i, vid := range out {
		nodes[i] = g.nodes[vid]
	}
	return iterator.NewOrderedNodes(nodes)
}

// To returns all nodes in g that can reach directly to v.
func (g *ReducedDirected) To(vid int64) graph.Nodes {
	in := g.edgesTo[vid]
	nodes := make([]graph.Node, len(in))
	for i, uid := range in {
		nodes[i] = g.nodes[uid]
	}
	return iterator.NewOrderedNodes(nodes)
}

// HasEdgeBetween returns whether an edge exists between nodes x and y.
func (g *ReducedDirected) HasEdgeBetween(xid, yid int64) bool {
	if xid == yid || !isValidID(xid) || !isValidID(yid) {
		return false
	}
	_, ok := g.weights[[2]int{int(xid), int(yid)}]
	if ok {
		return true
	}
	_, ok = g.weights[[2]int{int(yid), int(xid)}]
	return ok
}

// HasEdgeFromTo returns whether an edge exists from node u to v.
func (g *ReducedDirected) HasEdgeFromTo(uid, vid int64) bool {
	if uid == vid || !isValidID(uid) || !isValidID(vid) {
		return false
	}
	_, ok := g.weights[[2]int{int(uid), int(vid)}]
	return ok
}

// Edge returns the edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
func (g *ReducedDirected) Edge(uid, vid int64) graph.Edge {
	return g.WeightedEdge(uid, vid)
}

// WeightedEdge returns the weighted edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
func (g *ReducedDirected) WeightedEdge(uid, vid int64) graph.WeightedEdge {
	if uid == vid || !isValidID(uid) || !isValidID(vid) {
		return nil
	}
	w, ok := g.weights[[2]int{int(uid), int(vid)}]
	if !ok {
		return nil
	}
	return edge{from: g.nodes[uid], to: g.nodes[vid], weight: w}
}

// Weight returns the weight for the edge between x and y if Edge(x, y) returns a non-nil Edge.
// If x and y are the same node the internal node weight is returned. If there is no joining
// edge between the two nodes the weight value returned is zero. Weight returns true if an edge
// exists between x and y or if x and y have the same ID, false otherwise.
func (g *ReducedDirected) Weight(xid, yid int64) (w float64, ok bool) {
	if !isValidID(xid) || !isValidID(yid) {
		return 0, false
	}
	if xid == yid {
		return g.nodes[xid].weight, true
	}
	w, ok = g.weights[[2]int{int(xid), int(yid)}]
	return w, ok
}

// directedLocalMover is a step in graph modularity optimization.
type directedLocalMover struct {
	g *ReducedDirected

	// nodes is the set of working nodes.
	nodes []graph.Node
	// edgeWeightsOf is the weighted degree
	// of each node indexed by ID.
	edgeWeightsOf []directedWeights

	// m is the total sum of edge
	// weights in g.
	m float64

	// weight is the weight function
	// provided by g or a function
	// that returns the Weight value
	// of the non-nil edge between x
	// and y.
	weight func(xid, yid int64) float64

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

type directedWeights struct {
	out, in float64
}

// newDirectedLocalMover returns a new directedLocalMover initialized with
// the graph g, a set of communities and a modularity resolution parameter.
// The node IDs of g must be contiguous in [0,n) where n is the number of
// nodes.
// If g has a zero edge weight sum, nil is returned.
func newDirectedLocalMover(g *ReducedDirected, communities [][]graph.Node, resolution float64) *directedLocalMover {
	nodes := graph.NodesOf(g.Nodes())
	l := directedLocalMover{
		g:             g,
		nodes:         nodes,
		edgeWeightsOf: make([]directedWeights, len(nodes)),
		communities:   communities,
		memberships:   make([]int, len(nodes)),
		resolution:    resolution,
		weight:        positiveWeightFuncFor(g),
	}

	// Calculate the total edge weight of the graph
	// and degree weights for each node.
	for _, n := range l.nodes {
		u := n
		var wOut float64
		uid := u.ID()
		to := g.From(uid)
		for to.Next() {
			wOut += l.weight(uid, to.Node().ID())
		}

		v := n
		var wIn float64
		vid := v.ID()
		from := g.To(vid)
		for from.Next() {
			wIn += l.weight(from.Node().ID(), vid)
		}

		id := n.ID()
		w := l.weight(id, id)
		l.edgeWeightsOf[id] = directedWeights{out: w + wOut, in: w + wIn}
		l.m += w + wOut
	}

	// Assign membership mappings.
	for i, c := range communities {
		for _, n := range c {
			l.memberships[n.ID()] = i
		}
	}

	return &l
}

// localMovingHeuristic performs the Louvain local moving heuristic until
// no further moves can be made. It returns a boolean indicating that the
// directedLocalMover has not made any improvement to the community structure and
// so the Louvain algorithm is done.
func (l *directedLocalMover) localMovingHeuristic(rnd func(int) int) (done bool) {
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
// directedLocalMover using the random source rnd which should return an
// integer in the range [0,n).
func (l *directedLocalMover) shuffle(rnd func(n int) int) {
	l.moved = false
	for i := range l.nodes[:len(l.nodes)-1] {
		j := i + rnd(len(l.nodes)-i)
		l.nodes[i], l.nodes[j] = l.nodes[j], l.nodes[i]
	}
}

// move moves the node at src to the community at dst.
func (l *directedLocalMover) move(dst int, src commIdx) {
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
// the index of the chosen destination. The index into the directedLocalMover's
// communities field is returned in src if n is in communities.
func (l *directedLocalMover) deltaQ(n graph.Node) (deltaQ float64, dst int, src commIdx) {
	id := n.ID()

	a_aa := l.weight(id, id)
	k_a := l.edgeWeightsOf[id]
	m := l.m
	gamma := l.resolution

	// Find communities connected to n.
	connected := make(set.Ints)
	// The following for loop is equivalent to:
	//
	//  for _, v := range l.g.From(n) {
	//  	connected.Add(l.memberships[v.ID()])
	//  }
	//  for _, v := range l.g.To(n) {
	//  	connected.Add(l.memberships[v.ID()])
	//  }
	//
	// This is done to avoid two allocations.
	for _, vid := range l.g.edgesFrom[id] {
		connected.Add(l.memberships[vid])
	}
	for _, vid := range l.g.edgesTo[id] {
		connected.Add(l.memberships[vid])
	}
	// Insert the node's own community.
	connected.Add(l.memberships[id])

	candidates := make([]int, 0, len(connected))
	for i := range connected {
		candidates = append(candidates, i)
	}
	sort.Ints(candidates)

	// Calculate the highest modularity gain
	// from moving into another community and
	// keep the index of that community.
	var dQremove float64
	dQadd, dst, src := math.Inf(-1), -1, commIdx{-1, -1}
	for _, i := range candidates {
		c := l.communities[i]
		var k_aC, sigma_totC directedWeights // C is a substitution for ^𝛼 or ^𝛽.
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

			k_aC.in += l.weight(uid, id)
			k_aC.out += l.weight(id, uid)
			// sigma_totC could be kept for each community
			// and updated for moves, changing the calculation
			// of sigma_totC here from O(n_c) to O(1), but
			// in practice the time savings do not appear
			// to be compelling and do not make up for the
			// increase in code complexity and space required.
			w := l.edgeWeightsOf[uid]
			sigma_totC.in += w.in
			sigma_totC.out += w.out
		}

		// See louvain.tex for a derivation of these equations.
		switch {
		case removal:
			// The community c was the current community,
			// so calculate the change due to removal.
			dQremove = (k_aC.in /*^𝛼*/ - a_aa) + (k_aC.out /*^𝛼*/ - a_aa) -
				gamma*(k_a.in*(sigma_totC.out /*^𝛼*/ -k_a.out)+k_a.out*(sigma_totC.in /*^𝛼*/ -k_a.in))/m

		default:
			// Otherwise calculate the change due to an addition
			// to c and retain if it is the current best.
			dQ := k_aC.in /*^𝛽*/ + k_aC.out /*^𝛽*/ -
				gamma*(k_a.in*sigma_totC.out /*^𝛽*/ +k_a.out*sigma_totC.in /*^𝛽*/)/m

			if dQ > dQadd {
				dQadd = dQ
				dst = i
			}
		}
	}

	return (dQadd - dQremove) / m, dst, src
}
