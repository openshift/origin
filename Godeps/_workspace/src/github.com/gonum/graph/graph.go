// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph

// All a node needs to do is identify itself. This allows the user to pass in nodes more
// interesting than an int, but also allow us to reap the benefits of having a map-storable,
// comparable type.
type Node interface {
	ID() int
}

// Allows edges to do something more interesting that just be a group of nodes. While the methods
// are called Head and Tail, they are not considered directed unless the given interface specifies
// otherwise.
type Edge interface {
	Head() Node
	Tail() Node
}

// A Graph implements the behavior of an undirected graph.
//
// All methods in Graph are implicitly undirected. Graph algorithms that care about directionality
// will intelligently choose the DirectedGraph behavior if that interface is also implemented,
// even if the function itself only takes in a Graph (or a super-interface of graph).
type Graph interface {
	// NodeExists returns true when node is currently in the graph.
	NodeExists(Node) bool

	// NodeList returns a list of all nodes in no particular order, useful for
	// determining things like if a graph is fully connected. The caller is
	// free to modify this list. Implementations should construct a new list
	// and not return internal representation.
	NodeList() []Node

	// Neighbors returns all nodes connected by any edge to this node.
	Neighbors(Node) []Node

	// EdgeBetween returns an edge between node and neighbor such that
	// Head is one argument and Tail is the other. If no
	// such edge exists, this function returns nil.
	EdgeBetween(node, neighbor Node) Edge
}

// Directed graphs are characterized by having seperable Heads and Tails in their edges.
// That is, if node1 goes to node2, that does not necessarily imply that node2 goes to node1.
//
// While it's possible for a directed graph to have fully reciprocal edges (i.e. the graph is
// symmetric) -- it is not required to be. The graph is also required to implement Graph
// because in many cases it can be useful to know all neighbors regardless of direction.
type DirectedGraph interface {
	Graph
	// Successors gives the nodes connected by OUTBOUND edges.
	// If the graph is an undirected graph, this set is equal to Predecessors.
	Successors(Node) []Node

	// EdgeTo returns an edge between node and successor such that
	// Head returns node and Tail returns successor, if no
	// such edge exists, this function returns nil.
	EdgeTo(node, successor Node) Edge

	// Predecessors gives the nodes connected by INBOUND edges.
	// If the graph is an undirected graph, this set is equal to Successors.
	Predecessors(Node) []Node
}

// Returns all undirected edges in the graph
type EdgeLister interface {
	EdgeList() []Edge
}

type EdgeListGraph interface {
	Graph
	EdgeLister
}

// Returns all directed edges in the graph.
type DirectedEdgeLister interface {
	DirectedEdgeList() []Edge
}

type DirectedEdgeListGraph interface {
	Graph
	DirectedEdgeLister
}

// A crunch graph forces a sparse graph to become a dense graph. That is, if the node IDs are
// [1,4,9,7] it would "crunch" the ids into the contiguous block [0,1,2,3]. Order is not
// required to be preserved between the non-cruched and crunched instances (that means in
// the example above 0 may correspond to 4 or 7 or 9, not necessarily 1).
//
// All dense graphs must have the first ID as 0.
type CrunchGraph interface {
	Graph
	Crunch()
}

// A Graph that implements Coster has an actual cost between adjacent nodes, also known as a
// weighted graph. If a graph implements coster and a function needs to read cost (e.g. A*),
// this function will take precedence over the Uniform Cost function (all weights are 1) if "nil"
// is passed in for the function argument.
//
// If the argument is nil, or the edge is invalid for some reason, this should return math.Inf(1)
type Coster interface {
	Cost(Edge) float64
}

type CostGraph interface {
	Coster
	Graph
}

type CostDirectedGraph interface {
	Coster
	DirectedGraph
}

