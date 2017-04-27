// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dot

import (
	"testing"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
)

// set is an integer set.
type set map[int]struct{}

func linksTo(i ...int) set {
	if len(i) == 0 {
		return nil
	}
	s := make(set)
	for _, v := range i {
		s[v] = struct{}{}
	}
	return s
}

var (
	// Example graph from http://en.wikipedia.org/wiki/File:PageRanks-Example.svg 16:17, 8 July 2009
	// Node identities are rewritten here to use integers from 0 to match with the DOT output.
	pageRankGraph = []set{
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
	powerMethodGraph = []set{
		0: linksTo(1, 2),
		1: linksTo(3),
		2: linksTo(3, 4),
		3: linksTo(4),
		4: linksTo(0),
	}
)

func directedGraphFrom(g []set) graph.Directed {
	dg := concrete.NewDirectedGraph()
	for u, e := range g {
		for v := range e {
			dg.SetEdge(concrete.Edge{F: concrete.Node(u), T: concrete.Node(v)}, 0)
		}
	}
	return dg
}

func undirectedGraphFrom(g []set) graph.Graph {
	dg := concrete.NewGraph()
	for u, e := range g {
		for v := range e {
			dg.SetEdge(concrete.Edge{F: concrete.Node(u), T: concrete.Node(v)}, 0)
		}
	}
	return dg
}

const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

type namedNode struct {
	id   int
	name string
}

func (n namedNode) ID() int       { return n.id }
func (n namedNode) DOTID() string { return n.name }

func directedNamedIDGraphFrom(g []set) graph.Directed {
	dg := concrete.NewDirectedGraph()
	for u, e := range g {
		nu := namedNode{id: u, name: alpha[u : u+1]}
		for v := range e {
			nv := namedNode{id: v, name: alpha[v : v+1]}
			dg.SetEdge(concrete.Edge{F: nu, T: nv}, 0)
		}
	}
	return dg
}

func undirectedNamedIDGraphFrom(g []set) graph.Graph {
	dg := concrete.NewGraph()
	for u, e := range g {
		nu := namedNode{id: u, name: alpha[u : u+1]}
		for v := range e {
			nv := namedNode{id: v, name: alpha[v : v+1]}
			dg.SetEdge(concrete.Edge{F: nu, T: nv}, 0)
		}
	}
	return dg
}

type attrNode struct {
	id   int
	name string
	attr []Attribute
}

func (n attrNode) ID() int                    { return n.id }
func (n attrNode) DOTAttributes() []Attribute { return n.attr }

func directedNodeAttrGraphFrom(g []set, attr [][]Attribute) graph.Directed {
	dg := concrete.NewDirectedGraph()
	for u, e := range g {
		var at []Attribute
		if u < len(attr) {
			at = attr[u]
		}
		nu := attrNode{id: u, attr: at}
		for v := range e {
			if v < len(attr) {
				at = attr[v]
			}
			nv := attrNode{id: v, attr: at}
			dg.SetEdge(concrete.Edge{F: nu, T: nv}, 0)
		}
	}
	return dg
}

func undirectedNodeAttrGraphFrom(g []set, attr [][]Attribute) graph.Graph {
	dg := concrete.NewGraph()
	for u, e := range g {
		var at []Attribute
		if u < len(attr) {
			at = attr[u]
		}
		nu := attrNode{id: u, attr: at}
		for v := range e {
			if v < len(attr) {
				at = attr[v]
			}
			nv := attrNode{id: v, attr: at}
			dg.SetEdge(concrete.Edge{F: nu, T: nv}, 0)
		}
	}
	return dg
}

type namedAttrNode struct {
	id   int
	name string
	attr []Attribute
}

func (n namedAttrNode) ID() int                    { return n.id }
func (n namedAttrNode) DOTID() string              { return n.name }
func (n namedAttrNode) DOTAttributes() []Attribute { return n.attr }

func directedNamedIDNodeAttrGraphFrom(g []set, attr [][]Attribute) graph.Directed {
	dg := concrete.NewDirectedGraph()
	for u, e := range g {
		var at []Attribute
		if u < len(attr) {
			at = attr[u]
		}
		nu := namedAttrNode{id: u, name: alpha[u : u+1], attr: at}
		for v := range e {
			if v < len(attr) {
				at = attr[v]
			}
			nv := namedAttrNode{id: v, name: alpha[v : v+1], attr: at}
			dg.SetEdge(concrete.Edge{F: nu, T: nv}, 0)
		}
	}
	return dg
}

func undirectedNamedIDNodeAttrGraphFrom(g []set, attr [][]Attribute) graph.Graph {
	dg := concrete.NewGraph()
	for u, e := range g {
		var at []Attribute
		if u < len(attr) {
			at = attr[u]
		}
		nu := namedAttrNode{id: u, name: alpha[u : u+1], attr: at}
		for v := range e {
			if v < len(attr) {
				at = attr[v]
			}
			nv := namedAttrNode{id: v, name: alpha[v : v+1], attr: at}
			dg.SetEdge(concrete.Edge{F: nu, T: nv}, 0)
		}
	}
	return dg
}

type attrEdge struct {
	from, to graph.Node

	attr []Attribute
}

func (e attrEdge) From() graph.Node           { return e.from }
func (e attrEdge) To() graph.Node             { return e.to }
func (e attrEdge) DOTAttributes() []Attribute { return e.attr }

func directedEdgeAttrGraphFrom(g []set, attr map[edge][]Attribute) graph.Directed {
	dg := concrete.NewDirectedGraph()
	for u, e := range g {
		for v := range e {
			dg.SetEdge(attrEdge{from: concrete.Node(u), to: concrete.Node(v), attr: attr[edge{from: u, to: v}]}, 0)
		}
	}
	return dg
}

func undirectedEdgeAttrGraphFrom(g []set, attr map[edge][]Attribute) graph.Graph {
	dg := concrete.NewGraph()
	for u, e := range g {
		for v := range e {
			dg.SetEdge(attrEdge{from: concrete.Node(u), to: concrete.Node(v), attr: attr[edge{from: u, to: v}]}, 0)
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

// TODO(kortschak): Figure out a better way to handle the fact that
// headedness is an undefined concept in undirected graphs. We sort
// nodes by ID, so lower ID nodes are always from nodes in undirected
// graphs. We can probably do this in the printer, but I am leaving
// this here as a WARNING.
// Maybe the approach should be to document that for undirected graphs
// the low ID node should be returned by the FromPort and the high ID
// by the ToPort calls.
func (e portedEdge) FromPort() (port, compass string) {
	return e.fromPort, e.fromCompass
}
func (e portedEdge) ToPort() (port, compass string) {
	return e.toPort, e.toCompass
}

func directedPortedAttrGraphFrom(g []set, attr [][]Attribute, ports map[edge]portedEdge) graph.Directed {
	dg := concrete.NewDirectedGraph()
	for u, e := range g {
		var at []Attribute
		if u < len(attr) {
			at = attr[u]
		}
		nu := attrNode{id: u, attr: at}
		for v := range e {
			if v < len(attr) {
				at = attr[v]
			}
			pe := ports[edge{from: u, to: v}]
			pe.from = nu
			pe.to = attrNode{id: v, attr: at}
			dg.SetEdge(pe, 0)
		}
	}
	return dg
}

func undirectedPortedAttrGraphFrom(g []set, attr [][]Attribute, ports map[edge]portedEdge) graph.Graph {
	dg := concrete.NewGraph()
	for u, e := range g {
		var at []Attribute
		if u < len(attr) {
			at = attr[u]
		}
		nu := attrNode{id: u, attr: at}
		for v := range e {
			if v < len(attr) {
				at = attr[v]
			}
			pe := ports[edge{from: u, to: v}]
			pe.from = nu
			pe.to = attrNode{id: v, attr: at}
			dg.SetEdge(pe, 0)
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

type attributer []Attribute

func (a attributer) DOTAttributes() []Attribute { return a }

func (g graphAttributer) DOTAttributers() (graph, node, edge Attributer) {
	return g.graph, g.node, g.edge
}

type structuredGraph struct {
	*concrete.Graph
	sub []Graph
}

func undirectedStructuredGraphFrom(c []edge, g ...[]set) graph.Graph {
	s := &structuredGraph{Graph: concrete.NewGraph()}
	var base int
	for i, sg := range g {
		sub := concrete.NewGraph()
		for u, e := range sg {
			for v := range e {
				ce := concrete.Edge{F: concrete.Node(u + base), T: concrete.Node(v + base)}
				sub.SetEdge(ce, 0)
			}
		}
		s.sub = append(s.sub, namedGraph{id: i, Graph: sub})
		base += len(sg)
	}
	for _, e := range c {
		s.SetEdge(concrete.Edge{F: concrete.Node(e.from), T: concrete.Node(e.to)}, 0)
	}
	return s
}

func (g structuredGraph) Structure() []Graph {
	return g.sub
}

type namedGraph struct {
	id int
	graph.Graph
}

func (g namedGraph) DOTID() string { return alpha[g.id : g.id+1] }

type subGraph struct {
	id int
	graph.Graph
}

func (g subGraph) ID() int { return g.id }
func (g subGraph) Subgraph() graph.Graph {
	return namedGraph{id: g.id, Graph: g.Graph}
}

func undirectedSubGraphFrom(g []set, s map[int][]set) graph.Graph {
	var base int
	subs := make(map[int]subGraph)
	for i, sg := range s {
		sub := concrete.NewGraph()
		for u, e := range sg {
			for v := range e {
				ce := concrete.Edge{F: concrete.Node(u + base), T: concrete.Node(v + base)}
				sub.SetEdge(ce, 0)
			}
		}
		subs[i] = subGraph{id: i, Graph: sub}
		base += len(sg)
	}

	dg := concrete.NewGraph()
	for u, e := range g {
		var nu graph.Node
		if sg, ok := subs[u]; ok {
			sg.id += base
			nu = sg
		} else {
			nu = concrete.Node(u + base)
		}
		for v := range e {
			var nv graph.Node
			if sg, ok := subs[v]; ok {
				sg.id += base
				nv = sg
			} else {
				nv = concrete.Node(v + base)
			}
			dg.SetEdge(concrete.Edge{F: nu, T: nv}, 0)
		}
	}
	return dg
}

var encodeTests = []struct {
	name   string
	g      graph.Graph
	strict bool

	prefix string

	want string
}{
	// Basic graph.Graph handling.
	{
		name: "PageRank",
		g:    directedGraphFrom(pageRankGraph),

		want: `digraph PageRank {
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

		want: `graph {
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

		want: `digraph {
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

		want: `graph {
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

		want: `# graph {
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

		want: `digraph PageRank {
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

		want: `graph {
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

		want: `digraph {
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

		want: `graph {
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

		want: `# graph {
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

		want: `digraph {
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

		want: `graph {
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
		g: directedNodeAttrGraphFrom(powerMethodGraph, [][]Attribute{
			2: {{"fontsize", "16"}, {"shape", "ellipse"}},
			4: {},
		}),

		want: `digraph {
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
		g: undirectedNodeAttrGraphFrom(powerMethodGraph, [][]Attribute{
			2: {{"fontsize", "16"}, {"shape", "ellipse"}},
			4: {},
		}),

		want: `graph {
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
		g: directedNamedIDNodeAttrGraphFrom(powerMethodGraph, [][]Attribute{
			2: {{"fontsize", "16"}, {"shape", "ellipse"}},
			4: {},
		}),

		want: `digraph {
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
		g: undirectedNamedIDNodeAttrGraphFrom(powerMethodGraph, [][]Attribute{
			0: nil,
			1: nil,
			2: {{"fontsize", "16"}, {"shape", "ellipse"}},
			3: nil,
			4: {},
		}),

		want: `graph {
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

		want: `digraph {
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

		want: `graph {
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
		g: directedEdgeAttrGraphFrom(powerMethodGraph, map[edge][]Attribute{
			edge{from: 0, to: 2}: {{"label", `"???"`}, {"style", "dashed"}},
			edge{from: 2, to: 4}: {},
			edge{from: 3, to: 4}: {{"color", "red"}},
		}),

		want: `digraph {
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
		g: undirectedEdgeAttrGraphFrom(powerMethodGraph, map[edge][]Attribute{
			edge{from: 0, to: 2}: {{"label", `"???"`}, {"style", "dashed"}},
			edge{from: 2, to: 4}: {},
			edge{from: 3, to: 4}: {{"color", "red"}},
		}),

		want: `graph {
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

	// Handling nodes with ports.
	{
		g: directedPortedAttrGraphFrom(powerMethodGraph, nil, nil),

		want: `digraph {
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

		want: `graph {
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
			[][]Attribute{
				2: {{"shape", "record"}, {"label", `"<Two>English|<Zwei>German"`}},
				4: {{"shape", "record"}, {"label", `"<Four>English|<Vier>German"`}},
			},
			map[edge]portedEdge{
				edge{from: 0, to: 1}: {fromCompass: "s"},
				edge{from: 0, to: 2}: {fromCompass: "s", toPort: "Zwei", toCompass: "e"},
				edge{from: 2, to: 3}: {fromPort: "Zwei", fromCompass: "e"},
				edge{from: 2, to: 4}: {fromPort: "Two", fromCompass: "w", toPort: "Four", toCompass: "w"},
				edge{from: 3, to: 4}: {toPort: "Four", toCompass: "w"},
				edge{from: 4, to: 0}: {fromPort: "Four", fromCompass: "_", toCompass: "s"},
			},
		),

		want: `digraph {
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
			[][]Attribute{
				2: {{"shape", "record"}, {"label", `"<Two>English|<Zwei>German"`}},
				4: {{"shape", "record"}, {"label", `"<Four>English|<Vier>German"`}},
			},
			map[edge]portedEdge{
				edge{from: 0, to: 1}: {fromCompass: "s"},
				edge{from: 0, to: 2}: {fromCompass: "s", toPort: "Zwei", toCompass: "e"},
				edge{from: 2, to: 3}: {fromPort: "Zwei", fromCompass: "e"},
				edge{from: 2, to: 4}: {fromPort: "Two", fromCompass: "w", toPort: "Four", toCompass: "w"},
				edge{from: 3, to: 4}: {toPort: "Four", toCompass: "w"},

				// This definition is reversed (see comment above at portedEdge
				// definition) so that 4 gets the from port. This is a result
				// of the fact that we sort nodes by ID, so the lower node
				// will be always be printed first when the graph is undirected,
				// thus becoming the from port, but we define the edges here
				// from a directed adjacency list.
				edge{from: 4, to: 0}: {fromCompass: "s", toPort: "Four", toCompass: "_"},
			},
		),

		want: `graph {
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
		g: graphAttributer{Graph: undirectedEdgeAttrGraphFrom(powerMethodGraph, map[edge][]Attribute{
			edge{from: 0, to: 2}: {{"label", `"???"`}, {"style", "dashed"}},
			edge{from: 2, to: 4}: {},
			edge{from: 3, to: 4}: {{"color", "red"}},
		})},

		want: `graph {
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
		g: graphAttributer{Graph: undirectedEdgeAttrGraphFrom(powerMethodGraph, map[edge][]Attribute{
			edge{from: 0, to: 2}: {{"label", `"???"`}, {"style", "dashed"}},
			edge{from: 2, to: 4}: {},
			edge{from: 3, to: 4}: {{"color", "red"}},
		}),
			graph: []Attribute{{"rankdir", `"LR"`}},
			node:  []Attribute{{"fontsize", "16"}, {"shape", "ellipse"}},
		},

		want: `graph {
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

		want: `graph {
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

		want: `graph {
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
		g: undirectedSubGraphFrom(pageRankGraph, map[int][]set{2: powerMethodGraph}),

		want: `graph {
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
		name:   "H",
		g:      undirectedSubGraphFrom(pageRankGraph, map[int][]set{1: powerMethodGraph}),
		strict: true,

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
		got, err := Marshal(test.g, test.name, test.prefix, "\t", test.strict)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if string(got) != test.want {
			t.Errorf("unexpected DOT result for test %d:\ngot: %s\nwant:%s", i, got, test.want)
		}
	}
}
