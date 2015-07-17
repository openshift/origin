package graph

import (
	"testing"
)

func TestMultipleEdgeKindsBetweenTheSameNodes(t *testing.T) {
	g := New()

	fooNode := makeTestNode(g, "foo")
	barNode := makeTestNode(g, "bar")

	g.AddEdge(fooNode, barNode, "first")
	g.AddEdge(fooNode, barNode, "second")

	edge := g.Edge(fooNode, barNode)
	if !g.EdgeKinds(edge).Has("first") {
		t.Errorf("expected first, got %v", edge)
	}
	if !g.EdgeKinds(edge).Has("second") {
		t.Errorf("expected second, got %v", edge)
	}
}
