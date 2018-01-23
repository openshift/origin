package graph

import (
	"fmt"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
	"github.com/gonum/graph/encoding/dot"
)

type Node struct {
	Id         int
	UniqueName string
	LabelName  string
	Color      string
}

func (n Node) ID() int {
	return n.Id
}

// DOTAttributes implements an attribute getter for the DOT encoding
func (n Node) DOTAttributes() []dot.Attribute {
	color := n.Color
	if len(color) == 0 {
		color = "black"
	}

	return []dot.Attribute{
		{Key: "label", Value: fmt.Sprintf("%q", n.LabelName)},
		{Key: "color", Value: color},
	}
}

func NewMutableDirectedGraph(g *concrete.DirectedGraph) *MutableDirectedGraph {
	return &MutableDirectedGraph{
		DirectedGraph: concrete.NewDirectedGraph(),
		nodesByName:   make(map[string]graph.Node),
	}
}

type MutableDirectedGraph struct {
	*concrete.DirectedGraph

	nodesByName map[string]graph.Node
}

func (g *MutableDirectedGraph) AddNode(n *Node) {
	g.nodesByName[n.UniqueName] = n
	g.DirectedGraph.AddNode(n)
}

func (g *MutableDirectedGraph) NodeByName(name string) (graph.Node, bool) {
	n, exists := g.nodesByName[name]
	return n, exists && g.DirectedGraph.Has(n)
}
