package graph

import (
	"fmt"
	"io"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
)

type Node struct {
	concrete.Node
	UniqueName
}

type UniqueName string

type UniqueNameFunc func(obj interface{}) UniqueName

func (n UniqueName) UniqueName() string {
	return string(n)
}

type uniqueNamer interface {
	UniqueName() string
}

type NodeFinder interface {
	Find(name UniqueName) graph.Node
}

// UniqueNodeInitializer is a graph that allows nodes with a unique name to be added without duplication.
// If the node is newly added, true will be returned.
type UniqueNodeInitializer interface {
	FindOrCreate(name UniqueName, fn NodeInitializerFunc) (graph.Node, bool)
}

type NodeInitializerFunc func(Node) graph.Node

func EnsureUnique(g UniqueNodeInitializer, name UniqueName, fn NodeInitializerFunc) graph.Node {
	node, _ := g.FindOrCreate(name, fn)
	return node
}

type MutableDirectedEdge interface {
	AddEdge(head, tail graph.Node, edgeKind string)
}

type MutableUniqueGraph interface {
	graph.Mutable
	MutableDirectedEdge
	UniqueNodeInitializer
	NodeFinder
}

type Edge struct {
	concrete.Edge
	K string
}

func NewEdge(head, tail graph.Node, kind string) Edge {
	return Edge{concrete.Edge{head, tail}, kind}
}

func (e Edge) Kind() string {
	return e.K
}

type GraphDescriber interface {
	Name(node graph.Node) string
	Kind(node graph.Node) string
	Object(node graph.Node) interface{}
	EdgeKind(edge graph.Edge) string
}

type Interface interface {
	graph.DirectedGraph
	graph.EdgeLister

	GraphDescriber
	MutableUniqueGraph
}

type Graph struct {
	// the standard graph
	graph.DirectedGraph
	// helper methods for switching on the kind and types of the node
	GraphDescriber

	// exposes the public interface for adding nodes
	uniqueNamedGraph
	// the internal graph object, which allows edges and nodes to be directly added
	internal *concrete.DirectedGraph
}

// Graph must implement MutableUniqueGraph
var _ MutableUniqueGraph = Graph{}

// New initializes a graph from input to output.
func New() Graph {
	g := concrete.NewDirectedGraph()
	return Graph{
		DirectedGraph:  g,
		GraphDescriber: typedGraph{},

		uniqueNamedGraph: newUniqueNamedGraph(g),

		internal: g,
	}
}

// RootNodes returns all the roots of this graph.
func (g Graph) RootNodes() []graph.Node {
	roots := []graph.Node{}
	for _, n := range g.internal.NodeList() {
		if len(g.internal.Predecessors(n)) != 0 {
			continue
		}
		roots = append(roots, n)
	}
	return roots
}

// PredecessorEdges invokes fn with all of the predecessor edges of node that have the specified
// edge kind.
func (g Graph) PredecessorEdges(node graph.Node, fn EdgeFunc, edgeKind ...string) {
	for _, n := range g.Predecessors(node) {
		edge := g.EdgeBetween(n, node)
		kind := g.EdgeKind(edge)
		for _, allowed := range edgeKind {
			if allowed != kind {
				continue
			}
			fn(g, n, node, kind)
			break
		}
	}
}

// SuccessorEdges invokes fn with all of the successor edges of node that have the specified
// edge kind.
func (g Graph) SuccessorEdges(node graph.Node, fn EdgeFunc, edgeKind ...string) {
	for _, n := range g.Successors(node) {
		edge := g.EdgeBetween(node, n)
		kind := g.EdgeKind(edge)
		for _, allowed := range edgeKind {
			if allowed != kind {
				continue
			}
			fn(g, node, n, kind)
			break
		}
	}
}

func (g Graph) EdgeList() []graph.Edge {
	return g.internal.EdgeList()
}

func (g Graph) AddNode(n graph.Node) {
	g.internal.AddNode(n)
}

