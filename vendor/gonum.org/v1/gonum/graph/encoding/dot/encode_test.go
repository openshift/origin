// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dot

import (
	"bytes"
	"os/exec"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding"
	"gonum.org/v1/gonum/graph/multi"
	"gonum.org/v1/gonum/graph/simple"
)

// intset is an integer set.
type intset map[int64]struct{}

func linksTo(i ...int64) intset {
	if len(i) == 0 {
		return nil
	}
	s := make(intset)
	for _, v := range i {
		s[v] = struct{}{}
	}
	return s
}

var (
	// Example graph from http://en.wikipedia.org/wiki/File:PageRanks-Example.svg 16:17, 8 July 2009
	// Node identities are rewritten here to use integers from 0 to match with the DOT output.
	pageRankGraph = []intset{
		0:  nil,
		1:  linksTo(2),
		2:  linksTo(1),
		3:  linksTo(0, 1),
		4:  linksTo(3, 1, 5),
		5:  linksTo(1, 4),
		6:  linksTo(1, 4),
		7:  linksTo(1, 4),
		8:  linksTo(1, 4),
		9:  linksTo(4),
		10: linksTo(4),
	}

	// Example graph from http://en.wikipedia.org/w/index.php?title=PageRank&oldid=659286279#Power_Method
	powerMethodGraph = []intset{
		0: linksTo(1, 2),
		1: linksTo(3),
		2: linksTo(3, 4),
		3: linksTo(4),
		4: linksTo(0),
	}
)

func directedGraphFrom(g []intset) graph.Directed {
	dg := simple.NewDirectedGraph()
	for u, e := range g {
		for v := range e {
			dg.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
		}
	}
	return dg
}

func undirectedGraphFrom(g []intset) graph.Graph {
	dg := simple.NewUndirectedGraph()
	for u, e := range g {
		for v := range e {
			dg.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
		}
	}
	return dg
}

const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

type namedNode struct {
	id   int64
	name string
}

func (n namedNode) ID() int64     { return n.id }
func (n namedNode) DOTID() string { return n.name }

func directedNamedIDGraphFrom(g []intset) graph.Directed {
	dg := simple.NewDirectedGraph()
	for u, e := range g {
		u := int64(u)
		nu := namedNode{id: u, name: alpha[u : u+1]}
		for v := range e {
			nv := namedNode{id: v, name: alpha[v : v+1]}
			dg.SetEdge(simple.Edge{F: nu, T: nv})
		}
	}
	return dg
}

func undirectedNamedIDGraphFrom(g []intset) graph.Graph {
	dg := simple.NewUndirectedGraph()
	for u, e := range g {
		u := int64(u)
		nu := namedNode{id: u, name: alpha[u : u+1]}
		for v := range e {
			nv := namedNode{id: v, name: alpha[v : v+1]}
			dg.SetEdge(simple.Edge{F: nu, T: nv})
		}
	}
	return dg
}

type attrNode struct {
	id   int64
	name string
	attr []encoding.Attribute
}

func (n attrNode) ID() int64                        { return n.id }
func (n attrNode) Attributes() []encoding.Attribute { return n.attr }

func directedNodeAttrGraphFrom(g []intset, attr [][]encoding.Attribute) graph.Directed {
	dg := simple.NewDirectedGraph()
	for u, e := range g {
		u := int64(u)
		var at []encoding.Attribute
		if u < int64(len(attr)) {
			at = attr[u]
		}
		nu := attrNode{id: u, attr: at}
		for v := range e {
			if v < int64(len(attr)) {
				at = attr[v]
			}
			nv := attrNode{id: v, attr: at}
			dg.SetEdge(simple.Edge{F: nu, T: nv})
		}
	}
	return dg
}

func undirectedNodeAttrGraphFrom(g []intset, attr [][]encoding.Attribute) graph.Graph {
	dg := simple.NewUndirectedGraph()
	for u, e := range g {
		u := int64(u)
		var at []encoding.Attribute
		if u < int64(len(attr)) {
			at = attr[u]
		}
		nu := attrNode{id: u, attr: at}
		for v := range e {
			if v < int64(len(attr)) {
				at = attr[v]
			}
			nv := attrNode{id: v, attr: at}
			dg.SetEdge(simple.Edge{F: nu, T: nv})
		}
	}
	return dg
}

type namedAttrNode struct {
	id   int64
	name string
	attr []encoding.Attribute
}

func (n namedAttrNode) ID() int64                        { return n.id }
func (n namedAttrNode) DOTID() string                    { return n.name }
func (n namedAttrNode) Attributes() []encoding.Attribute { return n.attr }

