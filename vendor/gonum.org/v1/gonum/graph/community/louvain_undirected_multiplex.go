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

// UndirectedMultiplex is an undirected multiplex graph.
type UndirectedMultiplex interface {
	Multiplex

	// Layer returns the lth layer of the
	// multiplex graph.
	Layer(l int) graph.Undirected
}

// qUndirectedMultiplex returns the modularity Q score of the multiplex graph layers
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
//
// graph.Undirect may be used as a shim to allow calculation of Q for
// directed graphs.
func qUndirectedMultiplex(g UndirectedMultiplex, communities [][]graph.Node, weights, resolutions []float64) []float64 {
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
		var m2 float64
		k := make(map[int64]float64, len(nodes))
		for _, u := range nodes {
			uid := u.ID()
			w := weight(uid, uid)
			to := layer.From(uid)
			for to.Next() {
				w += weight(uid, to.Node().ID())
			}
			m2 += w
			k[uid] = w
		}

		if communities == nil {
			var qLayer float64
			for _, u := range nodes {
				uid := u.ID()
				kU := k[uid]
				qLayer += weight(uid, uid) - layerResolution*kU*kU/m2
			}
			q[l] = layerWeight * qLayer
			continue
		}

		// Iterate over the communities, calculating
		// the non-self edge weights for the upper
		// triangle and adjust the diagonal.
		var qLayer float64
		for _, c := range communities {
			for i, u := range c {
				uid := u.ID()
				kU := k[uid]
				qLayer += weight(uid, uid) - layerResolution*kU*kU/m2
				for _, v := range c[i+1:] {
					vid := v.ID()
					qLayer += 2 * (weight(uid, vid) - layerResolution*kU*k[vid]/m2)
				}
			}
		}
		q[l] = layerWeight * qLayer
	}

	return q
}

// UndirectedLayers implements UndirectedMultiplex.
type UndirectedLayers []graph.Undirected

// NewUndirectedLayers returns an UndirectedLayers using the provided layers
// ensuring there is a match between IDs for each layer.
func NewUndirectedLayers(layers ...graph.Undirected) (UndirectedLayers, error) {
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
		if !set.Int64sEqual(next, base) {
			return nil, fmt.Errorf("community: layer ID mismatch between layers: %d", i+1)
		}
	}
	return layers, nil
}

// Nodes returns the nodes of the receiver.
func (g UndirectedLayers) Nodes() graph.Nodes {
	if len(g) == 0 {
		return nil
	}
	return g[0].Nodes()
}

// Depth returns the depth of the multiplex graph.
func (g UndirectedLayers) Depth() int { return len(g) }

// Layer returns the lth layer of the multiplex graph.
func (g UndirectedLayers) Layer(l int) graph.Undirected { return g[l] }

// louvainUndirectedMultiplex returns the hierarchical modularization of g at the given resolution
// using the Louvain algorithm. If all is true and g has negatively weighted layers, all
// communities will be searched during the modularization. If src is nil, rand.Intn is
// used as the random generator. louvainUndirectedMultiplex will panic if g has any edge with
// edge weight that does not sign-match the layer weight.
//
// graph.Undirect may be used as a shim to allow modularization of directed graphs.
func louvainUndirectedMultiplex(g UndirectedMultiplex, weights, resolutions []float64, all bool, src rand.Source) *ReducedUndirectedMultiplex {
	if weights != nil && len(weights) != g.Depth() {
		panic("community: weights vector length mismatch")
	}
	if resolutions != nil && len(resolutions) != 1 && len(resolutions) != g.Depth() {
		panic("community: resolutions vector length mismatch")
	}

	// See louvain.tex for a detailed description
	// of the algorithm used here.

	c := reduceUndirectedMultiplex(g, nil, weights)
	rnd := rand.Intn
	if src != nil {
		rnd = rand.New(src).Intn
	}
	for {
		l := newUndirectedMultiplexLocalMover(c, c.communities, weights, resolutions, all)
		if l == nil {
			return c
		}
		if done := l.localMovingHeuristic(rnd); done {
			return c
		}
		c = reduceUndirectedMultiplex(c, l.communities, weights)
	}
}