// AddEdge implements MutableUniqueGraph
func (g Graph) AddEdge(head, tail graph.Node, edgeKind string) {
	g.internal.AddDirectedEdge(NewEdge(head, tail, edgeKind), 1)
}

// addEdges adds the specified edges, filtered by the provided edge connection
// function.
func (g Graph) addEdges(edges []graph.Edge, fn EdgeFunc) {
	for _, e := range edges {
		switch t := e.(type) {
		case concrete.WeightedEdge:
			if fn(g, t.Head(), t.Tail(), t.Edge.(Edge).K) {
				g.internal.AddDirectedEdge(t.Edge.(Edge), t.Cost)
			}
		case Edge:
			if fn(g, t.Head(), t.Tail(), t.K) {
				g.internal.AddDirectedEdge(t, 1.0)
			}
		default:
			panic("bad edge")
		}
	}
}

// NodeFunc is passed a new graph, a node in the graph, and should return true if the
// node should be included.
type NodeFunc func(g Interface, n graph.Node) bool

// EdgeFunc is passed a new graph, an edge in the current graph, and should mutate
// the new graph as needed. If true is returned, the existing edge will be added to the graph.
type EdgeFunc func(g Interface, head, tail graph.Node, edgeKind string) bool

// EdgeSubgraph returns the directed subgraph with only the edges that match the
// provided function.
func (g Graph) EdgeSubgraph(edgeFn EdgeFunc) Graph {
	out := New()
	for _, node := range g.NodeList() {
		out.internal.AddNode(node)
	}
	out.addEdges(g.internal.EdgeList(), edgeFn)
	return out
}

// Subgraph returns the directed subgraph with only the nodes and edges that match the
// provided functions.
func (g Graph) Subgraph(nodeFn NodeFunc, edgeFn EdgeFunc) Graph {
	out := New()
	for _, node := range g.NodeList() {
		if nodeFn(out, node) {
			out.internal.AddNode(node)
		}
	}
	out.addEdges(g.internal.EdgeList(), edgeFn)
	return out
}

// SubgraphWithNodes returns the directed subgraph with only the listed nodes and edges that
// match the provided function.
func (g Graph) SubgraphWithNodes(nodes []graph.Node, fn EdgeFunc) Graph {
	out := New()
	for _, node := range nodes {
		out.internal.AddNode(node)
	}
	out.addEdges(g.internal.EdgeList(), fn)
	return out
}

// ConnectedEdgeSubgraph creates a new graph that iterates through all edges in the graph
// and includes all edges the provided function returns true for. Nodes not referenced by
// an edge will be dropped unless the function adds them explicitly.
func (g Graph) ConnectedEdgeSubgraph(fn EdgeFunc) Graph {
	out := New()
	out.addEdges(g.internal.EdgeList(), fn)
	return out
}

// AllNodes includes all nodes in the graph
func AllNodes(g Interface, node graph.Node) bool {
	return true
}

// ExistingDirectEdge returns true if both head and tail already exist in the graph and the edge kind is
// not ReferencedByEdgeKind (the generic reverse edge kind). This will purge the graph of any
// edges created by AddReversedEdge.
func ExistingDirectEdge(g Interface, head, tail graph.Node, edgeKind string) bool {
	return edgeKind != ReferencedByEdgeKind && g.NodeExists(head) && g.NodeExists(tail)
}

// ReverseExistingDirectEdge reverses the order of the edge and drops the existing edge only if
// both head and tail already exist in the graph and the edge kind is not ReferencedByEdgeKind
// (the generic reverse edge kind).
func ReverseExistingDirectEdge(g Interface, head, tail graph.Node, edgeKind string) bool {
	return ExistingDirectEdge(g, head, tail, edgeKind) && ReverseGraphEdge(g, head, tail, edgeKind)
}

// ReverseGraphEdge reverses the order of the edge and drops the existing edge.
func ReverseGraphEdge(g Interface, head, tail graph.Node, edgeKind string) bool {
	g.AddEdge(tail, head, edgeKind)
	return false
}

