package genericgraph

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/gonum/graph"
	"github.com/gonum/graph/encoding/dot"
	"github.com/gonum/graph/simple"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Node struct {
	simple.Node
	UniqueName
}

// DOTAttributes implements an attribute getter for the DOT encoding
func (n Node) DOTAttributes() []dot.Attribute {
	return []dot.Attribute{{Key: "label", Value: fmt.Sprintf("%q", n.UniqueName)}}
}

// ExistenceChecker is an interface for those nodes that can be created without a backing object.
// This can happen when a node wants an edge to a non-existent node.  We know the node should exist,
// The graph needs something in that location to track the information we have about the node, but the
// backing object doesn't exist.
type ExistenceChecker interface {
	// Found returns false if the node represents an object that we don't have the backing object for
	Found() bool
}

type UniqueName string

type UniqueNameFunc func(obj interface{}) UniqueName

func (n UniqueName) UniqueName() string {
	return string(n)
}

func (n UniqueName) String() string {
	return string(n)
}

type uniqueNamer interface {
	UniqueName() UniqueName
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
	AddEdge(from, to graph.Node, edgeKind string)
}

type MutableUniqueGraph interface {
	graph.DirectedBuilder
	MutableDirectedEdge
	UniqueNodeInitializer
	NodeFinder
}

type Edge struct {
	simple.Edge
	kinds sets.String
}

func NewEdge(from, to graph.Node, weight float64, kinds ...string) Edge {
	return Edge{Edge: simple.Edge{F: from, T: to, W: weight}, kinds: sets.NewString(kinds...)}
}

func (e Edge) Kinds() sets.String {
	return e.kinds
}

func (e Edge) IsKind(kind string) bool {
	return e.kinds.Has(kind)
}

// DOTAttributes implements an attribute getter for the DOT encoding
func (e Edge) DOTAttributes() []dot.Attribute {
	return []dot.Attribute{{Key: "label", Value: fmt.Sprintf("%q", strings.Join(e.Kinds().List(), ","))}}
}

type GraphDescriber interface {
	Name(node graph.Node) string
	Kind(node graph.Node) string
	Object(node graph.Node) interface{}
	EdgeKinds(edge graph.Edge) sets.String
}

type Interface interface {
	graph.DirectedBuilder

	GraphDescriber
	MutableDirectedEdge
	UniqueNodeInitializer
	NodeFinder
}

type Namer interface {
	ResourceName(obj interface{}) string
}

type namer struct{}

var DefaultNamer Namer = namer{}

func (namer) ResourceName(obj interface{}) string {
	switch t := obj.(type) {
	case uniqueNamer:
		return t.UniqueName().String()
	default:
		return reflect.TypeOf(obj).String()
	}
}

type Graph struct {
	// the standard graph
	graph.DirectedBuilder
	// helper methods for switching on the kind and types of the node
	GraphDescriber

	// exposes the public interface for adding nodes
	uniqueNamedGraph
	// the internal graph object, which allows edges and nodes to be directly added
	internal *simple.DirectedGraph
}

// Graph must implement MutableUniqueGraph
var _ MutableUniqueGraph = Graph{}

// New initializes a graph from input to output.
func New() Graph {
	g := simple.NewDirectedGraph(1.0, 0.0)
	return Graph{
		DirectedBuilder: g,
		GraphDescriber:  typedGraph{},

		uniqueNamedGraph: newUniqueNamedGraph(g),

		internal: g,
	}
}

func (g Graph) String() string {
	ret := ""

	nodes := g.Nodes()
	sort.Sort(ByID(nodes))
	for _, node := range nodes {
		ret += fmt.Sprintf("%d: %v\n", node.ID(), g.GraphDescriber.Name(node))

		// can't use SuccessorEdges, because I want stable ordering
		successors := g.From(node)
		sort.Sort(ByID(successors))
		for _, successor := range successors {
			edge := g.Edge(node, successor)
			kinds := g.EdgeKinds(edge)
			for _, kind := range kinds.List() {
				ret += fmt.Sprintf("\t%v to %d: %v\n", kind, successor.ID(), g.GraphDescriber.Name(successor))
			}
		}
	}

	return ret
}

func (g Graph) Edges() []graph.Edge {
	return g.internal.Edges()
}

func (g Graph) RemoveEdge(e graph.Edge) {
	g.internal.RemoveEdge(e)
}

func (g Graph) RemoveNode(node graph.Node) {
	g.internal.RemoveNode(node)
}

// ByID is a sorted group of nodes by ID
type ByID []graph.Node

func (m ByID) Len() int      { return len(m) }
func (m ByID) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m ByID) Less(i, j int) bool {
	return m[i].ID() < m[j].ID()
}

// NodesByKind returns all the nodes of the graph with the provided kinds
func (g Graph) NodesByKind(nodeKinds ...string) []graph.Node {
	ret := []graph.Node{}

	kinds := sets.NewString(nodeKinds...)
	for _, node := range g.internal.Nodes() {
		if kinds.Has(g.Kind(node)) {
			ret = append(ret, node)
		}
	}

	return ret
}

