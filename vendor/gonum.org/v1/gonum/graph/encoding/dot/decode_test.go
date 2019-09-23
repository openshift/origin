// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dot

import (
	"fmt"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding"
	"gonum.org/v1/gonum/graph/simple"
)

func TestRoundTrip(t *testing.T) {
	golden := []struct {
		want     string
		directed bool
	}{
		{
			want:     directed,
			directed: true,
		},
		{
			want:     undirected,
			directed: false,
		},
		{
			want:     directedID,
			directed: true,
		},
		{
			want:     undirectedID,
			directed: false,
		},
		{
			want:     directedWithPorts,
			directed: true,
		},
		{
			want:     undirectedWithPorts,
			directed: false,
		},
	}
	for i, g := range golden {
		var dst encoding.Builder
		if g.directed {
			dst = newDotDirectedGraph()
		} else {
			dst = newDotUndirectedGraph()
		}
		data := []byte(g.want)
		if err := Unmarshal(data, dst); err != nil {
			t.Errorf("i=%d: unable to unmarshal DOT graph; %v", i, err)
			continue
		}
		buf, err := Marshal(dst, "", "", "\t", false)
		if err != nil {
			t.Errorf("i=%d: unable to marshal graph; %v", i, dst)
			continue
		}
		got := string(buf)
		if got != g.want {
			t.Errorf("i=%d: graph content mismatch; want:\n%s\n\ngot:\n%s", i, g.want, got)
			continue
		}
	}
}

const directed = `digraph {
	graph [
		outputorder=edgesfirst
	];
	node [
		shape=circle
		style=filled
	];
	edge [
		penwidth=5
		color=gray
	];

	// Node definitions.
	A [label="foo 2"];
	B [label="bar 2"];

	// Edge definitions.
	A -> B [label="baz 2"];
}`

const undirected = `graph {
	graph [
		outputorder=edgesfirst
	];
	node [
		shape=circle
		style=filled
	];
	edge [
		penwidth=5
		color=gray
	];

	// Node definitions.
	A [label="foo 2"];
	B [label="bar 2"];

	// Edge definitions.
	A -- B [label="baz 2"];
}`

const directedID = `digraph G {
	// Node definitions.
	A;
	B;

	// Edge definitions.
	A -> B;
}`

const undirectedID = `graph H {
	// Node definitions.
	A;
	B;

	// Edge definitions.
	A -- B;
}`

const directedWithPorts = `digraph {
	// Node definitions.
	A;
	B;
	C;
	D;
	E;
	F;

	// Edge definitions.
	A:foo -> B:bar;
	A -> C:bar;
	B:foo -> C;
	D:foo:n -> E:bar:s;
	D:e -> F:bar:w;
	E:_ -> F:c;
}`

const undirectedWithPorts = `graph {
	// Node definitions.
	A;
	B;
	C;
	D;
	E;
	F;

	// Edge definitions.
	A:foo -- B:bar;
	A -- C:bar;
	B:foo -- C;
	D:foo:n -- E:bar:s;
	D:e -- F:bar:w;
	E:_ -- F:c;
}`

func TestChainedEdgeAttributes(t *testing.T) {
	golden := []struct {
		in, want string
		directed bool
	}{
		{
			in:       directedChained,
			want:     directedNonchained,
			directed: true,
		},
		{
			in:       undirectedChained,
			want:     undirectedNonchained,
			directed: false,
		},
	}
	for i, g := range golden {
		var dst encoding.Builder
		if g.directed {
			dst = newDotDirectedGraph()
		} else {
			dst = newDotUndirectedGraph()
		}
		data := []byte(g.in)
		if err := Unmarshal(data, dst); err != nil {
			t.Errorf("i=%d: unable to unmarshal DOT graph; %v", i, err)
			continue
		}
		buf, err := Marshal(dst, "", "", "\t", false)
		if err != nil {
			t.Errorf("i=%d: unable to marshal graph; %v", i, dst)
			continue
		}
		got := string(buf)
		if got != g.want {
			t.Errorf("i=%d: graph content mismatch; want:\n%s\n\ngot:\n%s", i, g.want, got)
			continue
		}
	}
}

