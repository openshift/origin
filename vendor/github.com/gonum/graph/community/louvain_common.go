// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This repository is no longer maintained.
// Development has moved to https://github.com/gonum/gonum.
//
// Package community provides graph community detection functions.
package community

import (
	"fmt"
	"math/rand"

	"github.com/gonum/graph"
)

// Q returns the modularity Q score of the graph g subdivided into the
// given communities at the given resolution. If communities is nil, the
// unclustered modularity score is returned. The resolution parameter
// is γ as defined in Reichardt and Bornholdt doi:10.1103/PhysRevE.74.016110.
// Q will panic if g has any edge with negative edge weight.
//
// If g is undirected, Q is calculated according to
//  Q = 1/2m \sum_{ij} [ A_{ij} - (\gamma k_i k_j)/2m ] \delta(c_i,c_j),
// If g is directed, it is calculated according to
//  Q = 1/m \sum_{ij} [ A_{ij} - (\gamma k_i^in k_j^out)/m ] \delta(c_i,c_j).
//
// graph.Undirect may be used as a shim to allow calculation of Q for
// directed graphs with the undirected modularity function.
func Q(g graph.Graph, communities [][]graph.Node, resolution float64) float64 {
	switch g := g.(type) {
	case graph.Undirected:
		return qUndirected(g, communities, resolution)
	case graph.Directed:
		return qDirected(g, communities, resolution)
	default:
		panic(fmt.Sprintf("community: invalid graph type: %T", g))
	}
}

// ReducedGraph is a modularised graph.
type ReducedGraph interface {
	graph.Graph

	// Communities returns the community memberships
	// of the nodes in the graph used to generate
	// the reduced graph.
	Communities() [][]graph.Node

	// Structure returns the community structure of
	// the current level of the module clustering.
	// Each slice in the returned value recursively
	// describes the membership of a community at
	// the current level by indexing via the node
	// ID into the structure of the non-nil
	// ReducedGraph returned by Expanded, or when the
	// ReducedGraph is nil, by containing nodes
	// from the original input graph.
	//
	// The returned value should not be mutated.
	Structure() [][]graph.Node

	// Expanded returns the next lower level of the
	// module clustering or nil if at the lowest level.
	//
	// The returned ReducedGraph will be the same
	// concrete type as the receiver.
	Expanded() ReducedGraph
}

// Modularize returns the hierarchical modularization of g at the given resolution
// using the Louvain algorithm. If src is nil, rand.Intn is used as the random
// generator. Modularize will panic if g has any edge with negative edge weight.
//
// If g is undirected it is modularised to minimise
//  Q = 1/2m \sum_{ij} [ A_{ij} - (\gamma k_i k_j)/2m ] \delta(c_i,c_j),
// If g is directed it is modularised to minimise
//  Q = 1/m \sum_{ij} [ A_{ij} - (\gamma k_i^in k_j^out)/m ] \delta(c_i,c_j).
//
// The concrete type of the ReducedGraph will be a pointer to either a
// ReducedUndirected or a ReducedDirected depending on the type of g.
//
// graph.Undirect may be used as a shim to allow modularization of
// directed graphs with the undirected modularity function.
func Modularize(g graph.Graph, resolution float64, src *rand.Rand) ReducedGraph {
	switch g := g.(type) {
	case graph.Undirected:
		return louvainUndirected(g, resolution, src)
	case graph.Directed:
		return louvainDirected(g, resolution, src)
	default:
		panic(fmt.Sprintf("community: invalid graph type: %T", g))
	}
}

// Multiplex is a multiplex graph.
type Multiplex interface {
	// Nodes returns the slice of nodes
	// for the multiplex graph.
	// All layers must refer to the same
	// set of nodes.
	Nodes() []graph.Node

	// Depth returns the number of layers
	// in the multiplex graph.
	Depth() int
}