func directedNamedIDNodeAttrGraphFrom(g []intset, attr [][]encoding.Attribute) graph.Directed {
	dg := simple.NewDirectedGraph()
	for u, e := range g {
		u := int64(u)
		var at []encoding.Attribute
		if u < int64(len(attr)) {
			at = attr[u]
		}
		nu := namedAttrNode{id: u, name: alpha[u : u+1], attr: at}
		for v := range e {
			if v < int64(len(attr)) {
				at = attr[v]
			}
			nv := namedAttrNode{id: v, name: alpha[v : v+1], attr: at}
			dg.SetEdge(simple.Edge{F: nu, T: nv})
		}
	}
	return dg
}

func undirectedNamedIDNodeAttrGraphFrom(g []intset, attr [][]encoding.Attribute) graph.Graph {
	dg := simple.NewUndirectedGraph()
	for u, e := range g {
		u := int64(u)
		var at []encoding.Attribute
		if u < int64(len(attr)) {
			at = attr[u]
		}
		nu := namedAttrNode{id: u, name: alpha[u : u+1], attr: at}
		for v := range e {
			if v < int64(len(attr)) {
				at = attr[v]
			}
			nv := namedAttrNode{id: v, name: alpha[v : v+1], attr: at}
			dg.SetEdge(simple.Edge{F: nu, T: nv})
		}
	}
	return dg
}

type attrEdge struct {
	from, to graph.Node

	attr []encoding.Attribute
}

func (e attrEdge) From() graph.Node                 { return e.from }
func (e attrEdge) To() graph.Node                   { return e.to }
func (e attrEdge) ReversedEdge() graph.Edge         { e.from, e.to = e.to, e.from; return e }
func (e attrEdge) Weight() float64                  { return 0 }
func (e attrEdge) Attributes() []encoding.Attribute { return e.attr }

func directedEdgeAttrGraphFrom(g []intset, attr map[edge][]encoding.Attribute) graph.Directed {
	dg := simple.NewDirectedGraph()
	for u, e := range g {
		u := int64(u)
		for v := range e {
			dg.SetEdge(attrEdge{from: simple.Node(u), to: simple.Node(v), attr: attr[edge{from: u, to: v}]})
		}
	}
	return dg
}

func undirectedEdgeAttrGraphFrom(g []intset, attr map[edge][]encoding.Attribute) graph.Graph {
	dg := simple.NewUndirectedGraph()
	for u, e := range g {
		u := int64(u)
		for v := range e {
			dg.SetEdge(attrEdge{from: simple.Node(u), to: simple.Node(v), attr: attr[edge{from: u, to: v}]})
		}
	}
	return dg
}

type portedEdge struct {
	from, to graph.Node

	directed bool

	fromPort    string
	fromCompass string
	toPort      string
	toCompass   string
}

func (e portedEdge) From() graph.Node { return e.from }
func (e portedEdge) To() graph.Node   { return e.to }
func (e portedEdge) ReversedEdge() graph.Edge {
	e.from, e.to = e.to, e.from
	e.fromPort, e.toPort = e.toPort, e.fromPort
	e.fromCompass, e.toCompass = e.toCompass, e.fromCompass
	return e
}
func (e portedEdge) Weight() float64 { return 0 }

func (e portedEdge) FromPort() (port, compass string) {
	return e.fromPort, e.fromCompass
}
func (e portedEdge) ToPort() (port, compass string) {
	return e.toPort, e.toCompass
}

func directedPortedAttrGraphFrom(g []intset, attr [][]encoding.Attribute, ports map[edge]portedEdge) graph.Directed {
	dg := simple.NewDirectedGraph()
	for u, e := range g {
		u := int64(u)
		var at []encoding.Attribute
		if u < int64(len(attr)) {
			at = attr[u]
		}
		nu := attrNode{id: u, attr: at}
		for v := range e {
			if v < int64(len(attr)) {
				at = attr[v]
			}
			pe := ports[edge{from: u, to: v}]
			pe.from = nu
			pe.to = attrNode{id: v, attr: at}
			dg.SetEdge(pe)
		}
	}
	return dg
}

func undirectedPortedAttrGraphFrom(g []intset, attr [][]encoding.Attribute, ports map[edge]portedEdge) graph.Graph {
	dg := simple.NewUndirectedGraph()
	for u, e := range g {
		u := int64(u)
		var at []encoding.Attribute
		if u < int64(len(attr)) {
			at = attr[u]
		}
		nu := attrNode{id: u, attr: at}
		for v := range e {
			if v < int64(len(attr)) {
				at = attr[v]
			}
			pe := ports[edge{from: u, to: v}]
			pe.from = nu
			pe.to = attrNode{id: v, attr: at}
			dg.SetEdge(pe)
		}
	}
	return dg
}

type graphAttributer struct {
	graph.Graph
	graph attributer
	node  attributer
	edge  attributer
}

type attributer []encoding.Attribute

func (a attributer) Attributes() []encoding.Attribute { return a }

func (g graphAttributer) DOTAttributers() (graph, node, edge encoding.Attributer) {
	return g.graph, g.node, g.edge
}

