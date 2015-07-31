package depthfirst

import (
	"testing"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
	"github.com/gonum/graph/internal"
)

var count = 0

// DepthFirst implements stateful depth-first graph traversal.
// Modifies behavior of visitor.DepthFirst to allow nodes to be visited multiple
// times as long as they're not in the current stack
type DepthFirst struct {
	EdgeFilter func(graph.Edge) bool
	Visit      func(u, v graph.Node)
	stack      internal.NodeStack
}

// Walk performs a depth-first traversal of the graph g starting from the given node
func (d *DepthFirst) Walk(g graph.Graph, from graph.Node, until func(graph.Node) bool) graph.Node {
	d.stack.Push(from)

	for d.stack.Len() > 0 {
		t := d.stack.Pop()
		if until != nil && until(t) {
			return t
		}
		for _, n := range g.From(t) {
			if d.EdgeFilter != nil && !d.EdgeFilter(g.Edge(t, n)) {
				continue
			}
			if d.Visited(n.ID()) {
				continue
			}
			if d.Visit != nil {
				d.Visit(t, n)
			}
			d.stack.Push(n)

			count++
			if count > 100 {
				return nil
			}
		}
	}
	return nil
}

func (d *DepthFirst) Visited(id int) bool {
	for _, n := range d.stack {
		if n.ID() == id {
			return true
		}
	}
	return false
}

func TestDF(t *testing.T) {
	g := concrete.NewDirectedGraph()

	a := concrete.Node(g.NewNodeID())
	b := concrete.Node(g.NewNodeID())

	g.AddNode(a)
	g.AddNode(b)
	g.SetEdge(concrete.Edge{a, b}, 1)
	g.SetEdge(concrete.Edge{b, a}, 1)

	df := &DepthFirst{
		EdgeFilter: func(graph.Edge) bool {
			return true
		},
		Visit: func(u, v graph.Node) {
			t.Logf("%d -> %d\n", u.ID(), v.ID())
		},
	}

	df.Walk(g, a, nil)

	if count > 100 {
		t.Errorf("looped")
	}
}
