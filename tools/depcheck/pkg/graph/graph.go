package graph

import (
	"fmt"

	"github.com/gonum/graph"
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

type MutableDirectedGraph interface {
	graph.Directed

	AddNode(graph.Node)

	RemoveEdge(graph.Edge)
	RemoveNode(graph.Node)
}