type structuredGraph struct {
	*simple.UndirectedGraph
	sub []Graph
}

func undirectedStructuredGraphFrom(c []edge, g ...[]intset) graph.Graph {
	s := &structuredGraph{UndirectedGraph: simple.NewUndirectedGraph()}
	var base int64
	for i, sg := range g {
		sub := simple.NewUndirectedGraph()
		for u, e := range sg {
			u := int64(u)
			for v := range e {
				ce := simple.Edge{F: simple.Node(u + base), T: simple.Node(v + base)}
				sub.SetEdge(ce)
			}
		}
		s.sub = append(s.sub, namedGraph{id: int64(i), Graph: sub})
		base += int64(len(sg))
	}
	for _, e := range c {
		s.SetEdge(simple.Edge{F: simple.Node(e.from), T: simple.Node(e.to)})
	}
	return s
}

func (g structuredGraph) Structure() []Graph {
	return g.sub
}

type namedGraph struct {
	id int64
	graph.Graph
}

func (g namedGraph) DOTID() string { return alpha[g.id : g.id+1] }

type subGraph struct {
	id int64
	graph.Graph
}

func (g subGraph) ID() int64 { return g.id }
func (g subGraph) Subgraph() graph.Graph {
	return namedGraph(g)
}

func undirectedSubGraphFrom(g []intset, s map[int64][]intset) graph.Graph {
	var base int64
	subs := make(map[int64]subGraph)
	for i, sg := range s {
		sub := simple.NewUndirectedGraph()
		for u, e := range sg {
			u := int64(u)
			for v := range e {
				ce := simple.Edge{F: simple.Node(u + base), T: simple.Node(v + base)}
				sub.SetEdge(ce)
			}
		}
		subs[i] = subGraph{id: int64(i), Graph: sub}
		base += int64(len(sg))
	}

	dg := simple.NewUndirectedGraph()
	for u, e := range g {
		u := int64(u)
		var nu graph.Node
		if sg, ok := subs[u]; ok {
			sg.id += base
			nu = sg
		} else {
			nu = simple.Node(u + base)
		}
		for v := range e {
			var nv graph.Node
			if sg, ok := subs[v]; ok {
				sg.id += base
				nv = sg
			} else {
				nv = simple.Node(v + base)
			}
			dg.SetEdge(simple.Edge{F: nu, T: nv})
		}
	}
	return dg
}

