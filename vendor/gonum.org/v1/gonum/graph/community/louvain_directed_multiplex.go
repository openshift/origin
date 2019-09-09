// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package community

import (
	"fmt"
	"math"
	"sort"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/internal/set"
	"gonum.org/v1/gonum/graph/iterator"
)

// DirectedMultiplex is a directed multiplex graph.
type DirectedMultiplex interface {
	Multiplex

	// Layer returns the lth layer of the
	// multiplex graph.
	Layer(l int) graph.Directed
}

// qDirectedMultiplex returns the modularity Q score of the multiplex graph layers
// subdivided into the given communities at the given resolutions and weights. Q is
// returned as the vector of weighted Q scores for each layer of the multiplex graph.
// If communities is nil, the unclustered modularity score is returned.
// If weights is nil layers are equally weighted, otherwise the length of
// weights must equal the number of layers. If resolutions is nil, a resolution
// of 1.0 is used for all layers, otherwise either a single element slice may be used
// to specify a global resolution, or the length of resolutions must equal the number
// of layers. The resolution parameter is γ as defined in Reichardt and Bornholdt
// doi:10.1103/PhysRevE.74.016110.
// qUndirectedMultiplex will panic if the graph has any layer weight-scaled edge with
// negative edge weight.
//
//  Q_{layer} = w_{layer} \sum_{ij} [ A_{layer}*_{ij} - (\gamma_{layer} k_i k_j)/2m ] \delta(c_i,c_j)
//
// Note that Q values for multiplex graphs are not scaled by the total layer edge weight.
func qDirectedMultiplex(g DirectedMultiplex, communities [][]graph.Node, weights, resolutions []float64) []float64 {
	q := make([]float64, g.Depth())
	nodes := graph.NodesOf(g.Nodes())
	layerWeight := 1.0
	layerResolution := 1.0
	if len(resolutions) == 1 {
		layerResolution = resolutions[0]
	}
	for l := 0; l < g.Depth(); l++ {
		layer := g.Layer(l)

		if weights != nil {
			layerWeight = weights[l]
		}
		if layerWeight == 0 {
			continue
		}

		if len(resolutions) > 1 {
			layerResolution = resolutions[l]
		}

		var weight func(xid, yid int64) float64
		if layerWeight < 0 {
			weight = negativeWeightFuncFor(layer)
		} else {
			weight = positiveWeightFuncFor(layer)
		}

		// Calculate the total edge weight of the layer
		// and the table of penetrating edge weight sums.
		var m float64
		k := make(map[int64]directedWeights, len(nodes))
		for _, n := range nodes {
			var wOut float64
			u := n
			uid := u.ID()
			to := layer.From(uid)
			for to.Next() {
				wOut += weight(uid, to.Node().ID())
			}
			var wIn float64
			v := n
			vid := v.ID()
			from := layer.To(vid)
			for from.Next() {
				wIn += weight(from.Node().ID(), vid)
			}
			id := n.ID()
			w := weight(id, id)
			m += w + wOut // We only need to count edges once.
			k[n.ID()] = directedWeights{out: w + wOut, in: w + wIn}
		}

		if communities == nil {
			var qLayer float64
			for _, u := range nodes {
				uid := u.ID()
				kU := k[uid]
				qLayer += weight(uid, uid) - layerResolution*kU.out*kU.in/m
			}
			q[l] = layerWeight * qLayer
			continue
		}

		var qLayer float64
		for _, c := range communities {
			for _, u := range c {
				uid := u.ID()
				kU := k[uid]
				for _, v := range c {
					vid := v.ID()
					kV := k[vid]
					qLayer += weight(uid, vid) - layerResolution*kU.out*kV.in/m
				}
			}
		}
		q[l] = layerWeight * qLayer
	}

	return q
}

// DirectedLayers implements DirectedMultiplex.
type DirectedLayers []graph.Directed

// NewDirectedLayers returns a DirectedLayers using the provided layers
// ensuring there is a match between IDs for each layer.
func NewDirectedLayers(layers ...graph.Directed) (DirectedLayers, error) {
	if len(layers) == 0 {
		return nil, nil
	}
	base := make(set.Int64s)
	nodes := layers[0].Nodes()
	for nodes.Next() {
		base.Add(nodes.Node().ID())
	}
	for i, l := range layers[1:] {
		next := make(set.Int64s)
		nodes := l.Nodes()
		for nodes.Next() {
			next.Add(nodes.Node().ID())
		}
		if !set.Int64sEqual(base, next) {
			return nil, fmt.Errorf("community: layer ID mismatch between layers: %d", i+1)
		}
	}
	return layers, nil
}