// ReducedUndirectedMultiplex is an undirected graph of communities derived from a
// parent graph by reduction.
type ReducedUndirectedMultiplex struct {
	// nodes is the set of nodes held
	// by the graph. In a ReducedUndirectedMultiplex
	// the node ID is the index into
	// nodes.
	nodes  []multiplexCommunity
	layers []undirectedEdges

	// communities is the community
	// structure of the graph.
	communities [][]graph.Node

	parent *ReducedUndirectedMultiplex
}

var (
	_ UndirectedMultiplex      = (*ReducedUndirectedMultiplex)(nil)
	_ graph.WeightedUndirected = (*undirectedLayerHandle)(nil)
)

// Nodes returns all the nodes in the graph.
func (g *ReducedUndirectedMultiplex) Nodes() graph.Nodes {
	nodes := make([]graph.Node, len(g.nodes))
	for i := range g.nodes {
		nodes[i] = node(i)
	}
	return iterator.NewOrderedNodes(nodes)
}

// Depth returns the number of layers in the multiplex graph.
func (g *ReducedUndirectedMultiplex) Depth() int { return len(g.layers) }

// Layer returns the lth layer of the multiplex graph.
func (g *ReducedUndirectedMultiplex) Layer(l int) graph.Undirected {
	return undirectedLayerHandle{multiplex: g, layer: l}
}

