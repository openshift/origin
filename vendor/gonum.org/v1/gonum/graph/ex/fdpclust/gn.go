// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

type GraphNode struct {
	id        int64
	neighbors []graph.Node
	roots     []*GraphNode
}

func (g *GraphNode) Node(id int64) graph.Node {
	if id == g.id {
		return g
	}

	visited := map[int64]struct{}{g.id: {}}
	for _, root := range g.roots {
		if root.ID() == id {
			return root
		}

		if root.has(id, visited) {
			return root
		}
	}

	for _, neigh := range g.neighbors {
		if neigh.ID() == id {
			return neigh
		}

		if gn, ok := neigh.(*GraphNode); ok {
			if gn.has(id, visited) {
				return gn
			}
		}
	}

	return nil
}

func (g *GraphNode) has(id int64, visited map[int64]struct{}) bool {
	for _, root := range g.roots {
		if _, ok := visited[root.ID()]; ok {
			continue
		}

		visited[root.ID()] = struct{}{}
		if root.ID() == id {
			return true
		}

		if root.has(id, visited) {
			return true
		}

	}

	for _, neigh := range g.neighbors {
		if _, ok := visited[neigh.ID()]; ok {
			continue
		}

		visited[neigh.ID()] = struct{}{}
		if neigh.ID() == id {
			return true
		}

		if gn, ok := neigh.(*GraphNode); ok {
			if gn.has(id, visited) {
				return true
			}
		}
	}

	return false
}

func (g *GraphNode) Nodes() graph.Nodes {
	toReturn := []graph.Node{g}
	visited := map[int64]struct{}{g.id: {}}

	for _, root := range g.roots {
		toReturn = append(toReturn, root)
		visited[root.ID()] = struct{}{}

		toReturn = root.nodes(toReturn, visited)
	}

	for _, neigh := range g.neighbors {
		toReturn = append(toReturn, neigh)
		visited[neigh.ID()] = struct{}{}

		if gn, ok := neigh.(*GraphNode); ok {
			toReturn = gn.nodes(toReturn, visited)
		}
	}

	return iterator.NewOrderedNodes(toReturn)
}

func (g *GraphNode) nodes(list []graph.Node, visited map[int64]struct{}) []graph.Node {
	for _, root := range g.roots {
		if _, ok := visited[root.ID()]; ok {
			continue
		}
		visited[root.ID()] = struct{}{}
		list = append(list, graph.Node(root))

		list = root.nodes(list, visited)
	}

	for _, neigh := range g.neighbors {
		if _, ok := visited[neigh.ID()]; ok {
			continue
		}

		list = append(list, neigh)
		if gn, ok := neigh.(*GraphNode); ok {
			list = gn.nodes(list, visited)
		}
	}

	return list
}

func (g *GraphNode) From(id int64) graph.Nodes {
	if id == g.ID() {
		return iterator.NewOrderedNodes(g.neighbors)
	}

	visited := map[int64]struct{}{g.id: {}}
	for _, root := range g.roots {
		visited[root.ID()] = struct{}{}

		if result := root.findNeighbors(id, visited); result != nil {
			return iterator.NewOrderedNodes(result)
		}
	}

	for _, neigh := range g.neighbors {
		visited[neigh.ID()] = struct{}{}

		if gn, ok := neigh.(*GraphNode); ok {
			if result := gn.findNeighbors(id, visited); result != nil {
				return iterator.NewOrderedNodes(result)
			}
		}
	}

	return nil
}

func (g *GraphNode) findNeighbors(id int64, visited map[int64]struct{}) []graph.Node {
	if id == g.ID() {
		return g.neighbors
	}

	for _, root := range g.roots {
		if _, ok := visited[root.ID()]; ok {
			continue
		}
		visited[root.ID()] = struct{}{}

		if result := root.findNeighbors(id, visited); result != nil {
			return result
		}
	}

	for _, neigh := range g.neighbors {
		if _, ok := visited[neigh.ID()]; ok {
			continue
		}
		visited[neigh.ID()] = struct{}{}

		if gn, ok := neigh.(*GraphNode); ok {
			if result := gn.findNeighbors(id, visited); result != nil {
				return result
			}
		}
	}

	return nil
}

func (g *GraphNode) HasEdgeBetween(uid, vid int64) bool {
	return g.EdgeBetween(uid, vid) != nil
}

func (g *GraphNode) Edge(uid, vid int64) graph.Edge {
	return g.EdgeBetween(uid, vid)
}

func (g *GraphNode) EdgeBetween(uid, vid int64) graph.Edge {
	if uid == g.id || vid == g.id {
		for _, neigh := range g.neighbors {
			if neigh.ID() == uid || neigh.ID() == vid {
				return simple.Edge{F: g, T: neigh}
			}
		}
		return nil
	}

	visited := map[int64]struct{}{g.id: {}}
	for _, root := range g.roots {
		visited[root.ID()] = struct{}{}
		if result := root.edgeBetween(uid, vid, visited); result != nil {
			return result
		}
	}

	for _, neigh := range g.neighbors {
		visited[neigh.ID()] = struct{}{}
		if gn, ok := neigh.(*GraphNode); ok {
			if result := gn.edgeBetween(uid, vid, visited); result != nil {
				return result
			}
		}
	}

	return nil
}

func (g *GraphNode) edgeBetween(uid, vid int64, visited map[int64]struct{}) graph.Edge {
	if uid == g.id || vid == g.id {
		for _, neigh := range g.neighbors {
			if neigh.ID() == uid || neigh.ID() == vid {
				return simple.Edge{F: g, T: neigh}
			}
		}
		return nil
	}

	for _, root := range g.roots {
		if _, ok := visited[root.ID()]; ok {
			continue
		}
		visited[root.ID()] = struct{}{}
		if result := root.edgeBetween(uid, vid, visited); result != nil {
			return result
		}
	}

	for _, neigh := range g.neighbors {
		if _, ok := visited[neigh.ID()]; ok {
			continue
		}

		visited[neigh.ID()] = struct{}{}
		if gn, ok := neigh.(*GraphNode); ok {
			if result := gn.edgeBetween(uid, vid, visited); result != nil {
				return result
			}
		}
	}

	return nil
}

func (g *GraphNode) ID() int64 {
	return g.id
}

func (g *GraphNode) AddNeighbor(n *GraphNode) {
	g.neighbors = append(g.neighbors, graph.Node(n))
}

func (g *GraphNode) AddRoot(n *GraphNode) {
	g.roots = append(g.roots, n)
}

func NewGraphNode(id int64) *GraphNode {
	return &GraphNode{id: id}
}