// QMultiplex returns the modularity Q score of the multiplex graph layers
// subdivided into the given communities at the given resolutions and weights. Q is
// returned as the vector of weighted Q scores for each layer of the multiplex graph.
// If communities is nil, the unclustered modularity score is returned.
// If weights is nil layers are equally weighted, otherwise the length of
// weights must equal the number of layers. If resolutions is nil, a resolution
// of 1.0 is used for all layers, otherwise either a single element slice may be used
// to specify a global resolution, or the length of resolutions must equal the number
// of layers. The resolution parameter is γ as defined in Reichardt and Bornholdt
// doi:10.1103/PhysRevE.74.016110.
// QMultiplex will panic if the graph has any layer weight-scaled edge with
// negative edge weight.
//
// If g is undirected, Q is calculated according to
//  Q_{layer} = w_{layer} \sum_{ij} [ A_{layer}*_{ij} - (\gamma_{layer} k_i k_j)/2m_{layer} ] \delta(c_i,c_j),
// If g is directed, it is calculated according to
//  Q_{layer} = w_{layer} \sum_{ij} [ A_{layer}*_{ij} - (\gamma_{layer} k_i^in k_j^out)/m_{layer} ] \delta(c_i,c_j).
//
// Note that Q values for multiplex graphs are not scaled by the total layer edge weight.
//
// graph.Undirect may be used as a shim to allow calculation of Q for
// directed graphs.
func QMultiplex(g Multiplex, communities [][]graph.Node, weights, resolutions []float64) []float64 {
	if weights != nil && len(weights) != g.Depth() {
		panic("community: weights vector length mismatch")
	}
	if resolutions != nil && len(resolutions) != 1 && len(resolutions) != g.Depth() {
		panic("community: resolutions vector length mismatch")
	}

	switch g := g.(type) {
	case UndirectedMultiplex:
		return qUndirectedMultiplex(g, communities, weights, resolutions)
	case DirectedMultiplex:
		return qDirectedMultiplex(g, communities, weights, resolutions)
	default:
		panic(fmt.Sprintf("community: invalid graph type: %T", g))
	}
}

// ReducedMultiplex is a modularised multiplex graph.
type ReducedMultiplex interface {
	Multiplex

	// Communities returns the community memberships
	// of the nodes in the graph used to generate
	// the reduced graph.
	Communities() [][]graph.Node

	// Structure returns the community structure of
	// the current level of the module clustering.
	// Each slice in the returned value recursively
	// describes the membership of a community at
	// the current level by indexing via the node
	// ID into the structure of the non-nil
	// ReducedGraph returned by Expanded, or when the
	// ReducedGraph is nil, by containing nodes
	// from the original input graph.
	//
	// The returned value should not be mutated.
	Structure() [][]graph.Node

	// Expanded returns the next lower level of the
	// module clustering or nil if at the lowest level.
	//
	// The returned ReducedGraph will be the same
	// concrete type as the receiver.
	Expanded() ReducedMultiplex
}

// ModularizeMultiplex returns the hierarchical modularization of g at the given resolution
// using the Louvain algorithm. If all is true and g have negatively weighted layers, all
// communities will be searched during the modularization. If src is nil, rand.Intn is
// used as the random generator. ModularizeMultiplex will panic if g has any edge with
// edge weight that does not sign-match the layer weight.
//
// If g is undirected it is modularised to minimise
//  Q = \sum w_{layer} \sum_{ij} [ A_{layer}*_{ij} - (\gamma_{layer} k_i k_j)/2m ] \delta(c_i,c_j).
// If g is directed it is modularised to minimise
//  Q = \sum w_{layer} \sum_{ij} [ A_{layer}*_{ij} - (\gamma_{layer} k_i^in k_j^out)/m_{layer} ] \delta(c_i,c_j).
//
// The concrete type of the ReducedMultiplex will be a pointer to a
// ReducedUndirectedMultiplex.
//
// graph.Undirect may be used as a shim to allow modularization of
// directed graphs with the undirected modularity function.
func ModularizeMultiplex(g Multiplex, weights, resolutions []float64, all bool, src *rand.Rand) ReducedMultiplex {
	if weights != nil && len(weights) != g.Depth() {
		panic("community: weights vector length mismatch")
	}
	if resolutions != nil && len(resolutions) != 1 && len(resolutions) != g.Depth() {
		panic("community: resolutions vector length mismatch")
	}

	switch g := g.(type) {
	case UndirectedMultiplex:
		return louvainUndirectedMultiplex(g, weights, resolutions, all, src)
	case DirectedMultiplex:
		return louvainDirectedMultiplex(g, weights, resolutions, all, src)
	default:
		panic(fmt.Sprintf("community: invalid graph type: %T", g))
	}
}