// Communities returns the community memberships of the nodes in the
// graph used to generate the reduced graph.
func (g *ReducedUndirectedMultiplex) Communities() [][]graph.Node {
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
func (g *ReducedUndirectedMultiplex) Structure() [][]graph.Node {
	return g.communities
}

// Expanded returns the next lower level of the module clustering or nil
// if at the lowest level.
func (g *ReducedUndirectedMultiplex) Expanded() ReducedMultiplex {
	return g.parent
}

// reduceUndirectedMultiplex returns a reduced graph constructed from g divided
// into the given communities. The communities value is mutated
// by the call to reduceUndirectedMultiplex. If communities is nil and g is a
// ReducedUndirectedMultiplex, it is returned unaltered.
func reduceUndirectedMultiplex(g UndirectedMultiplex, communities [][]graph.Node, weights []float64) *ReducedUndirectedMultiplex {
	if communities == nil {
		if r, ok := g.(*ReducedUndirectedMultiplex); ok {
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

		r := ReducedUndirectedMultiplex{
			nodes:       make([]multiplexCommunity, len(nodes)),
			layers:      make([]undirectedEdges, g.Depth()),
			communities: communities,
		}
		communityOf := make(map[int64]int, len(nodes))
		for i, n := range nodes {
			r.nodes[i] = multiplexCommunity{id: i, nodes: []graph.Node{n}, weights: make([]float64, depth(weights))}
			communityOf[n.ID()] = i
		}
		for i := range r.layers {
			r.layers[i] = undirectedEdges{
				edges:   make([][]int, len(nodes)),
				weights: make(map[[2]int]float64),
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
			for _, u := range nodes {
				var out []int
				uid := u.ID()
				ucid := communityOf[uid]
				to := layer.From(uid)
				for to.Next() {
					vid := to.Node().ID()
					vcid := communityOf[vid]
					if vcid != ucid {
						out = append(out, vcid)
					}
					if ucid < vcid {
						// Only store the weight once.
						r.layers[l].weights[[2]int{ucid, vcid}] = sign * weight(uid, vid)
					}
				}
				r.layers[l].edges[ucid] = out
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

	r := ReducedUndirectedMultiplex{
		nodes:  make([]multiplexCommunity, len(communities)),
		layers: make([]undirectedEdges, g.Depth()),
	}
	communityOf := make(map[int64]int, commNodes)
	for i, comm := range communities {
		r.nodes[i] = multiplexCommunity{id: i, nodes: comm, weights: make([]float64, depth(weights))}
		for _, n := range comm {
			communityOf[n.ID()] = i
		}
	}
	for i := range r.layers {
		r.layers[i] = undirectedEdges{
			edges:   make([][]int, len(communities)),
			weights: make(map[[2]int]float64),
		}
	}
	r.communities = make([][]graph.Node, len(communities))
	for i := range r.communities {
		r.communities[i] = []graph.Node{node(i)}
	}
	if g, ok := g.(*ReducedUndirectedMultiplex); ok {
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
		for ucid, comm := range communities {
			var out []int
			for i, u := range comm {
				uid := u.ID()
				r.nodes[ucid].weights[l] += sign * weight(uid, uid)
				for _, v := range comm[i+1:] {
					r.nodes[ucid].weights[l] += 2 * sign * weight(uid, v.ID())
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
					if !found && vcid != ucid {
						out = append(out, vcid)
					}
					if ucid < vcid {
						// Only store the weight once.
						r.layers[l].weights[[2]int{ucid, vcid}] += sign * weight(uid, vid)
					}
				}
			}
			r.layers[l].edges[ucid] = out
		}
	}
	return &r
}

// undirectedLayerHandle is a handle to a multiplex graph layer.
type undirectedLayerHandle struct {
	// multiplex is the complete
	// multiplex graph.
	multiplex *ReducedUndirectedMultiplex

	// layer is an index into the
	// multiplex for the current
	// layer.
	layer int
}

// Node returns the node with the given ID if it exists in the graph,
// and nil otherwise.
func (g undirectedLayerHandle) Node(id int64) graph.Node {
	if g.has(id) {
		return g.multiplex.nodes[id]
	}
	return nil
}

// has returns whether the node exists within the graph.
func (g undirectedLayerHandle) has(id int64) bool {
	return 0 <= id && id < int64(len(g.multiplex.nodes))
}

// Nodes returns all the nodes in the graph.
func (g undirectedLayerHandle) Nodes() graph.Nodes {
	nodes := make([]graph.Node, len(g.multiplex.nodes))
	for i := range g.multiplex.nodes {
		nodes[i] = node(i)
	}
	return iterator.NewOrderedNodes(nodes)
}

// From returns all nodes in g that can be reached directly from u.
func (g undirectedLayerHandle) From(uid int64) graph.Nodes {
	out := g.multiplex.layers[g.layer].edges[uid]
	nodes := make([]graph.Node, len(out))
	for i, vid := range out {
		nodes[i] = g.multiplex.nodes[vid]
	}
	return iterator.NewOrderedNodes(nodes)
}

// HasEdgeBetween returns whether an edge exists between nodes x and y.
func (g undirectedLayerHandle) HasEdgeBetween(xid, yid int64) bool {
	if xid == yid || !isValidID(xid) || !isValidID(yid) {
		return false
	}
	if xid > yid {
		xid, yid = yid, xid
	}
	_, ok := g.multiplex.layers[g.layer].weights[[2]int{int(xid), int(yid)}]
	return ok
}

// Edge returns the edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
func (g undirectedLayerHandle) Edge(uid, vid int64) graph.Edge {
	return g.WeightedEdgeBetween(uid, vid)
}

// WeightedEdge returns the weighted edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
func (g undirectedLayerHandle) WeightedEdge(uid, vid int64) graph.WeightedEdge {
	return g.WeightedEdgeBetween(uid, vid)
}

// EdgeBetween returns the edge between nodes x and y.
func (g undirectedLayerHandle) EdgeBetween(xid, yid int64) graph.Edge {
	return g.WeightedEdgeBetween(xid, yid)
}

// WeightedEdgeBetween returns the weighted edge between nodes x and y.
func (g undirectedLayerHandle) WeightedEdgeBetween(xid, yid int64) graph.WeightedEdge {
	if xid == yid || !isValidID(xid) || !isValidID(yid) {
		return nil
	}
	if yid < xid {
		xid, yid = yid, xid
	}
	w, ok := g.multiplex.layers[g.layer].weights[[2]int{int(xid), int(yid)}]
	if !ok {
		return nil
	}
	return multiplexEdge{from: g.multiplex.nodes[xid], to: g.multiplex.nodes[yid], weight: w}
}

// Weight returns the weight for the edge between x and y if Edge(x, y) returns a non-nil Edge.
// If x and y are the same node the internal node weight is returned. If there is no joining
// edge between the two nodes the weight value returned is zero. Weight returns true if an edge
// exists between x and y or if x and y have the same ID, false otherwise.
func (g undirectedLayerHandle) Weight(xid, yid int64) (w float64, ok bool) {
	if !isValidID(xid) || !isValidID(yid) {
		return 0, false
	}
	if xid == yid {
		return g.multiplex.nodes[xid].weights[g.layer], true
	}
	if xid > yid {
		xid, yid = yid, xid
	}
	w, ok = g.multiplex.layers[g.layer].weights[[2]int{int(xid), int(yid)}]
	return w, ok
}

// undirectedMultiplexLocalMover is a step in graph modularity optimization.
type undirectedMultiplexLocalMover struct {
	g *ReducedUndirectedMultiplex

	// nodes is the set of working nodes.
	nodes []graph.Node
	// edgeWeightOf is the weighted degree
	// of each node indexed by ID.
	edgeWeightOf [][]float64

	// m2 is the total sum of
	// edge weights in g.
	m2 []float64

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

// newUndirectedMultiplexLocalMover returns a new undirectedMultiplexLocalMover initialized with
// the graph g, a set of communities and a modularity resolution parameter. The
// node IDs of g must be contiguous in [0,n) where n is the number of nodes.
// If g has a zero edge weight sum, nil is returned.
func newUndirectedMultiplexLocalMover(g *ReducedUndirectedMultiplex, communities [][]graph.Node, weights, resolutions []float64, all bool) *undirectedMultiplexLocalMover {
	nodes := graph.NodesOf(g.Nodes())
	l := undirectedMultiplexLocalMover{
		g:            g,
		nodes:        nodes,
		edgeWeightOf: make([][]float64, g.Depth()),
		m2:           make([]float64, g.Depth()),
		communities:  communities,
		memberships:  make([]int, len(nodes)),
		resolutions:  resolutions,
		weights:      weights,
		weight:       make([]func(xid, yid int64) float64, g.Depth()),
	}

	// Calculate the total edge weight of the graph
	// and degree weights for each node.
	var zero int
	for i := 0; i < g.Depth(); i++ {
		l.edgeWeightOf[i] = make([]float64, len(nodes))
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
		for _, u := range l.nodes {
			uid := u.ID()
			w := weight(uid, uid)
			to := layer.From(uid)
			for to.Next() {
				w += weight(uid, to.Node().ID())
			}
			l.edgeWeightOf[i][uid] = w
			l.m2[i] += w
		}
		if l.m2[i] == 0 {
			zero++
		}
	}
	if zero == g.Depth() {
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
// undirectedMultiplexLocalMover has not made any improvement to the community
// structure and so the Louvain algorithm is done.
func (l *undirectedMultiplexLocalMover) localMovingHeuristic(rnd func(int) int) (done bool) {
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
// undirectedMultiplexLocalMover using the random source rnd which should return
// an integer in the range [0,n).
func (l *undirectedMultiplexLocalMover) shuffle(rnd func(n int) int) {
	l.moved = false
	for i := range l.nodes[:len(l.nodes)-1] {
		j := i + rnd(len(l.nodes)-i)
		l.nodes[i], l.nodes[j] = l.nodes[j], l.nodes[i]
	}
}

// move moves the node at src to the community at dst.
func (l *undirectedMultiplexLocalMover) move(dst int, src commIdx) {
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
// undirectedMultiplexLocalMover's communities field is returned in src if n
// is in communities.
func (l *undirectedMultiplexLocalMover) deltaQ(n graph.Node) (deltaQ float64, dst int, src commIdx) {
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
		//  }
		//
		// This is done to avoid an allocation for
		// each layer.
		for _, layer := range l.g.layers {
			for _, vid := range layer.edges[id] {
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
			m2 := l.m2[layer]
			if m2 == 0 {
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

			var k_aC, sigma_totC float64 // C is a substitution for ^𝛼 or ^𝛽.
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

				k_aC += l.weight[layer](id, uid)
				// sigma_totC could be kept for each community
				// and updated for moves, changing the calculation
				// of sigma_totC here from O(n_c) to O(1), but
				// in practice the time savings do not appear
				// to be compelling and do not make up for the
				// increase in code complexity and space required.
				sigma_totC += l.edgeWeightOf[layer][uid]
			}

			a_aa := l.weight[layer](id, id)
			k_a := l.edgeWeightOf[layer][id]
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
				dQremove += w * (k_aC /*^𝛼*/ - a_aa - gamma*k_a*(sigma_totC /*^𝛼*/ -k_a)/m2)

			default:
				// Otherwise calculate the change due to an addition
				// to c.
				_dQadd += w * (k_aC /*^𝛽*/ - gamma*k_a*sigma_totC /*^𝛽*/ /m2)
			}
		}
		if !removal && _dQadd > dQadd {
			dQadd = _dQadd
			dst = i
		}
	}

	return 2 * (dQadd - dQremove), dst, src
}