// A graph that implements HeuristicCoster implements a heuristic between any two given nodes.
// Like Coster, if a graph implements this and a function needs a heuristic cost (e.g. A*), this
// function will take precedence over the Null Heuristic (always returns 0) if "nil" is passed in
// for the function argument. If HeuristicCost is not intended to be used, it can be implemented as
// the null heuristic (always returns 0).
type HeuristicCoster interface {
	// HeuristicCost returns a heuristic cost between any two nodes.
	HeuristicCost(n1, n2 Node) float64
}

// A Mutable is a graph that can have arbitrary nodes and edges added or removed.
//
// Anything implementing Mutable is required to store the actual argument. So if AddNode(myNode) is
// called and later a user calls on the graph graph.NodeList(), the node added by AddNode must be
// an the exact node, not a new node with the same ID.
//
// In any case where conflict is possible (e.g. adding two nodes with the same ID), the later
// call always supercedes the earlier one.
//
// Functions will generally expect one of MutableGraph or MutableDirectedGraph and not Mutable
// itself. That said, any function that takes Mutable[x], the destination mutable should
// always be a different graph than the source.
type Mutable interface {
	// NewNode returns a node with a unique arbitrary ID.
	NewNode() Node

	// Adds a node to the graph. If this is called multiple times for the same ID, the newer node
	// overwrites the old one.
	AddNode(Node)

	// RemoveNode removes a node from the graph, as well as any edges
	// attached to it. If no such node exists, this is a no-op, not an error.
	RemoveNode(Node)
}

// MutableGraph is an interface ensuring the implementation of the ability to construct
// an arbitrary undirected graph. It is very important to note that any implementation
// of MutableGraph absolutely cannot safely implement the DirectedGraph interface.
//
// A MutableGraph is required to store any Edge argument in the same way Mutable must
// store a Node argument -- any retrieval call is required to return the exact supplied edge.
// This is what makes it incompatible with DirectedGraph.
//
// The reasoning is this: if you call AddUndirectedEdge(Edge{head,tail}); you are required
// to return the exact edge passed in when a retrieval method (EdgeTo/EdgeBetween) is called.
// If I call EdgeTo(tail,head), this means that since the edge exists, and was added as
// Edge{head,tail} this function MUST return Edge{head,tail}. However, EdgeTo requires this
// be returned as Edge{tail,head}. Thus there's a conflict that cannot be resolved between the
// two interface requirements.
type MutableGraph interface {
	CostGraph
	Mutable

	// Like EdgeBetween in Graph, AddUndirectedEdge adds an edge between two nodes.
	// If one or both nodes do not exist, the graph is expected to add them. However,
	// if the nodes already exist it should NOT replace existing nodes with e.Head() or
	// e.Tail(). Overwriting nodes should explicitly be done with another call to AddNode()
	AddUndirectedEdge(e Edge, cost float64)

	// RemoveEdge clears the stored edge between two nodes. Calling this will never
	// remove a node. If the edge does not exist this is a no-op, not an error.
	RemoveUndirectedEdge(Edge)
}

// MutableDirectedGraph is an interface that ensures one can construct an arbitrary directed
// graph. Naturally, a MutableDirectedGraph works for both undirected and directed cases,
// but simply using a MutableGraph may be cleaner. As the documentation for MutableGraph
// notes, however, a graph cannot safely implement MutableGraph and MutableDirectedGraph
// at the same time, because of the functionality of a EdgeTo in DirectedGraph.
type MutableDirectedGraph interface {
	CostDirectedGraph
	Mutable

	// Like EdgeTo in DirectedGraph, AddDirectedEdge adds an edge FROM head TO tail.
	// If one or both nodes do not exist, the graph is expected to add them. However,
	// if the nodes already exist it should NOT replace existing nodes with e.Head() or
	// e.Tail(). Overwriting nodes should explicitly be done with another call to AddNode()
	AddDirectedEdge(e Edge, cost float64)

	// Removes an edge FROM e.Head TO e.Tail. If no such edge exists, this is a no-op,
	// not an error.
	RemoveDirectedEdge(Edge)
}

// A function that returns the cost of following an edge
type CostFunc func(Edge) float64

// Estimates the cost of travelling between two nodes
type HeuristicCostFunc func(Node, Node) float64
