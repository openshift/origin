package graph

import (
	"fmt"
	"io"
	"sort"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

type Node struct {
	concrete.Node
	UniqueName
}

// ExistenceChecker is an interface for those nodes that can be created without a backing object.
// This can happen when a node wants an edge to a non-existent node.  We know the node should exist,
// The graph needs something in that location to track the information we have about the node, but the
// backing object doesn't exist.
type ExistenceChecker interface {
	// Found returns true if the node represents an object that we don't have the backing object for
	Found() bool
}

type ResourceNode interface {
	ResourceString() string
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
	kinds util.StringSet
}

func NewEdge(head, tail graph.Node, kinds ...string) Edge {
	return Edge{concrete.Edge{head, tail}, util.NewStringSet(kinds...)}
}

func (e Edge) Kinds() util.StringSet {
	return e.kinds
}

func (e Edge) IsKind(kind string) bool {
	return e.kinds.Has(kind)
}

type GraphDescriber interface {
	Name(node graph.Node) string
	Kind(node graph.Node) string
	Object(node graph.Node) interface{}
	EdgeKinds(edge graph.Edge) util.StringSet
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

func (g Graph) String() string {
	ret := ""

	nodeList := g.NodeList()
	sort.Sort(SortedNodeList(nodeList))
	for _, node := range nodeList {
		ret += fmt.Sprintf("%d: %v\n", node.ID(), g.GraphDescriber.Name(node))

		// can't use SuccessorEdges, because I want stable ordering
		successors := g.Successors(node)
		sort.Sort(SortedNodeList(successors))
		for _, successor := range successors {
			edge := g.EdgeBetween(node, successor)
			kinds := g.EdgeKinds(edge)
			for _, kind := range kinds.List() {
				ret += fmt.Sprintf("\t%v to %d: %v\n", kind, successor.ID(), g.GraphDescriber.Name(successor))
			}
		}
	}

	return ret
}

type SortedNodeList []graph.Node

func (m SortedNodeList) Len() int      { return len(m) }
func (m SortedNodeList) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m SortedNodeList) Less(i, j int) bool {
	return m[i].ID() < m[j].ID()
}

// SyntheticNodes returns back the set of nodes that were created in response to edge requests, but did not exist
func (g Graph) SyntheticNodes() []graph.Node {
	ret := []graph.Node{}

	nodeList := g.NodeList()
	sort.Sort(SortedNodeList(nodeList))
	for _, node := range nodeList {
		if potentiallySyntheticNode, ok := node.(ExistenceChecker); ok {
			if !potentiallySyntheticNode.Found() {
				ret = append(ret, node)
			}
		}
	}

	return ret
}

func (g Graph) NodesByKind(nodeKinds ...string) []graph.Node {
	ret := []graph.Node{}

	kinds := util.NewStringSet(nodeKinds...)
	for _, node := range g.NodeList() {
		if kinds.Has(g.Kind(node)) {
			ret = append(ret, node)
		}
	}

	return ret
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
func (g Graph) PredecessorEdges(node graph.Node, fn EdgeFunc, edgeKinds ...string) {
	for _, n := range g.Predecessors(node) {
		edge := g.EdgeBetween(n, node)
		kinds := g.EdgeKinds(edge)

		if kinds.HasAny(edgeKinds...) {
			fn(g, n, node, kinds)
		}
	}
}

// SuccessorEdges invokes fn with all of the successor edges of node that have the specified
// edge kind.
func (g Graph) SuccessorEdges(node graph.Node, fn EdgeFunc, edgeKinds ...string) {
	for _, n := range g.Successors(node) {
		edge := g.EdgeBetween(node, n)
		kinds := g.EdgeKinds(edge)

		if kinds.HasAny(edgeKinds...) {
			fn(g, n, node, kinds)
		}
	}
}

// OutboundEdges returns all the outbound edges from node that are in the list of edgeKinds
// if edgeKinds is empty, then all edges are returned
func (g Graph) OutboundEdges(node graph.Node, edgeKinds ...string) []graph.Edge {
	ret := []graph.Edge{}

	for _, n := range g.Successors(node) {
		edge := g.EdgeBetween(n, node)
		if len(edgeKinds) == 0 || g.EdgeKinds(edge).HasAny(edgeKinds...) {
			ret = append(ret, edge)
		}
	}

	return ret
}

// InboundEdges returns all the inbound edges to node that are in the list of edgeKinds
// if edgeKinds is empty, then all edges are returned
func (g Graph) InboundEdges(node graph.Node, edgeKinds ...string) []graph.Edge {
	ret := []graph.Edge{}

	for _, n := range g.Predecessors(node) {
		edge := g.EdgeBetween(n, node)
		if len(edgeKinds) == 0 || g.EdgeKinds(edge).HasAny(edgeKinds...) {
			ret = append(ret, edge)
		}
	}

	return ret
}

func (g Graph) PredecessorNodesByEdgeKind(node graph.Node, edgeKinds ...string) []graph.Node {
	ret := []graph.Node{}

	for _, inboundEdges := range g.InboundEdges(node, edgeKinds...) {
		ret = append(ret, inboundEdges.Head())
	}

	return ret
}

func (g Graph) SuccessorNodesByEdgeKind(node graph.Node, edgeKinds ...string) []graph.Node {
	ret := []graph.Node{}

	for _, outboundEdge := range g.OutboundEdges(node, edgeKinds...) {
		ret = append(ret, outboundEdge.Tail())
	}

	return ret
}

func (g Graph) SuccessorNodesByNodeAndEdgeKind(node graph.Node, nodeKind, edgeKind string) []graph.Node {
	ret := []graph.Node{}

	for _, successor := range g.SuccessorNodesByEdgeKind(node, edgeKind) {
		if g.Kind(successor) != nodeKind {
			continue
		}

		ret = append(ret, successor)
	}

	return ret
}

func (g Graph) EdgeList() []graph.Edge {
	return g.internal.EdgeList()
}

func (g Graph) AddNode(n graph.Node) {
	g.internal.AddNode(n)
}

// AddEdge implements MutableUniqueGraph
func (g Graph) AddEdge(head, tail graph.Node, edgeKind string) {
	// a Contains edge has semantic meaning for osgraph.Graph objects.  It never makes sense
	// to allow a single object to be "contained" by multiple nodes.
	if edgeKind == ContainsEdgeKind {
		// check incoming edges on the tail to be certain that we aren't already contained
		containsEdges := g.InboundEdges(tail, ContainsEdgeKind)
		if len(containsEdges) != 0 {
			// TODO consider changing the AddEdge API to make this cleaner.  This is a pretty severe programming error
			panic(fmt.Sprintf("%v is already contained by %v", tail, containsEdges))
		}
	}

	kinds := util.NewStringSet(edgeKind)
	if existingEdge := g.EdgeBetween(head, tail); existingEdge != nil {
		kinds.Insert(g.EdgeKinds(existingEdge).List()...)
	}

	g.internal.AddDirectedEdge(NewEdge(head, tail, kinds.List()...), 1)
}

// addEdges adds the specified edges, filtered by the provided edge connection
// function.
func (g Graph) addEdges(edges []graph.Edge, fn EdgeFunc) {
	for _, e := range edges {
		switch t := e.(type) {
		case concrete.WeightedEdge:
			if fn(g, t.Head(), t.Tail(), t.Edge.(Edge).Kinds()) {
				g.internal.AddDirectedEdge(t.Edge.(Edge), t.Cost)
			}
		case Edge:
			if fn(g, t.Head(), t.Tail(), t.Kinds()) {
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
type EdgeFunc func(g Interface, head, tail graph.Node, edgeKinds util.StringSet) bool

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
func ExistingDirectEdge(g Interface, head, tail graph.Node, edgeKinds util.StringSet) bool {
	return !edgeKinds.Has(ReferencedByEdgeKind) && g.NodeExists(head) && g.NodeExists(tail)
}

// ReverseExistingDirectEdge reverses the order of the edge and drops the existing edge only if
// both head and tail already exist in the graph and the edge kind is not ReferencedByEdgeKind
// (the generic reverse edge kind).
func ReverseExistingDirectEdge(g Interface, head, tail graph.Node, edgeKinds util.StringSet) bool {
	return ExistingDirectEdge(g, head, tail, edgeKinds) && ReverseGraphEdge(g, head, tail, edgeKinds)
}

// ReverseGraphEdge reverses the order of the edge and drops the existing edge.
func ReverseGraphEdge(g Interface, head, tail graph.Node, edgeKinds util.StringSet) bool {
	for edgeKind := range edgeKinds {
		g.AddEdge(tail, head, edgeKind)
	}
	return false
}

// AddReversedEdge adds a reversed edge for every passed edge and preserves the existing
// edge. Used to convert a one directional edge into a bidirectional edge, but will
// create duplicate edges if a bidirectional edge between two nodes already exists.
func AddReversedEdge(g Interface, head, tail graph.Node, edgeKinds util.StringSet) bool {
	g.AddEdge(tail, head, ReferencedByEdgeKind)
	return true
}

// AddGraphEdgesTo returns an EdgeFunc that will add the selected edges to the passed
// graph.
func AddGraphEdgesTo(g Interface) EdgeFunc {
	return func(_ Interface, head, tail graph.Node, edgeKinds util.StringSet) bool {
		for edgeKind := range edgeKinds {
			g.AddEdge(head, tail, edgeKind)
		}

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

func (g typedGraph) EdgeKinds(edge graph.Edge) util.StringSet {
	var e Edge
	switch t := edge.(type) {
	case concrete.WeightedEdge:
		e = t.Edge.(Edge)
	case Edge:
		e = t
	default:
		return util.NewStringSet(UnknownEdgeKind)
	}
	return e.Kinds()
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
		for _, edgeKind := range g.EdgeKinds(edge).List() {
			fmt.Fprintf(out, "edge %d -> %d : %d\n", edge.Head().ID(), edge.Head().ID(), edgeKind)
		}
	}
}
