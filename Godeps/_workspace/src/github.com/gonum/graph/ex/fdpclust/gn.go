package main

import (
	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
)

type GraphNode struct {
	id        int
	neighbors []graph.Node
	roots     []*GraphNode
}

func (g *GraphNode) Has(n graph.Node) bool {
	if n.ID() == g.id {
		return true
	}

	visited := map[int]struct{}{g.id: struct{}{}}
	for _, root := range g.roots {
		if root.ID() == n.ID() {
			return true
		}

		if root.has(n, visited) {
			return true
		}
	}

	for _, neigh := range g.neighbors {
		if neigh.ID() == n.ID() {
			return true
		}

		if gn, ok := neigh.(*GraphNode); ok {
			if gn.has(n, visited) {
				return true
			}
		}
	}

	return false
}

func (g *GraphNode) has(n graph.Node, visited map[int]struct{}) bool {
	for _, root := range g.roots {
		if _, ok := visited[root.ID()]; ok {
			continue
		}

		visited[root.ID()] = struct{}{}
		if root.ID() == n.ID() {
			return true
		}

		if root.has(n, visited) {
			return true
		}

	}

	for _, neigh := range g.neighbors {
		if _, ok := visited[neigh.ID()]; ok {
			continue
		}

		visited[neigh.ID()] = struct{}{}
		if neigh.ID() == n.ID() {
			return true
		}

		if gn, ok := neigh.(*GraphNode); ok {
			if gn.has(n, visited) {
				return true
			}
		}

	}

	return false
}

func (g *GraphNode) Nodes() []graph.Node {
	toReturn := []graph.Node{g}
	visited := map[int]struct{}{g.id: struct{}{}}

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

	return toReturn
}

func (g *GraphNode) nodes(list []graph.Node, visited map[int]struct{}) []graph.Node {
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

func (g *GraphNode) From(n graph.Node) []graph.Node {
	if n.ID() == g.ID() {
		return g.neighbors
	}

	visited := map[int]struct{}{g.id: struct{}{}}
	for _, root := range g.roots {
		visited[root.ID()] = struct{}{}

		if result := root.findNeighbors(n, visited); result != nil {
			return result
		}
	}

	for _, neigh := range g.neighbors {
		visited[neigh.ID()] = struct{}{}

		if gn, ok := neigh.(*GraphNode); ok {
			if result := gn.findNeighbors(n, visited); result != nil {
				return result
			}
		}
	}

	return nil
}

func (g *GraphNode) findNeighbors(n graph.Node, visited map[int]struct{}) []graph.Node {
	if n.ID() == g.ID() {
		return g.neighbors
	}

	for _, root := range g.roots {
		if _, ok := visited[root.ID()]; ok {
			continue
		}
		visited[root.ID()] = struct{}{}

		if result := root.findNeighbors(n, visited); result != nil {
			return result
		}
	}

	for _, neigh := range g.neighbors {
		if _, ok := visited[neigh.ID()]; ok {
			continue
		}
		visited[neigh.ID()] = struct{}{}

		if gn, ok := neigh.(*GraphNode); ok {
			if result := gn.findNeighbors(n, visited); result != nil {
				return result
			}
		}
	}

	return nil
}

func (g *GraphNode) HasEdge(u, v graph.Node) bool {
	return g.EdgeBetween(u, v) != nil
}

func (g *GraphNode) Edge(u, v graph.Node) graph.Edge {
	return g.EdgeBetween(u, v)
}

func (g *GraphNode) EdgeBetween(u, v graph.Node) graph.Edge {
	if u.ID() == g.id || v.ID() == g.id {
		for _, neigh := range g.neighbors {
			if neigh.ID() == u.ID() || neigh.ID() == v.ID() {
				return concrete.Edge{g, neigh}
			}
		}
		return nil
	}

	visited := map[int]struct{}{g.id: struct{}{}}
	for _, root := range g.roots {
		visited[root.ID()] = struct{}{}
		if result := root.edgeBetween(u, v, visited); result != nil {
			return result
		}
	}

	for _, neigh := range g.neighbors {
		visited[neigh.ID()] = struct{}{}
		if gn, ok := neigh.(*GraphNode); ok {
			if result := gn.edgeBetween(u, v, visited); result != nil {
				return result
			}
		}
	}

	return nil
}

func (g *GraphNode) edgeBetween(u, v graph.Node, visited map[int]struct{}) graph.Edge {
	if u.ID() == g.id || v.ID() == g.id {
		for _, neigh := range g.neighbors {
			if neigh.ID() == u.ID() || neigh.ID() == v.ID() {
				return concrete.Edge{g, neigh}
			}
		}
		return nil
	}

	for _, root := range g.roots {
		if _, ok := visited[root.ID()]; ok {
			continue
		}
		visited[root.ID()] = struct{}{}
		if result := root.edgeBetween(u, v, visited); result != nil {
			return result
		}
	}

	for _, neigh := range g.neighbors {
		if _, ok := visited[neigh.ID()]; ok {
			continue
		}

		visited[neigh.ID()] = struct{}{}
		if gn, ok := neigh.(*GraphNode); ok {
			if result := gn.edgeBetween(u, v, visited); result != nil {
				return result
			}
		}
	}

	return nil
}

func (g *GraphNode) ID() int {
	return g.id
}

func (g *GraphNode) AddNeighbor(n *GraphNode) {
	g.neighbors = append(g.neighbors, graph.Node(n))
}

func (g *GraphNode) AddRoot(n *GraphNode) {
	g.roots = append(g.roots, n)
}

func NewGraphNode(id int) *GraphNode {
	return &GraphNode{id: id, neighbors: make([]graph.Node, 0), roots: make([]*GraphNode, 0)}
}