var encodeTests = []struct {
	name string
	g    graph.Graph

	prefix string

	want string
}{
	// Basic graph.Graph handling.
	{
		name: "PageRank",
		g:    directedGraphFrom(pageRankGraph),

		want: `strict digraph PageRank {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;
	5;
	6;
	7;
	8;
	9;
	10;

	// Edge definitions.
	1 -> 2;
	2 -> 1;
	3 -> 0;
	3 -> 1;
	4 -> 1;
	4 -> 3;
	4 -> 5;
	5 -> 1;
	5 -> 4;
	6 -> 1;
	6 -> 4;
	7 -> 1;
	7 -> 4;
	8 -> 1;
	8 -> 4;
	9 -> 4;
	10 -> 4;
}`,
	},
	{
		g: undirectedGraphFrom(pageRankGraph),

		want: `strict graph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;
	5;
	6;
	7;
	8;
	9;
	10;

	// Edge definitions.
	0 -- 3;
	1 -- 2;
	1 -- 3;
	1 -- 4;
	1 -- 5;
	1 -- 6;
	1 -- 7;
	1 -- 8;
	3 -- 4;
	4 -- 5;
	4 -- 6;
	4 -- 7;
	4 -- 8;
	4 -- 9;
	4 -- 10;
}`,
	},
	{
		g: directedGraphFrom(powerMethodGraph),

		want: `strict digraph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -> 1;
	0 -> 2;
	1 -> 3;
	2 -> 3;
	2 -> 4;
	3 -> 4;
	4 -> 0;
}`,
	},
	{
		g: undirectedGraphFrom(powerMethodGraph),

		want: `strict graph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -- 1;
	0 -- 2;
	0 -- 4;
	1 -- 3;
	2 -- 3;
	2 -- 4;
	3 -- 4;
}`,
	},
	{
		g:      undirectedGraphFrom(powerMethodGraph),
		prefix: "# ",

		want: `# strict graph {
# 	// Node definitions.
# 	0;
# 	1;
# 	2;
# 	3;
# 	4;
#
# 	// Edge definitions.
# 	0 -- 1;
# 	0 -- 2;
# 	0 -- 4;
# 	1 -- 3;
# 	2 -- 3;
# 	2 -- 4;
# 	3 -- 4;
# }`,
	},

	// Names named nodes.
	{
		name: "PageRank",
		g:    directedNamedIDGraphFrom(pageRankGraph),

		want: `strict digraph PageRank {
	// Node definitions.
	A;
	B;
	C;
	D;
	E;
	F;
	G;
	H;
	I;
	J;
	K;

	// Edge definitions.
	B -> C;
	C -> B;
	D -> A;
	D -> B;
	E -> B;
	E -> D;
	E -> F;
	F -> B;
	F -> E;
	G -> B;
	G -> E;
	H -> B;
	H -> E;
	I -> B;
	I -> E;
	J -> E;
	K -> E;
}`,
	},
	{
		g: undirectedNamedIDGraphFrom(pageRankGraph),

		want: `strict graph {
	// Node definitions.
	A;
	B;
	C;
	D;
	E;
	F;
	G;
	H;
	I;
	J;
	K;

	// Edge definitions.
	A -- D;
	B -- C;
	B -- D;
	B -- E;
	B -- F;
	B -- G;
	B -- H;
	B -- I;
	D -- E;
	E -- F;
	E -- G;
	E -- H;
	E -- I;
	E -- J;
	E -- K;
}`,
	},
	{
		g: directedNamedIDGraphFrom(powerMethodGraph),

		want: `strict digraph {
	// Node definitions.
	A;
	B;
	C;
	D;
	E;

	// Edge definitions.
	A -> B;
	A -> C;
	B -> D;
	C -> D;
	C -> E;
	D -> E;
	E -> A;
}`,
	},
	{
		g: undirectedNamedIDGraphFrom(powerMethodGraph),

		want: `strict graph {
	// Node definitions.
	A;
	B;
	C;
	D;
	E;

	// Edge definitions.
	A -- B;
	A -- C;
	A -- E;
	B -- D;
	C -- D;
	C -- E;
	D -- E;
}`,
	},
	{
		g:      undirectedNamedIDGraphFrom(powerMethodGraph),
		prefix: "# ",

		want: `# strict graph {
# 	// Node definitions.
# 	A;
# 	B;
# 	C;
# 	D;
# 	E;
#
# 	// Edge definitions.
# 	A -- B;
# 	A -- C;
# 	A -- E;
# 	B -- D;
# 	C -- D;
# 	C -- E;
# 	D -- E;
# }`,
	},

	// Handling nodes with attributes.
	{
		g: directedNodeAttrGraphFrom(powerMethodGraph, nil),

		want: `strict digraph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -> 1;
	0 -> 2;
	1 -> 3;
	2 -> 3;
	2 -> 4;
	3 -> 4;
	4 -> 0;
}`,
	},
	{
		g: undirectedNodeAttrGraphFrom(powerMethodGraph, nil),

		want: `strict graph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -- 1;
	0 -- 2;
	0 -- 4;
	1 -- 3;
	2 -- 3;
	2 -- 4;
	3 -- 4;
}`,
	},
	{
		g: directedNodeAttrGraphFrom(powerMethodGraph, [][]encoding.Attribute{
			2: {{Key: "fontsize", Value: "16"}, {Key: "shape", Value: "ellipse"}},
			4: {},
		}),

		want: `strict digraph {
	// Node definitions.
	0;
	1;
	2 [
		fontsize=16
		shape=ellipse
	];
	3;
	4;

	// Edge definitions.
	0 -> 1;
	0 -> 2;
	1 -> 3;
	2 -> 3;
	2 -> 4;
	3 -> 4;
	4 -> 0;
}`,
	},
	{
		g: undirectedNodeAttrGraphFrom(powerMethodGraph, [][]encoding.Attribute{
			2: {{Key: "fontsize", Value: "16"}, {Key: "shape", Value: "ellipse"}},
			4: {},
		}),

		want: `strict graph {
	// Node definitions.
	0;
	1;
	2 [
		fontsize=16
		shape=ellipse
	];
	3;
	4;

	// Edge definitions.
	0 -- 1;
	0 -- 2;
	0 -- 4;
	1 -- 3;
	2 -- 3;
	2 -- 4;
	3 -- 4;
}`,
	},
	{
		g: directedNamedIDNodeAttrGraphFrom(powerMethodGraph, [][]encoding.Attribute{
			2: {{Key: "fontsize", Value: "16"}, {Key: "shape", Value: "ellipse"}},
			4: {},
		}),

		want: `strict digraph {
	// Node definitions.
	A;
	B;
	C [
		fontsize=16
		shape=ellipse
	];
	D;
	E;

	// Edge definitions.
	A -> B;
	A -> C;
	B -> D;
	C -> D;
	C -> E;
	D -> E;
	E -> A;
}`,
	},
	{
		g: undirectedNamedIDNodeAttrGraphFrom(powerMethodGraph, [][]encoding.Attribute{
			0: nil,
			1: nil,
			2: {{Key: "fontsize", Value: "16"}, {Key: "shape", Value: "ellipse"}},
			3: nil,
			4: {},
		}),

		want: `strict graph {
	// Node definitions.
	A;
	B;
	C [
		fontsize=16
		shape=ellipse
	];
	D;
	E;

	// Edge definitions.
	A -- B;
	A -- C;
	A -- E;
	B -- D;
	C -- D;
	C -- E;
	D -- E;
}`,
	},

	// Handling edge with attributes.
	{
		g: directedEdgeAttrGraphFrom(powerMethodGraph, nil),

		want: `strict digraph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -> 1;
	0 -> 2;
	1 -> 3;
	2 -> 3;
	2 -> 4;
	3 -> 4;
	4 -> 0;
}`,
	},
	{
		g: undirectedEdgeAttrGraphFrom(powerMethodGraph, nil),

		want: `strict graph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -- 1;
	0 -- 2;
	0 -- 4;
	1 -- 3;
	2 -- 3;
	2 -- 4;
	3 -- 4;
}`,
	},
	{
		g: directedEdgeAttrGraphFrom(powerMethodGraph, map[edge][]encoding.Attribute{
			{from: 0, to: 2}: {{Key: "label", Value: `"???"`}, {Key: "style", Value: "dashed"}},
			{from: 2, to: 4}: {},
			{from: 3, to: 4}: {{Key: "color", Value: "red"}},
		}),

		want: `strict digraph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -> 1;
	0 -> 2 [
		label="???"
		style=dashed
	];
	1 -> 3;
	2 -> 3;
	2 -> 4;
	3 -> 4 [color=red];
	4 -> 0;
}`,
	},
	{
		g: undirectedEdgeAttrGraphFrom(powerMethodGraph, map[edge][]encoding.Attribute{
			{from: 0, to: 2}: {{Key: "label", Value: `"???"`}, {Key: "style", Value: "dashed"}},
			{from: 2, to: 4}: {},
			{from: 3, to: 4}: {{Key: "color", Value: "red"}},
		}),

		want: `strict graph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -- 1;
	0 -- 2 [
		label="???"
		style=dashed
	];
	0 -- 4;
	1 -- 3;
	2 -- 3;
	2 -- 4;
	3 -- 4 [color=red];
}`,
	},
	{
		g: undirectedEdgeAttrGraphFrom(powerMethodGraph, map[edge][]encoding.Attribute{
			// label attribute not quoted and containing spaces.
			{from: 0, to: 2}: {{Key: "label", Value: `hello world`}, {Key: "style", Value: "dashed"}},
			{from: 2, to: 4}: {},
			{from: 3, to: 4}: {{Key: "label", Value: `foo bar`}},
		}),

		want: `strict graph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -- 1;
	0 -- 2 [
		label="hello world"
		style=dashed
	];
	0 -- 4;
	1 -- 3;
	2 -- 3;
	2 -- 4;
	3 -- 4 [label="foo bar"];
}`,
	},
	{
		g: undirectedEdgeAttrGraphFrom(powerMethodGraph, map[edge][]encoding.Attribute{
			// keywords must be quoted if used as attributes.
			{from: 0, to: 2}: {{Key: "label", Value: `NODE`}, {Key: "style", Value: "dashed"}},
			{from: 2, to: 4}: {},
			{from: 3, to: 4}: {{Key: "label", Value: `subgraph`}},
		}),

		want: `strict graph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -- 1;
	0 -- 2 [
		label="NODE"
		style=dashed
	];
	0 -- 4;
	1 -- 3;
	2 -- 3;
	2 -- 4;
	3 -- 4 [label="subgraph"];
}`,
	},

	// Handling nodes with ports.
	{
		g: directedPortedAttrGraphFrom(powerMethodGraph, nil, nil),

		want: `strict digraph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -> 1;
	0 -> 2;
	1 -> 3;
	2 -> 3;
	2 -> 4;
	3 -> 4;
	4 -> 0;
}`,
	},
	{
		g: undirectedPortedAttrGraphFrom(powerMethodGraph, nil, nil),

		want: `strict graph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -- 1;
	0 -- 2;
	0 -- 4;
	1 -- 3;
	2 -- 3;
	2 -- 4;
	3 -- 4;
}`,
	},
	{
		g: directedPortedAttrGraphFrom(powerMethodGraph,
			[][]encoding.Attribute{
				2: {{Key: "shape", Value: "record"}, {Key: "label", Value: `"<Two>English|<Zwei>German"`}},
				4: {{Key: "shape", Value: "record"}, {Key: "label", Value: `"<Four>English|<Vier>German"`}},
			},
			map[edge]portedEdge{
				{from: 0, to: 1}: {fromCompass: "s"},
				{from: 0, to: 2}: {fromCompass: "s", toPort: "Zwei", toCompass: "e"},
				{from: 2, to: 3}: {fromPort: "Zwei", fromCompass: "e"},
				{from: 2, to: 4}: {fromPort: "Two", fromCompass: "w", toPort: "Four", toCompass: "w"},
				{from: 3, to: 4}: {toPort: "Four", toCompass: "w"},
				{from: 4, to: 0}: {fromPort: "Four", fromCompass: "_", toCompass: "s"},
			},
		),

		want: `strict digraph {
	// Node definitions.
	0;
	1;
	2 [
		shape=record
		label="<Two>English|<Zwei>German"
	];
	3;
	4 [
		shape=record
		label="<Four>English|<Vier>German"
	];

	// Edge definitions.
	0:s -> 1;
	0:s -> 2:Zwei:e;
	1 -> 3;
	2:Zwei:e -> 3;
	2:Two:w -> 4:Four:w;
	3 -> 4:Four:w;
	4:Four:_ -> 0:s;
}`,
	},
	{
		g: undirectedPortedAttrGraphFrom(powerMethodGraph,
			[][]encoding.Attribute{
				2: {{Key: "shape", Value: "record"}, {Key: "label", Value: `"<Two>English|<Zwei>German"`}},
				4: {{Key: "shape", Value: "record"}, {Key: "label", Value: `"<Four>English|<Vier>German"`}},
			},
			map[edge]portedEdge{
				{from: 0, to: 1}: {fromCompass: "s"},
				{from: 0, to: 2}: {fromCompass: "s", toPort: "Zwei", toCompass: "e"},
				{from: 2, to: 3}: {fromPort: "Zwei", fromCompass: "e"},
				{from: 2, to: 4}: {fromPort: "Two", fromCompass: "w", toPort: "Four", toCompass: "w"},
				{from: 3, to: 4}: {toPort: "Four", toCompass: "w"},
				{from: 4, to: 0}: {fromPort: "Four", fromCompass: "_", toCompass: "s"},
			},
		),

		want: `strict graph {
	// Node definitions.
	0;
	1;
	2 [
		shape=record
		label="<Two>English|<Zwei>German"
	];
	3;
	4 [
		shape=record
		label="<Four>English|<Vier>German"
	];

	// Edge definitions.
	0:s -- 1;
	0:s -- 2:Zwei:e;
	0:s -- 4:Four:_;
	1 -- 3;
	2:Zwei:e -- 3;
	2:Two:w -- 4:Four:w;
	3 -- 4:Four:w;
}`,
	},

	// Handling graph attributes.
	{
		g: graphAttributer{Graph: undirectedEdgeAttrGraphFrom(powerMethodGraph, map[edge][]encoding.Attribute{
			{from: 0, to: 2}: {{Key: "label", Value: `"???"`}, {Key: "style", Value: "dashed"}},
			{from: 2, to: 4}: {},
			{from: 3, to: 4}: {{Key: "color", Value: "red"}},
		})},

		want: `strict graph {
	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -- 1;
	0 -- 2 [
		label="???"
		style=dashed
	];
	0 -- 4;
	1 -- 3;
	2 -- 3;
	2 -- 4;
	3 -- 4 [color=red];
}`,
	},
	{
		g: graphAttributer{Graph: undirectedEdgeAttrGraphFrom(powerMethodGraph, map[edge][]encoding.Attribute{
			{from: 0, to: 2}: {{Key: "label", Value: `"???"`}, {Key: "style", Value: "dashed"}},
			{from: 2, to: 4}: {},
			{from: 3, to: 4}: {{Key: "color", Value: "red"}},
		}),
			graph: []encoding.Attribute{{Key: "rankdir", Value: `"LR"`}},
			node:  []encoding.Attribute{{Key: "fontsize", Value: "16"}, {Key: "shape", Value: "ellipse"}},
		},

		want: `strict graph {
	graph [
		rankdir="LR"
	];
	node [
		fontsize=16
		shape=ellipse
	];

	// Node definitions.
	0;
	1;
	2;
	3;
	4;

	// Edge definitions.
	0 -- 1;
	0 -- 2 [
		label="???"
		style=dashed
	];
	0 -- 4;
	1 -- 3;
	2 -- 3;
	2 -- 4;
	3 -- 4 [color=red];
}`,
	},

	// Handling structured graphs.
	{
		g: undirectedStructuredGraphFrom(nil, powerMethodGraph, pageRankGraph),

		want: `strict graph {
	subgraph A {
		// Node definitions.
		0;
		1;
		2;
		3;
		4;

		// Edge definitions.
		0 -- 1;
		0 -- 2;
		0 -- 4;
		1 -- 3;
		2 -- 3;
		2 -- 4;
		3 -- 4;
	}
	subgraph B {
		// Node definitions.
		5;
		6;
		7;
		8;
		9;
		10;
		11;
		12;
		13;
		14;
		15;

		// Edge definitions.
		5 -- 8;
		6 -- 7;
		6 -- 8;
		6 -- 9;
		6 -- 10;
		6 -- 11;
		6 -- 12;
		6 -- 13;
		8 -- 9;
		9 -- 10;
		9 -- 11;
		9 -- 12;
		9 -- 13;
		9 -- 14;
		9 -- 15;
	}
}`,
	},
	{
		g: undirectedStructuredGraphFrom([]edge{{from: 0, to: 9}}, powerMethodGraph, pageRankGraph),

		want: `strict graph {
	subgraph A {
		// Node definitions.
		0;
		1;
		2;
		3;
		4;

		// Edge definitions.
		0 -- 1;
		0 -- 2;
		0 -- 4;
		1 -- 3;
		2 -- 3;
		2 -- 4;
		3 -- 4;
	}
	subgraph B {
		// Node definitions.
		5;
		6;
		7;
		8;
		9;
		10;
		11;
		12;
		13;
		14;
		15;

		// Edge definitions.
		5 -- 8;
		6 -- 7;
		6 -- 8;
		6 -- 9;
		6 -- 10;
		6 -- 11;
		6 -- 12;
		6 -- 13;
		8 -- 9;
		9 -- 10;
		9 -- 11;
		9 -- 12;
		9 -- 13;
		9 -- 14;
		9 -- 15;
	}
	// Node definitions.
	0;
	9;

	// Edge definitions.
	0 -- 9;
}`,
	},

	// Handling subgraphs.
	{
		g: undirectedSubGraphFrom(pageRankGraph, map[int64][]intset{2: powerMethodGraph}),

		want: `strict graph {
	// Node definitions.
	5;
	6;
	8;
	9;
	10;
	11;
	12;
	13;
	14;
	15;

	// Edge definitions.
	5 -- 8;
	6 -- subgraph H {
		// Node definitions.
		0;
		1;
		2;
		3;
		4;

		// Edge definitions.
		0 -- 1;
		0 -- 2;
		0 -- 4;
		1 -- 3;
		2 -- 3;
		2 -- 4;
		3 -- 4;
	};
	6 -- 8;
	6 -- 9;
	6 -- 10;
	6 -- 11;
	6 -- 12;
	6 -- 13;
	8 -- 9;
	9 -- 10;
	9 -- 11;
	9 -- 12;
	9 -- 13;
	9 -- 14;
	9 -- 15;
}`,
	},
	{
		name: "H",
		g:    undirectedSubGraphFrom(pageRankGraph, map[int64][]intset{1: powerMethodGraph}),

		want: `strict graph H {
	// Node definitions.
	5;
	7;
	8;
	9;
	10;
	11;
	12;
	13;
	14;
	15;

	// Edge definitions.
	5 -- 8;
	subgraph G {
		// Node definitions.
		0;
		1;
		2;
		3;
		4;

		// Edge definitions.
		0 -- 1;
		0 -- 2;
		0 -- 4;
		1 -- 3;
		2 -- 3;
		2 -- 4;
		3 -- 4;
	} -- 7;
	subgraph G {
		// Node definitions.
		0;
		1;
		2;
		3;
		4;
	} -- 8;
	subgraph G {
		// Node definitions.
		0;
		1;
		2;
		3;
		4;
	} -- 9;
	subgraph G {
		// Node definitions.
		0;
		1;
		2;
		3;
		4;
	} -- 10;
	subgraph G {
		// Node definitions.
		0;
		1;
		2;
		3;
		4;
	} -- 11;
	subgraph G {
		// Node definitions.
		0;
		1;
		2;
		3;
		4;
	} -- 12;
	subgraph G {
		// Node definitions.
		0;
		1;
		2;
		3;
		4;
	} -- 13;
	8 -- 9;
	9 -- 10;
	9 -- 11;
	9 -- 12;
	9 -- 13;
	9 -- 14;
	9 -- 15;
}`,
	},
}

