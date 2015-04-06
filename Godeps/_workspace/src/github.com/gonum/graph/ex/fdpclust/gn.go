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

func (g *GraphNode) NodeExists(n graph.Node) bool {
	if n.ID() == g.id {
		return true
	}

	visited := map[int]struct{}{g.id: struct{}{}}
	for _, root := range g.roots {
		if root.ID() == n.ID() {
			return true
		}

		if root.nodeExists(n, visited) {
			return true
		}
	}

	for _, neigh := range g.neighbors {
		if neigh.ID() == n.ID() {
			return true
		}

		if gn, ok := neigh.(*GraphNode); ok {
			if gn.nodeExists(n, visited) {
				return true
			}
		}
	}

	return false
}

func (g *GraphNode) nodeExists(n graph.Node, visited map[int]struct{}) bool {
	for _, root := range g.roots {
		if _, ok := visited[root.ID()]; ok {
			continue
		}

		visited[root.ID()] = struct{}{}
		if root.ID() == n.ID() {
			return true
		}

		if root.nodeExists(n, visited) {
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
			if gn.nodeExists(n, visited) {
				return true
			}
		}

	}

	return false
}

func (g *GraphNode) NodeList() []graph.Node {
	toReturn := []graph.Node{g}
	visited := map[int]struct{}{g.id: struct{}{}}

	for _, root := range g.roots {
		toReturn = append(toReturn, root)
		visited[root.ID()] = struct{}{}

		toReturn = root.nodeList(toReturn, visited)
	}

	for _, neigh := range g.neighbors {
		toReturn = append(toReturn, neigh)
		visited[neigh.ID()] = struct{}{}

		if gn, ok := neigh.(*GraphNode); ok {
			toReturn = gn.nodeList(toReturn, visited)
		}
	}

	return toReturn
}

func (g *GraphNode) nodeList(list []graph.Node, visited map[int]struct{}) []graph.Node {
	for _, root := range g.roots {
		if _, ok := visited[root.ID()]; ok {
			continue
		}
		visited[root.ID()] = struct{}{}
		list = append(list, graph.Node(root))

		list = root.nodeList(list, visited)
	}

	for _, neigh := range g.neighbors {
		if _, ok := visited[neigh.ID()]; ok {
			continue
		}

		list = append(list, neigh)
		if gn, ok := neigh.(*GraphNode); ok {
			list = gn.nodeList(list, visited)
		}
	}

	return list
}

func (g *GraphNode) Neighbors(n graph.Node) []graph.Node {
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

func (g *GraphNode) EdgeBetween(n, neighbor graph.Node) graph.Edge {
	if n.ID() == g.id || neighbor.ID() == g.id {
		for _, neigh := range g.neighbors {
			if neigh.ID() == n.ID() || neigh.ID() == neighbor.ID() {
				return concrete.Edge{g, neigh}
			}
		}

		return nil
	}

	visited := map[int]struct{}{g.id: struct{}{}}
	for _, root := range g.roots {
		visited[root.ID()] = struct{}{}
		if result := root.edgeBetween(n, neighbor, visited); result != nil {
			return result
		}
	}

	for _, neigh := range g.neighbors {
		visited[neigh.ID()] = struct{}{}
		if gn, ok := neigh.(*GraphNode); ok {
			if result := gn.edgeBetween(n, neighbor, visited); result != nil {
				return result
			}
		}
	}

	return nil
}

func (g *GraphNode) edgeBetween(n, neighbor graph.Node, visited map[int]struct{}) graph.Edge {
	if n.ID() == g.id || neighbor.ID() == g.id {
		for _, neigh := range g.neighbors {
			if neigh.ID() == n.ID() || neigh.ID() == neighbor.ID() {
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
		if result := root.edgeBetween(n, neighbor, visited); result != nil {
			return result
		}
	}

	for _, neigh := range g.neighbors {
		if _, ok := visited[neigh.ID()]; ok {
			continue
		}

		visited[neigh.ID()] = struct{}{}
		if gn, ok := neigh.(*GraphNode); ok {
			if result := gn.edgeBetween(n, neighbor, visited); result != nil {
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
