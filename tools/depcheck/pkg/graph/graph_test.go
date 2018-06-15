package graph

import (
	"fmt"
	"testing"

	"github.com/gonum/graph/concrete"
)

type testNode struct {
	realNode *Node

	children []string
}

var testNodes = []*testNode{
	{
		realNode: &Node{
			UniqueName: "test/root",
		},
		children: []string{
			"test/root/one",
			"test/root/foo",
		},
	},
	{
		realNode: &Node{
			UniqueName: "test/root/one",
		},
		children: []string{
			"test/root/one/two",
		},
	},
	{
		realNode: &Node{
			UniqueName: "test/root/one/two",
		},
		children: []string{
			"test/root/one/two/three",
		},
	},
	{
		realNode: &Node{
			UniqueName: "test/root/one/two/three",
		},
		children: []string{
			"test/root/foo",
		},
	},
	{
		realNode: &Node{
			UniqueName: "test/root/foo",
		},
		children: []string{},
	},
}

func addNodes(g *MutableDirectedGraph) error {
	for _, n := range testNodes {
		n.realNode.Id = g.NewNodeID()
		err := g.AddNode(n.realNode)
		if err != nil {
			return err
		}
	}

	// add edges
	for _, n := range testNodes {
		for _, childName := range n.children {
			childNode, exists := g.NodeByName(childName)
			if !exists {
				return fmt.Errorf("expected testNode with name %q to exist", childName)
			}
			if g.HasEdgeFromTo(n.realNode, childNode) {
				return fmt.Errorf("attempt to add duplicate edge between %q and %q", n.realNode.UniqueName, childName)
			}

			g.SetEdge(concrete.Edge{
				F: n.realNode,
				T: childNode,
			}, 0)
		}
	}
	return nil
}

func TestMutableDirectedGraph_PruneOrphans(t *testing.T) {
	g := NewMutableDirectedGraph([]string{"test/root"})
	if err := addNodes(g); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testCases := []struct {
		name             string
		g                *MutableDirectedGraph
		originatingNodes []string
		removeNodes      []string
		expectOrphans    []string
	}{
		{
			name:          "pruning the root node orphans every test node",
			g:             g.Copy(),
			removeNodes:   []string{"test/root"},
			expectOrphans: []string{"test/root/one", "test/root/one/two", "test/root/one/two/three", "test/root/foo"},
		},
		{
			name:          "pruning non-root target node does not return an orphaned set containing a node with an inbound edge from outside of that tree",
			g:             g.Copy(),
			removeNodes:   []string{"test/root/one"},
			expectOrphans: []string{"test/root/one/two", "test/root/one/two/three"},
		},
		{
			name:        "pruning the parent of a node with multiple inbound edges does not result in the child node becoming orphaned",
			g:           g.Copy(),
			removeNodes: []string{"test/root/one/two/three"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, toRemove := range tc.removeNodes {
				toRemoveNode, exists := tc.g.NodeByName(toRemove)
				if !exists {
					t.Fatalf("expected node with name %q to exist in the graph", toRemove)
				}

				tc.g.RemoveNode(toRemoveNode.(*Node))
			}

			orphanedSet := tc.g.PruneOrphans()
			if len(orphanedSet) != len(tc.expectOrphans) {
				t.Fatalf("orphaned set mismatch: expected a set of %v orphans, but saw %v", len(tc.expectOrphans), len(orphanedSet))
			}

			for _, expected := range tc.expectOrphans {
				sawExpected := false
				for _, actualNode := range orphanedSet {
					if actualNode.UniqueName == expected {
						sawExpected = true
						break
					}
				}

				if !sawExpected {
					t.Fatalf("expected node %q to be included in the actual orphaned set", expected)
				}
			}
		})
	}
}
