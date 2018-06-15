package graph

import (
	"fmt"
	"strings"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
	"github.com/gonum/graph/encoding/dot"
)

type Node struct {
	Id         int
	UniqueName string
	Color      string
}

func (n Node) ID() int {
	return n.Id
}

func (n Node) String() string {
	return labelNameForNode(n.UniqueName)
}

// DOTAttributes implements an attribute getter for the DOT encoding
func (n Node) DOTAttributes() []dot.Attribute {
	color := n.Color
	if len(color) == 0 {
		color = "black"
	}

	return []dot.Attribute{
		{Key: "label", Value: fmt.Sprintf("%q", n)},
		{Key: "color", Value: color},
	}
}

// labelNameForNode trims vendored paths of their full /vendor/ path
func labelNameForNode(importPath string) string {
	segs := strings.Split(importPath, "/vendor/")
	if len(segs) > 1 {
		return segs[1]
	}

	return importPath
}

func NewMutableDirectedGraph(roots []string) *MutableDirectedGraph {
	return &MutableDirectedGraph{
		DirectedGraph: concrete.NewDirectedGraph(),
		nodesByName:   make(map[string]graph.Node),
		rootNodeNames: roots,
	}
}

type MutableDirectedGraph struct {
	*concrete.DirectedGraph

	nodesByName   map[string]graph.Node
	rootNodeNames []string
}

func (g *MutableDirectedGraph) AddNode(n *Node) error {
	if _, exists := g.nodesByName[n.UniqueName]; exists {
		return fmt.Errorf("node .UniqueName collision: %s", n.UniqueName)
	}

	g.nodesByName[n.UniqueName] = n
	g.DirectedGraph.AddNode(n)
	return nil
}

// RemoveNode deletes the given node from the graph,
// removing all of its outbound and inbound edges as well.
func (g *MutableDirectedGraph) RemoveNode(n *Node) {
	delete(g.nodesByName, n.UniqueName)
	g.DirectedGraph.RemoveNode(n)
}

func (g *MutableDirectedGraph) NodeByName(name string) (graph.Node, bool) {
	n, exists := g.nodesByName[name]
	return n, exists && g.DirectedGraph.Has(n)
}

// PruneOrphans recursively removes nodes with no inbound edges.
// Nodes marked as root nodes are ignored.
// Returns a list of recursively pruned nodes.
func (g *MutableDirectedGraph) PruneOrphans() []*Node {
	removed := []*Node{}

	for _, n := range g.nodesByName {
		node, ok := n.(*Node)
		if !ok {
			continue
		}
		if len(g.To(n)) > 0 {
			continue
		}
		if contains(node.UniqueName, g.rootNodeNames) {
			continue
		}

		g.RemoveNode(node)
		removed = append(removed, node)
	}

	if len(removed) == 0 {
		return []*Node{}
	}

	return append(removed, g.PruneOrphans()...)
}

func contains(needle string, haystack []string) bool {
	for _, str := range haystack {
		if needle == str {
			return true
		}
	}

	return false
}

// Copy creates a new graph instance, preserving all of the nodes and edges
// from the original graph. The nodes themselves are shallow copies from the
// original graph.
func (g *MutableDirectedGraph) Copy() *MutableDirectedGraph {
	newGraph := NewMutableDirectedGraph(g.rootNodeNames)

	// copy nodes
	for _, n := range g.Nodes() {
		newNode, ok := n.(*Node)
		if !ok {
			continue
		}

		if err := newGraph.AddNode(newNode); err != nil {
			// this should never happen: the only error that could occur is a node name collision,
			// which would imply that the original graph that we are copying had node-name collisions.
			panic(fmt.Errorf("unexpected error while copying graph: %v", err))
		}
	}

	// copy edges
	for _, n := range g.Nodes() {
		node, ok := n.(*Node)
		if !ok {
			continue
		}

		if _, exists := newGraph.NodeByName(node.UniqueName); !exists {
			continue
		}

		from := g.From(n)
		for _, to := range from {
			if newGraph.HasEdgeFromTo(n, to) {
				continue
			}

			newGraph.SetEdge(concrete.Edge{
				F: n,
				T: to,
			}, 0)
		}
	}

	return newGraph
}
