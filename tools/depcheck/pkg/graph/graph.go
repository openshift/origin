package graph

import (
	"fmt"

	"github.com/gonum/graph/encoding/dot"
)

type Node struct {
	Id         int
	UniqueName string
	LabelName  string
}

func (n Node) ID() int {
	return n.Id
}

// DOTAttributes implements an attribute getter for the DOT encoding
func (n Node) DOTAttributes() []dot.Attribute {
	return []dot.Attribute{{Key: "label", Value: fmt.Sprintf("%q", n.LabelName)}}
}