// undirectedEdges is the edge structure of a reduced undirected graph.
type undirectedEdges struct {
	// edges and weights is the set
	// of edges between nodes.
	// weights is keyed such that
	// the first element of the key
	// is less than the second.
	edges   [][]int
	weights map[[2]int]float64
}

// directedEdges is the edge structure of a reduced directed graph.
type directedEdges struct {
	// edgesFrom, edgesTo and weights
	// is the set of edges between nodes.
	edgesFrom [][]int
	edgesTo   [][]int
	weights   map[[2]int]float64
}

// community is a reduced graph node describing its membership.
type community struct {
	id int

	nodes []graph.Node

	weight float64
}

func (n community) ID() int { return n.id }

// edge is a reduced graph edge.
type edge struct {
	from, to community
	weight   float64
}

func (e edge) From() graph.Node { return e.from }
func (e edge) To() graph.Node   { return e.to }
func (e edge) Weight() float64  { return e.weight }

// multiplexCommunity is a reduced multiplex graph node describing its membership.
type multiplexCommunity struct {
	id int

	nodes []graph.Node

	weights []float64
}

func (n multiplexCommunity) ID() int { return n.id }

// multiplexEdge is a reduced graph edge for a multiplex graph.
type multiplexEdge struct {
	from, to multiplexCommunity
	weight   float64
}

func (e multiplexEdge) From() graph.Node { return e.from }
func (e multiplexEdge) To() graph.Node   { return e.to }
func (e multiplexEdge) Weight() float64  { return e.weight }

// commIdx is an index of a node in a community held by a localMover.
type commIdx struct {
	community int
	node      int
}

// node is defined to avoid an import of .../graph/simple.
type node int

func (n node) ID() int { return int(n) }

// minTaker is a set iterator.
type minTaker interface {
	TakeMin(p *int) bool
}

// dense is a dense integer set iterator.
type dense struct {
	pos int
	n   int
}

// TakeMin mimics intsets.Sparse TakeMin for dense sets. If the dense
// iterator position is less than the iterator size, TakeMin sets *p
// to the the iterator position and increments the position and returns
// true.
// Otherwise, it returns false and *p is undefined.
func (d *dense) TakeMin(p *int) bool {
	if d.pos >= d.n {
		return false
	}
	*p = d.pos
	d.pos++
	return true
}

const (
	negativeWeight = "community: unexpected negative edge weight"
	positiveWeight = "community: unexpected positive edge weight"
)

// positiveWeightFuncFor returns a constructed weight function for the
// positively weighted g.
func positiveWeightFuncFor(g graph.Graph) func(x, y graph.Node) float64 {
	if wg, ok := g.(graph.Weighter); ok {
		return func(x, y graph.Node) float64 {
			w, ok := wg.Weight(x, y)
			if !ok {
				return 0
			}
			if w < 0 {
				panic(negativeWeight)
			}
			return w
		}
	}
	return func(x, y graph.Node) float64 {
		e := g.Edge(x, y)
		if e == nil {
			return 0
		}
		w := e.Weight()
		if w < 0 {
			panic(negativeWeight)
		}
		return w
	}
}

// negativeWeightFuncFor returns a constructed weight function for the
// negatively weighted g.
func negativeWeightFuncFor(g graph.Graph) func(x, y graph.Node) float64 {
	if wg, ok := g.(graph.Weighter); ok {
		return func(x, y graph.Node) float64 {
			w, ok := wg.Weight(x, y)
			if !ok {
				return 0
			}
			if w > 0 {
				panic(positiveWeight)
			}
			return -w
		}
	}
	return func(x, y graph.Node) float64 {
		e := g.Edge(x, y)
		if e == nil {
			return 0
		}
		w := e.Weight()
		if w > 0 {
			panic(positiveWeight)
		}
		return -w
	}
}

// depth returns max(1, len(weights)). It is used to ensure
// that multiplex community weights are properly initialised.
func depth(weights []float64) int {
	if weights == nil {
		return 1
	}
	return len(weights)
}