// AddReversedEdge adds a reversed edge for every passed edge and preserves the existing
// edge. Used to convert a one directional edge into a bidirectional edge, but will
// create duplicate edges if a bidirectional edge between two nodes already exists.
func AddReversedEdge(g Interface, head, tail graph.Node, edgeKind string) bool {
	g.AddEdge(tail, head, ReferencedByEdgeKind)
	return true
}

// AddGraphEdgesTo returns an EdgeFunc that will add the selected edges to the passed
// graph.
func AddGraphEdgesTo(g Interface) EdgeFunc {
	return func(_ Interface, head, tail graph.Node, edgeKind string) bool {
		g.AddEdge(head, tail, edgeKind)
		return false
	}
}

type uniqueNamedGraph struct {
	graph.Mutable
	names map[UniqueName]graph.Node
}

func newUniqueNamedGraph(g graph.Mutable) uniqueNamedGraph {
	return uniqueNamedGraph{
		Mutable: g,
		names:   make(map[UniqueName]graph.Node),
	}
}

func (g uniqueNamedGraph) FindOrCreate(name UniqueName, fn NodeInitializerFunc) (graph.Node, bool) {
	if node, ok := g.names[name]; ok {
		return node, true
	}
	id := g.NewNode().ID()
	node := fn(Node{concrete.Node(id), name})
	g.names[name] = node
	g.AddNode(node)
	return node, false
}

func (g uniqueNamedGraph) Find(name UniqueName) graph.Node {
	if node, ok := g.names[name]; ok {
		return node
	}
	return nil
}

type typedGraph struct{}

type stringer interface {
	String() string
}

func (g typedGraph) Name(node graph.Node) string {
	switch t := node.(type) {
	case stringer:
		return t.String()
	case uniqueNamer:
		return t.UniqueName()
	default:
		return fmt.Sprintf("<unknown:%d>", node.ID())
	}
}

type objectifier interface {
	Object() interface{}
}

func (g typedGraph) Object(node graph.Node) interface{} {
	switch t := node.(type) {
	case objectifier:
		return t.Object()
	default:
		return nil
	}
}

type kind interface {
	Kind() string
}

func (g typedGraph) Kind(node graph.Node) string {
	if k, ok := node.(kind); ok {
		return k.Kind()
	}
	return UnknownNodeKind
}

func (g typedGraph) EdgeKind(edge graph.Edge) string {
	var e Edge
	switch t := edge.(type) {
	case concrete.WeightedEdge:
		e = t.Edge.(Edge)
	case Edge:
		e = t
	default:
		return UnknownEdgeKind
	}
	return e.Kind()
}

type NodeSet map[int]struct{}

func (n NodeSet) Has(id int) bool {
	_, ok := n[id]
	return ok
}

func (n NodeSet) Add(id int) {
	n[id] = struct{}{}
}

func NodesByKind(g Interface, nodes []graph.Node, kinds ...string) [][]graph.Node {
	buckets := make(map[string]int)
	for i, kind := range kinds {
		buckets[kind] = i
	}
	if nodes == nil {
		nodes = g.NodeList()
	}

	last := len(kinds)
	result := make([][]graph.Node, last+1)
	for _, node := range nodes {
		if bucket, ok := buckets[g.Kind(node)]; ok {
			result[bucket] = append(result[bucket], node)
		} else {
			result[last] = append(result[last], node)
		}
	}
	return result
}

func pathCovered(path []graph.Node, paths map[int][]graph.Node) bool {
	l := len(path)
	for _, existing := range paths {
		if l >= len(existing) {
			continue
		}
		if pathEqual(path, existing) {
			return true
		}
	}
	return false
}

func pathEqual(a, b []graph.Node) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func Fprint(out io.Writer, g Graph) {
	for _, node := range g.NodeList() {
		fmt.Fprintf(out, "node %d %s\n", node.ID(), node)
	}
	for _, edge := range g.EdgeList() {
		fmt.Fprintf(out, "edge %d -> %d : %d\n", edge.Head().ID(), edge.Head().ID(), g.EdgeKind(edge))
	}
}