const directedChained = `digraph {
	graph [
		outputorder=edgesfirst
	];
	node [
		shape=circle
		style=filled
	];
	edge [
		penwidth=5
		color=gray
	];

	// Node definitions.
	A [label="foo 2"];
	B [label="bar 2"];

	// Edge definitions.
	A -> B -> A [label="baz 2"];
}`

const directedNonchained = `digraph {
	graph [
		outputorder=edgesfirst
	];
	node [
		shape=circle
		style=filled
	];
	edge [
		penwidth=5
		color=gray
	];

	// Node definitions.
	A [label="foo 2"];
	B [label="bar 2"];

	// Edge definitions.
	A -> B [label="baz 2"];
	B -> A [label="baz 2"];
}`

const undirectedChained = `graph {
	graph [
		outputorder=edgesfirst
	];
	node [
		shape=circle
		style=filled
	];
	edge [
		penwidth=5
		color=gray
	];

	// Node definitions.
	A [label="foo 2"];
	B [label="bar 2"];
	C [label="bif 2"];

	// Edge definitions.
	A -- B -- C [label="baz 2"];
}`

const undirectedNonchained = `graph {
	graph [
		outputorder=edgesfirst
	];
	node [
		shape=circle
		style=filled
	];
	edge [
		penwidth=5
		color=gray
	];

	// Node definitions.
	A [label="foo 2"];
	B [label="bar 2"];
	C [label="bif 2"];

	// Edge definitions.
	A -- B [label="baz 2"];
	B -- C [label="baz 2"];
}`

// Below follows a minimal implementation of a graph capable of validating the
// round-trip encoding and decoding of DOT graphs with nodes and edges
// containing DOT attributes.

// dotDirectedGraph extends simple.DirectedGraph to add NewNode and NewEdge
// methods for creating user-defined nodes and edges.
//
// dotDirectedGraph implements the encoding.Builder and the dot.Graph
// interfaces.
type dotDirectedGraph struct {
	*simple.DirectedGraph
	id                string
	graph, node, edge attributes
}

// newDotDirectedGraph returns a new directed capable of creating user-defined
// nodes and edges.
func newDotDirectedGraph() *dotDirectedGraph {
	return &dotDirectedGraph{DirectedGraph: simple.NewDirectedGraph()}
}

// NewNode returns a new node with a unique node ID for the graph.
func (g *dotDirectedGraph) NewNode() graph.Node {
	return &dotNode{Node: g.DirectedGraph.NewNode()}
}

// NewEdge returns a new Edge from the source to the destination node.
func (g *dotDirectedGraph) NewEdge(from, to graph.Node) graph.Edge {
	return &dotEdge{Edge: g.DirectedGraph.NewEdge(from, to)}
}

// DOTAttributers implements the dot.Attributers interface.
func (g *dotDirectedGraph) DOTAttributers() (graph, node, edge encoding.Attributer) {
	return g.graph, g.node, g.edge
}

// DOTAttributeSetters implements the dot.AttributeSetters interface.
func (g *dotDirectedGraph) DOTAttributeSetters() (graph, node, edge encoding.AttributeSetter) {
	return &g.graph, &g.node, &g.edge
}

// SetDOTID sets the DOT ID of the graph.
func (g *dotDirectedGraph) SetDOTID(id string) {
	g.id = id
}

// DOTID returns the DOT ID of the graph.
func (g *dotDirectedGraph) DOTID() string {
	return g.id
}

// dotUndirectedGraph extends simple.UndirectedGraph to add NewNode and NewEdge
// methods for creating user-defined nodes and edges.
//
// dotUndirectedGraph implements the encoding.Builder and the dot.Graph
// interfaces.
type dotUndirectedGraph struct {
	*simple.UndirectedGraph
	id                string
	graph, node, edge attributes
}

// newDotUndirectedGraph returns a new undirected capable of creating user-
// defined nodes and edges.
func newDotUndirectedGraph() *dotUndirectedGraph {
	return &dotUndirectedGraph{UndirectedGraph: simple.NewUndirectedGraph()}
}