// Nodes returns the nodes of the receiver.
func (g DirectedLayers) Nodes() graph.Nodes {
	if len(g) == 0 {
		return nil
	}
	return g[0].Nodes()
}

// Depth returns the depth of the multiplex graph.
func (g DirectedLayers) Depth() int { return len(g) }

// Layer returns the lth layer of the multiplex graph.
func (g DirectedLayers) Layer(l int) graph.Directed { return g[l] }

// louvainDirectedMultiplex returns the hierarchical modularization of g at the given resolution
// using the Louvain algorithm. If all is true and g has negatively weighted layers, all
// communities will be searched during the modularization. If src is nil, rand.Intn is
// used as the random generator. louvainDirectedMultiplex will panic if g has any edge with
// edge weight that does not sign-match the layer weight.
//
// graph.Undirect may be used as a shim to allow modularization of directed graphs.
func louvainDirectedMultiplex(g DirectedMultiplex, weights, resolutions []float64, all bool, src rand.Source) *ReducedDirectedMultiplex {
	if weights != nil && len(weights) != g.Depth() {
		panic("community: weights vector length mismatch")
	}
	if resolutions != nil && len(resolutions) != 1 && len(resolutions) != g.Depth() {
		panic("community: resolutions vector length mismatch")
	}

	// See louvain.tex for a detailed description
	// of the algorithm used here.

	c := reduceDirectedMultiplex(g, nil, weights)
	rnd := rand.Intn
	if src != nil {
		rnd = rand.New(src).Intn
	}
	for {
		l := newDirectedMultiplexLocalMover(c, c.communities, weights, resolutions, all)
		if l == nil {
			return c
		}
		if done := l.localMovingHeuristic(rnd); done {
			return c
		}
		c = reduceDirectedMultiplex(c, l.communities, weights)
	}
}

// ReducedDirectedMultiplex is a directed graph of communities derived from a
// parent graph by reduction.
type ReducedDirectedMultiplex struct {
	// nodes is the set of nodes held
	// by the graph. In a ReducedDirectedMultiplex
	// the node ID is the index into
	// nodes.
	nodes  []multiplexCommunity
	layers []directedEdges

	// communities is the community
	// structure of the graph.
	communities [][]graph.Node

	parent *ReducedDirectedMultiplex
}

var (
	_ DirectedMultiplex      = (*ReducedDirectedMultiplex)(nil)
	_ graph.WeightedDirected = (*directedLayerHandle)(nil)
)

// Nodes returns all the nodes in the graph.
func (g *ReducedDirectedMultiplex) Nodes() graph.Nodes {
	nodes := make([]graph.Node, len(g.nodes))
	for i := range g.nodes {
		nodes[i] = node(i)
	}
	return iterator.NewOrderedNodes(nodes)
}

// Depth returns the number of layers in the multiplex graph.
func (g *ReducedDirectedMultiplex) Depth() int { return len(g.layers) }

// Layer returns the lth layer of the multiplex graph.
func (g *ReducedDirectedMultiplex) Layer(l int) graph.Directed {
	return directedLayerHandle{multiplex: g, layer: l}
}