// PredecessorEdges invokes fn with all of the predecessor edges of node that have the specified
// edge kind.
func (g Graph) PredecessorEdges(node graph.Node, fn EdgeFunc, edgeKinds ...string) {
	for _, n := range g.To(node) {
		edge := g.Edge(n, node)
		kinds := g.EdgeKinds(edge)

		if kinds.HasAny(edgeKinds...) {
			fn(g, n, node, kinds)
		}
	}
}

// SuccessorEdges invokes fn with all of the successor edges of node that have the specified
// edge kind.
func (g Graph) SuccessorEdges(node graph.Node, fn EdgeFunc, edgeKinds ...string) {
	for _, n := range g.From(node) {
		edge := g.Edge(node, n)
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

	for _, n := range g.From(node) {
		edge := g.Edge(node, n)
		if edge == nil {
			continue
		}

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

	for _, n := range g.To(node) {
		edge := g.Edge(n, node)
		if edge == nil {
			continue
		}

		if len(edgeKinds) == 0 || g.EdgeKinds(edge).HasAny(edgeKinds...) {
			ret = append(ret, edge)
		}
	}

	return ret
}

// PredecessorNodesByEdgeKind returns all the predecessor nodes of the given node
// that can be reached via edges of the provided kinds
func (g Graph) PredecessorNodesByEdgeKind(node graph.Node, edgeKinds ...string) []graph.Node {
	ret := []graph.Node{}

	for _, inboundEdges := range g.InboundEdges(node, edgeKinds...) {
		ret = append(ret, inboundEdges.From())
	}

	return ret
}

// SuccessorNodesByEdgeKind returns all the successor nodes of the given node
// that can be reached via edges of the provided kinds
func (g Graph) SuccessorNodesByEdgeKind(node graph.Node, edgeKinds ...string) []graph.Node {
	ret := []graph.Node{}

	for _, outboundEdge := range g.OutboundEdges(node, edgeKinds...) {
		ret = append(ret, outboundEdge.To())
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

func (g Graph) AddNode(n graph.Node) {
	g.internal.AddNode(n)
}

// AddEdge implements MutableUniqueGraph
func (g Graph) AddEdge(from, to graph.Node, edgeKind string) {
	// a Contains edge has semantic meaning for osgraph.Graph objects.  It never makes sense
	// to allow a single object to be "contained" by multiple nodes.
	if edgeKind == ContainsEdgeKind {
		// check incoming edges on the 'to' node to be certain that we aren't already contained
		containsEdges := g.InboundEdges(to, ContainsEdgeKind)
		if len(containsEdges) != 0 {
			// TODO consider changing the AddEdge API to make this cleaner.  This is a pretty severe programming error
			panic(fmt.Sprintf("%v is already contained by %v", to, containsEdges))
		}
	}

	kinds := sets.NewString(edgeKind)
	if existingEdge := g.Edge(from, to); existingEdge != nil {
		kinds.Insert(g.EdgeKinds(existingEdge).List()...)
	}

	g.internal.SetEdge(NewEdge(from, to, 1.0, kinds.List()...))
}

// addEdges adds the specified edges, filtered by the provided edge connection
// function.
func (g Graph) addEdges(edges []graph.Edge, fn EdgeFunc) {
	for _, e := range edges {
		switch t := e.(type) {
		case Edge:
			if fn(g, t.From(), t.To(), t.Kinds()) {
				g.internal.SetEdge(t)
			}
		case simple.Edge:
			if fn(g, t.From(), t.To(), sets.NewString()) {
				g.internal.SetEdge(t)
			}
		default:
			panic("bad edge")
		}
	}
}

// NodeFunc is passed a new graph, a node in the graph, and should return true if the
// node should be included.
type NodeFunc func(g Interface, n graph.Node) bool

// NodesOfKind returns a new NodeFunc accepting the provided kinds of nodes
// If no kinds are specified, the returned NodeFunc will accept all nodes
func NodesOfKind(kinds ...string) NodeFunc {
	if len(kinds) == 0 {
		return func(g Interface, n graph.Node) bool {
			return true
		}
	}

	allowedKinds := sets.NewString(kinds...)
	return func(g Interface, n graph.Node) bool {
		return allowedKinds.Has(g.Kind(n))
	}
}

// EdgeFunc is passed a new graph, an edge in the current graph, and should mutate
// the new graph as needed. If true is returned, the existing edge will be added to the graph.
type EdgeFunc func(g Interface, from, to graph.Node, edgeKinds sets.String) bool

// EdgesOfKind returns a new EdgeFunc accepting the provided kinds of edges
// If no kinds are specified, the returned EdgeFunc will accept all edges
func EdgesOfKind(kinds ...string) EdgeFunc {
	if len(kinds) == 0 {
		return func(g Interface, from, to graph.Node, edgeKinds sets.String) bool {
			return true
		}
	}

	allowedKinds := sets.NewString(kinds...)
	return func(g Interface, from, to graph.Node, edgeKinds sets.String) bool {
		return allowedKinds.HasAny(edgeKinds.List()...)
	}
}

// RemoveInboundEdges returns a new EdgeFunc dismissing any inbound edges to
// the provided set of nodes
func RemoveInboundEdges(nodes []graph.Node) EdgeFunc {
	return func(g Interface, from, to graph.Node, edgeKinds sets.String) bool {
		for _, node := range nodes {
			if node == to {
				return false
			}
		}
		return true
	}
}

func RemoveOutboundEdges(nodes []graph.Node) EdgeFunc {
	return func(g Interface, from, to graph.Node, edgeKinds sets.String) bool {
		for _, node := range nodes {
			if node == from {
				return false
			}
		}
		return true
	}
}

// EdgeSubgraph returns the directed subgraph with only the edges that match the
// provided function.
func (g Graph) EdgeSubgraph(edgeFn EdgeFunc) Graph {
	out := New()
	for _, node := range g.Nodes() {
		out.internal.AddNode(node)
	}
	out.addEdges(g.internal.Edges(), edgeFn)
	return out
}

// Subgraph returns the directed subgraph with only the nodes and edges that match the
// provided functions.
func (g Graph) Subgraph(nodeFn NodeFunc, edgeFn EdgeFunc) Graph {
	out := New()
	for _, node := range g.Nodes() {
		if nodeFn(out, node) {
			out.internal.AddNode(node)
		}
	}
	out.addEdges(g.internal.Edges(), edgeFn)
	return out
}

// SubgraphWithNodes returns the directed subgraph with only the listed nodes and edges that
// match the provided function.
func (g Graph) SubgraphWithNodes(nodes []graph.Node, fn EdgeFunc) Graph {
	out := New()
	for _, node := range nodes {
		out.internal.AddNode(node)
	}
	out.addEdges(g.internal.Edges(), fn)
	return out
}

// ExistingDirectEdge returns true if both from and to already exist in the graph and the edge kind is
// not ReferencedByEdgeKind (the generic reverse edge kind). This will purge the graph of any
// edges created by AddReversedEdge.
func ExistingDirectEdge(g Interface, from, to graph.Node, edgeKinds sets.String) bool {
	return !edgeKinds.Has(ReferencedByEdgeKind) && g.Has(from) && g.Has(to)
}

// ReverseExistingDirectEdge reverses the order of the edge and drops the existing edge only if
// both from and to already exist in the graph and the edge kind is not ReferencedByEdgeKind
// (the generic reverse edge kind).
func ReverseExistingDirectEdge(g Interface, from, to graph.Node, edgeKinds sets.String) bool {
	return ExistingDirectEdge(g, from, to, edgeKinds) && ReverseGraphEdge(g, from, to, edgeKinds)
}

// ReverseGraphEdge reverses the order of the edge and drops the existing edge.
func ReverseGraphEdge(g Interface, from, to graph.Node, edgeKinds sets.String) bool {
	for edgeKind := range edgeKinds {
		g.AddEdge(to, from, edgeKind)
	}
	return false
}

// AddReversedEdge adds a reversed edge for every passed edge and preserves the existing
// edge. Used to convert a one directional edge into a bidirectional edge, but will
// create duplicate edges if a bidirectional edge between two nodes already exists.
func AddReversedEdge(g Interface, from, to graph.Node, edgeKinds sets.String) bool {
	g.AddEdge(to, from, ReferencedByEdgeKind)
	return true
}

type uniqueNamedGraph struct {
	graph.Builder
	names map[UniqueName]graph.Node
}

func newUniqueNamedGraph(g graph.Builder) uniqueNamedGraph {
	return uniqueNamedGraph{
		Builder: g,
		names:   make(map[UniqueName]graph.Node),
	}
}

func (g uniqueNamedGraph) FindOrCreate(name UniqueName, fn NodeInitializerFunc) (graph.Node, bool) {
	if node, ok := g.names[name]; ok {
		return node, true
	}
	id := g.NewNodeID()
	node := fn(Node{simple.Node(id), name})
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

func (g typedGraph) Name(node graph.Node) string {
	switch t := node.(type) {
	case fmt.Stringer:
		return t.String()
	case uniqueNamer:
		return t.UniqueName().String()
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

func (g typedGraph) EdgeKinds(edge graph.Edge) sets.String {
	var e Edge
	switch t := edge.(type) {
	case Edge:
		e = t
	case simple.Edge:
		e = Edge{Edge: t}
	default:
		return sets.NewString(UnknownEdgeKind)
	}
	return e.Kinds()
}

func NodesByKind(g Interface, nodes []graph.Node, kinds ...string) [][]graph.Node {
	buckets := make(map[string]int)
	for i, kind := range kinds {
		buckets[kind] = i
	}
	if nodes == nil {
		nodes = g.Nodes()
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

// IsFromDifferentNamespace returns if a node is in a different namespace
// than the one provided.
func IsFromDifferentNamespace(namespace string, node graph.Node) bool {
	potentiallySyntheticNode, ok := node.(ExistenceChecker)
	if !ok || potentiallySyntheticNode.Found() {
		return false
	}
	objectified, ok := node.(objectifier)
	if !ok {
		return false
	}
	object, err := meta.Accessor(objectified)
	if err != nil {
		return false
	}
	return object.GetNamespace() != namespace
}