func TestEncode(t *testing.T) {
	for i, test := range encodeTests {
		got, err := Marshal(test.g, test.name, test.prefix, "\t")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if string(got) != test.want {
			t.Errorf("unexpected DOT result for test %d:\ngot: %s\nwant:%s", i, got, test.want)
		}
		checkDOT(t, got)
	}
}

type intlist []int64

func createMultigraph(g []intlist) graph.Multigraph {
	dg := multi.NewUndirectedGraph()
	for u, e := range g {
		u := int64(u)
		nu := multi.Node(u)
		for _, v := range e {
			nv := multi.Node(v)
			dg.SetLine(dg.NewLine(nu, nv))
		}
	}
	return dg
}

func createNamedMultigraph(g []intlist) graph.Multigraph {
	dg := multi.NewUndirectedGraph()
	for u, e := range g {
		u := int64(u)
		nu := namedNode{id: u, name: alpha[u : u+1]}
		for _, v := range e {
			nv := namedNode{id: v, name: alpha[v : v+1]}
			dg.SetLine(dg.NewLine(nu, nv))
		}
	}
	return dg
}

func createDirectedMultigraph(g []intlist) graph.Multigraph {
	dg := multi.NewDirectedGraph()
	for u, e := range g {
		u := int64(u)
		nu := multi.Node(u)
		for _, v := range e {
			nv := multi.Node(v)
			dg.SetLine(dg.NewLine(nu, nv))
		}
	}
	return dg
}