// NewNode adds a new node with a unique node ID to the graph.
func (g *dotUndirectedGraph) NewNode() graph.Node {
	return &dotNode{Node: g.UndirectedGraph.NewNode()}
}

// NewEdge returns a new Edge from the source to the destination node.
func (g *dotUndirectedGraph) NewEdge(from, to graph.Node) graph.Edge {
	return &dotEdge{Edge: g.UndirectedGraph.NewEdge(from, to)}
}

// DOTAttributers implements the dot.Attributers interface.
func (g *dotUndirectedGraph) DOTAttributers() (graph, node, edge encoding.Attributer) {
	return g.graph, g.node, g.edge
}

// DOTUnmarshalerAttrs implements the dot.UnmarshalerAttrs interface.
func (g *dotUndirectedGraph) DOTAttributeSetters() (graph, node, edge encoding.AttributeSetter) {
	return &g.graph, &g.node, &g.edge
}

// SetDOTID sets the DOT ID of the graph.
func (g *dotUndirectedGraph) SetDOTID(id string) {
	g.id = id
}

// DOTID returns the DOT ID of the graph.
func (g *dotUndirectedGraph) DOTID() string {
	return g.id
}

// dotNode extends simple.Node with a label field to test round-trip encoding
// and decoding of node DOT label attributes.
type dotNode struct {
	graph.Node
	dotID string
	// Node label.
	Label string
}

// DOTID returns the node's DOT ID.
func (n *dotNode) DOTID() string {
	return n.dotID
}

// SetDOTID sets a DOT ID.
func (n *dotNode) SetDOTID(id string) {
	n.dotID = id
}

// SetAttribute sets a DOT attribute.
func (n *dotNode) SetAttribute(attr encoding.Attribute) error {
	if attr.Key != "label" {
		return fmt.Errorf("unable to unmarshal node DOT attribute with key %q", attr.Key)
	}
	n.Label = attr.Value
	return nil
}

// Attributes returns the DOT attributes of the node.
func (n *dotNode) Attributes() []encoding.Attribute {
	if len(n.Label) == 0 {
		return nil
	}
	return []encoding.Attribute{{
		Key:   "label",
		Value: n.Label,
	}}
}

type dotPortLabels struct {
	Port, Compass string
}

// dotEdge extends simple.Edge with a label field to test round-trip encoding and
// decoding of edge DOT label attributes.
type dotEdge struct {
	graph.Edge
	// Edge label.
	Label          string
	FromPortLabels dotPortLabels
	ToPortLabels   dotPortLabels
}

// SetAttribute sets a DOT attribute.
func (e *dotEdge) SetAttribute(attr encoding.Attribute) error {
	if attr.Key != "label" {
		return fmt.Errorf("unable to unmarshal node DOT attribute with key %q", attr.Key)
	}
	e.Label = attr.Value
	return nil
}

// Attributes returns the DOT attributes of the edge.
func (e *dotEdge) Attributes() []encoding.Attribute {
	if len(e.Label) == 0 {
		return nil
	}
	return []encoding.Attribute{{
		Key:   "label",
		Value: e.Label,
	}}
}

func (e *dotEdge) SetFromPort(port, compass string) error {
	e.FromPortLabels.Port = port
	e.FromPortLabels.Compass = compass
	return nil
}

func (e *dotEdge) SetToPort(port, compass string) error {
	e.ToPortLabels.Port = port
	e.ToPortLabels.Compass = compass
	return nil
}

func (e *dotEdge) FromPort() (port, compass string) {
	return e.FromPortLabels.Port, e.FromPortLabels.Compass
}

func (e *dotEdge) ToPort() (port, compass string) {
	return e.ToPortLabels.Port, e.ToPortLabels.Compass
}

// attributes is a helper for global attributes.
type attributes []encoding.Attribute

func (a attributes) Attributes() []encoding.Attribute {
	return []encoding.Attribute(a)
}
func (a *attributes) SetAttribute(attr encoding.Attribute) error {
	*a = append(*a, attr)
	return nil
}