// Communities returns the community memberships of the nodes in the
// graph used to generate the reduced graph.
func (g *ReducedDirectedMultiplex) Communities() [][]graph.Node {
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
func (g *ReducedDirectedMultiplex) Structure() [][]graph.Node {
	return g.communities
}

// Expanded returns the next lower level of the module clustering or nil
// if at the lowest level.
func (g *ReducedDirectedMultiplex) Expanded() ReducedMultiplex {
	return g.parent
}

// reduceDirectedMultiplex returns a reduced graph constructed from g divided
// into the given communities. The communities value is mutated
// by the call to reduceDirectedMultiplex. If communities is nil and g is a
// ReducedDirectedMultiplex, it is returned unaltered.
func reduceDirectedMultiplex(g DirectedMultiplex, communities [][]graph.Node, weights []float64) *ReducedDirectedMultiplex {
	if communities == nil {
		if r, ok := g.(*ReducedDirectedMultiplex); ok {
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

		r := ReducedDirectedMultiplex{
			nodes:       make([]multiplexCommunity, len(nodes)),
			layers:      make([]directedEdges, g.Depth()),
			communities: communities,
		}
		communityOf := make(map[int64]int, len(nodes))
		for i, n := range nodes {
			r.nodes[i] = multiplexCommunity{id: i, nodes: []graph.Node{n}, weights: make([]float64, depth(weights))}
			communityOf[n.ID()] = i
		}
		for i := range r.layers {
			r.layers[i] = directedEdges{
				edgesFrom: make([][]int, len(nodes)),
				edgesTo:   make([][]int, len(nodes)),
				weights:   make(map[[2]int]float64),
			}
		}
		w := 1.0
		for l := 0; l < g.Depth(); l++ {
			layer := g.Layer(l)
			if weights != nil {
				w = weights[l]
			}
			if w == 0 {
				continue
			}
			var sign float64
			var weight func(xid, yid int64) float64
			if w < 0 {
				sign, weight = -1, negativeWeightFuncFor(layer)
			} else {
				sign, weight = 1, positiveWeightFuncFor(layer)
			}
			for _, n := range nodes {
				id := communityOf[n.ID()]

				var out []int
				u := n
				uid := u.ID()
				to := layer.From(uid)
				for to.Next() {
					vid := to.Node().ID()
					vcid := communityOf[vid]
					if vcid != id {
						out = append(out, vcid)
					}
					r.layers[l].weights[[2]int{id, vcid}] = sign * weight(uid, vid)
				}
				r.layers[l].edgesFrom[id] = out

				var in []int
				v := n
				vid := v.ID()
				from := layer.To(vid)
				for from.Next() {
					uid := from.Node().ID()
					ucid := communityOf[uid]
					if ucid != id {
						in = append(in, ucid)
					}
					r.layers[l].weights[[2]int{ucid, id}] = sign * weight(uid, vid)
				}
				r.layers[l].edgesTo[id] = in
			}
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

	r := ReducedDirectedMultiplex{
		nodes:  make([]multiplexCommunity, len(communities)),
		layers: make([]directedEdges, g.Depth()),
	}
	communityOf := make(map[int64]int, commNodes)
	for i, comm := range communities {
		r.nodes[i] = multiplexCommunity{id: i, nodes: comm, weights: make([]float64, depth(weights))}
		for _, n := range comm {
			communityOf[n.ID()] = i
		}
	}
	for i := range r.layers {
		r.layers[i] = directedEdges{
			edgesFrom: make([][]int, len(communities)),
			edgesTo:   make([][]int, len(communities)),
			weights:   make(map[[2]int]float64),
		}
	}
	r.communities = make([][]graph.Node, len(communities))
	for i := range r.communities {
		r.communities[i] = []graph.Node{node(i)}
	}
	if g, ok := g.(*ReducedDirectedMultiplex); ok {
		// Make sure we retain the truncated
		// community structure.
		g.communities = communities
		r.parent = g
	}
	w := 1.0
	for l := 0; l < g.Depth(); l++ {
		layer := g.Layer(l)
		if weights != nil {
			w = weights[l]
		}
		if w == 0 {
			continue
		}
		var sign float64
		var weight func(xid, yid int64) float64
		if w < 0 {
			sign, weight = -1, negativeWeightFuncFor(layer)
		} else {
			sign, weight = 1, positiveWeightFuncFor(layer)
		}
		for id, comm := range communities {
			var out, in []int
			for _, n := range comm {
				u := n
				uid := u.ID()
				for _, v := range comm {
					r.nodes[id].weights[l] += sign * weight(uid, v.ID())
				}

				to := layer.From(uid)
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
					r.layers[l].weights[[2]int{id, vcid}] += sign * weight(uid, vid) / 2
				}

				v := n
				vid := v.ID()
				from := layer.To(vid)
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
					r.layers[l].weights[[2]int{ucid, id}] += sign * weight(uid, vid) / 2
				}

			}
			r.layers[l].edgesFrom[id] = out
			r.layers[l].edgesTo[id] = in
		}
	}
	return &r
}

// directedLayerHandle is a handle to a multiplex graph layer.
type directedLayerHandle struct {
	// multiplex is the complete
	// multiplex graph.
	multiplex *ReducedDirectedMultiplex

	// layer is an index into the
	// multiplex for the current
	// layer.
	layer int
}

// Node returns the node with the given ID if it exists in the graph,
// and nil otherwise.
func (g directedLayerHandle) Node(id int64) graph.Node {
	if g.has(id) {
		return g.multiplex.nodes[id]
	}
	return nil
}

// has returns whether the node exists within the graph.
func (g directedLayerHandle) has(id int64) bool {
	return 0 <= id && id < int64(len(g.multiplex.nodes))
}

// Nodes returns all the nodes in the graph.
func (g directedLayerHandle) Nodes() graph.Nodes {
	nodes := make([]graph.Node, len(g.multiplex.nodes))
	for i := range g.multiplex.nodes {
		nodes[i] = node(i)
	}
	return iterator.NewOrderedNodes(nodes)
}

// From returns all nodes in g that can be reached directly from u.
func (g directedLayerHandle) From(uid int64) graph.Nodes {
	out := g.multiplex.layers[g.layer].edgesFrom[uid]
	nodes := make([]graph.Node, len(out))
	for i, vid := range out {
		nodes[i] = g.multiplex.nodes[vid]
	}
	return iterator.NewOrderedNodes(nodes)
}

// To returns all nodes in g that can reach directly to v.
func (g directedLayerHandle) To(vid int64) graph.Nodes {
	in := g.multiplex.layers[g.layer].edgesTo[vid]
	nodes := make([]graph.Node, len(in))
	for i, uid := range in {
		nodes[i] = g.multiplex.nodes[uid]
	}
	return iterator.NewOrderedNodes(nodes)
}

// HasEdgeBetween returns whether an edge exists between nodes x and y.
func (g directedLayerHandle) HasEdgeBetween(xid, yid int64) bool {
	if xid == yid {
		return false
	}
	if xid == yid || !isValidID(xid) || !isValidID(yid) {
		return false
	}
	_, ok := g.multiplex.layers[g.layer].weights[[2]int{int(xid), int(yid)}]
	if ok {
		return true
	}
	_, ok = g.multiplex.layers[g.layer].weights[[2]int{int(yid), int(xid)}]
	return ok
}

// HasEdgeFromTo returns whether an edge exists from node u to v.
func (g directedLayerHandle) HasEdgeFromTo(uid, vid int64) bool {
	if uid == vid || !isValidID(uid) || !isValidID(vid) {
		return false
	}
	_, ok := g.multiplex.layers[g.layer].weights[[2]int{int(uid), int(vid)}]
	return ok
}

// Edge returns the edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
func (g directedLayerHandle) Edge(uid, vid int64) graph.Edge {
	return g.WeightedEdge(uid, vid)
}

// WeightedEdge returns the weighted edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
func (g directedLayerHandle) WeightedEdge(uid, vid int64) graph.WeightedEdge {
	if uid == vid || !isValidID(uid) || !isValidID(vid) {
		return nil
	}
	w, ok := g.multiplex.layers[g.layer].weights[[2]int{int(uid), int(vid)}]
	if !ok {
		return nil
	}
	return multiplexEdge{from: g.multiplex.nodes[uid], to: g.multiplex.nodes[vid], weight: w}
}

// Weight returns the weight for the edge between x and y if Edge(x, y) returns a non-nil Edge.
// If x and y are the same node the internal node weight is returned. If there is no joining
// edge between the two nodes the weight value returned is zero. Weight returns true if an edge
// exists between x and y or if x and y have the same ID, false otherwise.
func (g directedLayerHandle) Weight(xid, yid int64) (w float64, ok bool) {
	if !isValidID(xid) || !isValidID(yid) {
		return 0, false
	}
	if xid == yid {
		return g.multiplex.nodes[xid].weights[g.layer], true
	}
	w, ok = g.multiplex.layers[g.layer].weights[[2]int{int(xid), int(yid)}]
	return w, ok
}

// directedMultiplexLocalMover is a step in graph modularity optimization.
type directedMultiplexLocalMover struct {
	g *ReducedDirectedMultiplex

	// nodes is the set of working nodes.
	nodes []graph.Node
	// edgeWeightsOf is the weighted degree
	// of each node indexed by ID.
	edgeWeightsOf [][]directedWeights

	// m is the total sum of
	// edge weights in g.
	m []float64

	// weight is the weight function
	// provided by g or a function
	// that returns the Weight value
	// of the non-nil edge between x
	// and y.
	weight []func(xid, yid int64) float64

	// communities is the current
	// division of g.
	communities [][]graph.Node
	// memberships is a mapping between
	// node ID and community membership.
	memberships []int

	// resolution is the Reichardt and
	// Bornholdt γ parameter as defined
	// in doi:10.1103/PhysRevE.74.016110.
	resolutions []float64

	// weights is the layer weights for
	// the modularisation.
	weights []float64

	// searchAll specifies whether the local
	// mover should consider non-connected
	// communities during the local moving
	// heuristic.
	searchAll bool

	// moved indicates that a call to
	// move has been made since the last
	// call to shuffle.
	moved bool

	// changed indicates that a move
	// has been made since the creation
	// of the local mover.
	changed bool
}

// newDirectedMultiplexLocalMover returns a new directedMultiplexLocalMover initialized with
// the graph g, a set of communities and a modularity resolution parameter. The
// node IDs of g must be contiguous in [0,n) where n is the number of nodes.
// If g has a zero edge weight sum, nil is returned.
func newDirectedMultiplexLocalMover(g *ReducedDirectedMultiplex, communities [][]graph.Node, weights, resolutions []float64, all bool) *directedMultiplexLocalMover {
	nodes := graph.NodesOf(g.Nodes())
	l := directedMultiplexLocalMover{
		g:             g,
		nodes:         nodes,
		edgeWeightsOf: make([][]directedWeights, g.Depth()),
		m:             make([]float64, g.Depth()),
		communities:   communities,
		memberships:   make([]int, len(nodes)),
		resolutions:   resolutions,
		weights:       weights,
		weight:        make([]func(xid, yid int64) float64, g.Depth()),
	}

	// Calculate the total edge weight of the graph
	// and degree weights for each node.
	var zero int
	for i := 0; i < g.Depth(); i++ {
		l.edgeWeightsOf[i] = make([]directedWeights, len(nodes))
		var weight func(xid, yid int64) float64

		if weights != nil {
			if weights[i] == 0 {
				zero++
				continue
			}
			if weights[i] < 0 {
				weight = negativeWeightFuncFor(g.Layer(i))
				l.searchAll = all
			} else {
				weight = positiveWeightFuncFor(g.Layer(i))
			}
		} else {
			weight = positiveWeightFuncFor(g.Layer(i))
		}

		l.weight[i] = weight
		layer := g.Layer(i)
		for _, n := range l.nodes {
			u := n
			uid := u.ID()
			var wOut float64
			to := layer.From(uid)
			for to.Next() {
				wOut += weight(uid, to.Node().ID())
			}

			v := n
			vid := v.ID()
			var wIn float64
			from := layer.To(vid)
			for from.Next() {
				wIn += weight(from.Node().ID(), vid)
			}

			id := n.ID()
			w := weight(id, id)
			l.edgeWeightsOf[i][uid] = directedWeights{out: w + wOut, in: w + wIn}
			l.m[i] += w + wOut
		}
		if l.m[i] == 0 {
			zero++
		}
	}
	if zero == g.Depth() {
		return nil
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
// directedMultiplexLocalMover has not made any improvement to the community
// structure and so the Louvain algorithm is done.
func (l *directedMultiplexLocalMover) localMovingHeuristic(rnd func(int) int) (done bool) {
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
// directedMultiplexLocalMover using the random source rnd which should return
// an integer in the range [0,n).
func (l *directedMultiplexLocalMover) shuffle(rnd func(n int) int) {
	l.moved = false
	for i := range l.nodes[:len(l.nodes)-1] {
		j := i + rnd(len(l.nodes)-i)
		l.nodes[i], l.nodes[j] = l.nodes[j], l.nodes[i]
	}
}

// move moves the node at src to the community at dst.
func (l *directedMultiplexLocalMover) move(dst int, src commIdx) {
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
// directedMultiplexLocalMover's communities field is returned in src if n
// is in communities.
func (l *directedMultiplexLocalMover) deltaQ(n graph.Node) (deltaQ float64, dst int, src commIdx) {
	id := n.ID()

	var iterator minTaker
	if l.searchAll {
		iterator = &dense{n: len(l.communities)}
	} else {
		// Find communities connected to n.
		connected := make(set.Ints)
		// The following for loop is equivalent to:
		//
		//  for i := 0; i < l.g.Depth(); i++ {
		//  	for _, v := range l.g.Layer(i).From(n) {
		//  		connected.Add(l.memberships[v.ID()])
		//  	}
		//  	for _, v := range l.g.Layer(i).To(n) {
		//  		connected.Add(l.memberships[v.ID()])
		//  	}
		//  }
		//
		// This is done to avoid an allocation for
		// each layer.
		for _, layer := range l.g.layers {
			for _, vid := range layer.edgesFrom[id] {
				connected.Add(l.memberships[vid])
			}
			for _, vid := range layer.edgesTo[id] {
				connected.Add(l.memberships[vid])
			}
		}
		// Insert the node's own community.
		connected.Add(l.memberships[id])
		iterator = newSlice(connected)
	}

	// Calculate the highest modularity gain
	// from moving into another community and
	// keep the index of that community.
	var dQremove float64
	dQadd, dst, src := math.Inf(-1), -1, commIdx{-1, -1}
	var i int
	for iterator.TakeMin(&i) {
		c := l.communities[i]
		var removal bool
		var _dQadd float64
		for layer := 0; layer < l.g.Depth(); layer++ {
			m := l.m[layer]
			if m == 0 {
				// Do not consider layers with zero sum edge weight.
				continue
			}
			w := 1.0
			if l.weights != nil {
				w = l.weights[layer]
			}
			if w == 0 {
				// Do not consider layers with zero weighting.
				continue
			}

			var k_aC, sigma_totC directedWeights // C is a substitution for ^𝛼 or ^𝛽.
			removal = false
			for j, u := range c {
				uid := u.ID()
				if uid == id {
					// Only mark and check src community on the first layer.
					if layer == 0 {
						if src.community != -1 {
							panic("community: multiple sources")
						}
						src = commIdx{i, j}
					}
					removal = true
				}

				k_aC.in += l.weight[layer](id, uid)
				k_aC.out += l.weight[layer](uid, id)
				// sigma_totC could be kept for each community
				// and updated for moves, changing the calculation
				// of sigma_totC here from O(n_c) to O(1), but
				// in practice the time savings do not appear
				// to be compelling and do not make up for the
				// increase in code complexity and space required.
				w := l.edgeWeightsOf[layer][uid]
				sigma_totC.in += w.in
				sigma_totC.out += w.out
			}

			a_aa := l.weight[layer](id, id)
			k_a := l.edgeWeightsOf[layer][id]
			gamma := 1.0
			if l.resolutions != nil {
				if len(l.resolutions) == 1 {
					gamma = l.resolutions[0]
				} else {
					gamma = l.resolutions[layer]
				}
			}

			// See louvain.tex for a derivation of these equations.
			// The weighting term, w, is described in V Traag,
			// "Algorithms and dynamical models for communities and
			// reputation in social networks", chapter 5.
			// http://www.traag.net/wp/wp-content/papercite-data/pdf/traag_algorithms_2013.pdf
			switch {
			case removal:
				// The community c was the current community,
				// so calculate the change due to removal.
				dQremove += w * ((k_aC.in /*^𝛼*/ - a_aa) + (k_aC.out /*^𝛼*/ - a_aa) -
					gamma*(k_a.in*(sigma_totC.out /*^𝛼*/ -k_a.out)+k_a.out*(sigma_totC.in /*^𝛼*/ -k_a.in))/m)

			default:
				// Otherwise calculate the change due to an addition
				// to c.
				_dQadd += w * (k_aC.in /*^𝛽*/ + k_aC.out /*^𝛽*/ -
					gamma*(k_a.in*sigma_totC.out /*^𝛽*/ +k_a.out*sigma_totC.in /*^𝛽*/)/m)
			}
		}
		if !removal && _dQadd > dQadd {
			dQadd = _dQadd
			dst = i
		}
	}

	return dQadd - dQremove, dst, src
}