func createNamedDirectedMultigraph(g []intlist) graph.Multigraph {
	dg := multi.NewDirectedGraph()
	for u, e := range g {
		u := int64(u)
		nu := namedNode{id: u, name: alpha[u : u+1]}
		for _, v := range e {
			nv := namedNode{id: v, name: alpha[v : v+1]}
			dg.SetLine(dg.NewLine(nu, nv))
		}
	}
	return dg
}

var encodeMultiTests = []struct {
	name string
	g    graph.Multigraph

	prefix string

	want string
}{
	{
		g: createMultigraph([]intlist{}),
		want: `graph {
}`,
	},
	{
		g: createMultigraph([]intlist{
			0: {1},
			1: {0, 2},
			2: {},
		}),
		want: `graph {
	// Node definitions.
	0;
	1;
	2;

	// Edge definitions.
	0 -- 1;
	0 -- 1;
	1 -- 2;
}`,
	},
	{
		g: createMultigraph([]intlist{
			0: {1},
			1: {2, 2},
			2: {0, 0, 0},
		}),
		want: `graph {
	// Node definitions.
	0;
	1;
	2;

	// Edge definitions.
	0 -- 1;
	0 -- 2;
	0 -- 2;
	0 -- 2;
	1 -- 2;
	1 -- 2;
}`,
	},
	{
		g: createNamedMultigraph([]intlist{
			0: {1},
			1: {2, 2},
			2: {0, 0, 0},
		}),
		want: `graph {
	// Node definitions.
	A;
	B;
	C;

	// Edge definitions.
	A -- B;
	A -- C;
	A -- C;
	A -- C;
	B -- C;
	B -- C;
}`,
	},
	{
		g: createMultigraph([]intlist{
			0: {2, 1, 0},
			1: {2, 1, 0},
			2: {2, 1, 0},
		}),
		want: `graph {
	// Node definitions.
	0;
	1;
	2;

	// Edge definitions.
	0 -- 0;
	0 -- 1;
	0 -- 1;
	0 -- 2;
	0 -- 2;
	1 -- 1;
	1 -- 2;
	1 -- 2;
	2 -- 2;
}`,
	},
	{
		g: createDirectedMultigraph([]intlist{}),
		want: `digraph {
}`,
	},
	{
		g: createDirectedMultigraph([]intlist{
			0: {1},
			1: {0, 2},
			2: {},
		}),
		want: `digraph {
	// Node definitions.
	0;
	1;
	2;

	// Edge definitions.
	0 -> 1;
	1 -> 0;
	1 -> 2;
}`,
	},
	{
		g: createDirectedMultigraph([]intlist{
			0: {1},
			1: {2, 2},
			2: {0, 0, 0},
		}),
		want: `digraph {
	// Node definitions.
	0;
	1;
	2;

	// Edge definitions.
	0 -> 1;
	1 -> 2;
	1 -> 2;
	2 -> 0;
	2 -> 0;
	2 -> 0;
}`,
	},
	{
		g: createNamedDirectedMultigraph([]intlist{
			0: {1},
			1: {2, 2},
			2: {0, 0, 0},
		}),
		want: `digraph {
	// Node definitions.
	A;
	B;
	C;

	// Edge definitions.
	A -> B;
	B -> C;
	B -> C;
	C -> A;
	C -> A;
	C -> A;
}`,
	},
	{
		g: createDirectedMultigraph([]intlist{
			0: {2, 1, 0},
			1: {2, 1, 0},
			2: {2, 1, 0},
		}),
		want: `digraph {
	// Node definitions.
	0;
	1;
	2;

	// Edge definitions.
	0 -> 0;
	0 -> 1;
	0 -> 2;
	1 -> 0;
	1 -> 1;
	1 -> 2;
	2 -> 0;
	2 -> 1;
	2 -> 2;
}`,
	},
}

func TestEncodeMulti(t *testing.T) {
	for i, test := range encodeMultiTests {
		got, err := MarshalMulti(test.g, test.name, test.prefix, "\t")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if string(got) != test.want {
			t.Errorf("unexpected DOT result for test %d:\ngot: %s\nwant:%s", i, got, test.want)
		}
		checkDOT(t, got)
	}
}

// checkDOT hands b to the dot executable if it exists and fails t if dot
// returns an error.
func checkDOT(t *testing.T, b []byte) {
	dot, err := exec.LookPath("dot")
	if err != nil {
		t.Logf("skipping DOT syntax check: %v", err)
		return
	}
	cmd := exec.Command(dot)
	cmd.Stdin = bytes.NewReader(b)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	err = cmd.Run()
	if err != nil {
		t.Errorf("invalid DOT syntax: %v\n%s\ninput:\n%s", err, stderr.String(), b)
	}
}
