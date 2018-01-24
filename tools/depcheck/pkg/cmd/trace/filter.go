package trace

import (
	"fmt"
	"strings"

	"github.com/gonum/graph/concrete"

	depgraph "github.com/openshift/origin/tools/depcheck/pkg/graph"
)

// FilterPackages receives a graph and a set of packagePrefixes contained within the graph.
// Returns a new graph with the sub-tree for each node matching the packagePrefix collapsed
// into just that node. Relationships among packagePrefixes are kept: edges originating from
// packagePrefix subpackages are re-written to originate from the packagePrefix, and edges
// terminating at packagePrefix subpackages are re-written to terminate at the packagePrefix.
func FilterPackages(g *depgraph.MutableDirectedGraph, packagePrefixes []string) (*depgraph.MutableDirectedGraph, error) {
	collapsedGraph := depgraph.NewMutableDirectedGraph(concrete.NewDirectedGraph())

	// copy all nodes to new graph
	for _, n := range g.Nodes() {
		node, ok := n.(*depgraph.Node)
		if !ok {
			continue
		}

		collapsedNodeName := getFilteredNodeName(packagePrefixes, node.UniqueName)
		_, exists := collapsedGraph.NodeByName(collapsedNodeName)
		if exists {
			continue
		}

		err := collapsedGraph.AddNode(&depgraph.Node{
			UniqueName: collapsedNodeName,
			Id:         n.ID(),
			LabelName:  labelNameForNode(collapsedNodeName),
		})
		if err != nil {
			return nil, err
		}
	}

	// add edges to collapsed graph
	for _, from := range g.Nodes() {
		node, ok := from.(*depgraph.Node)
		if !ok {
			return nil, fmt.Errorf("expected nodes in graph to be of type *depgraph.Node")
		}

		fromNodeName := getFilteredNodeName(packagePrefixes, node.UniqueName)
		fromNode, exists := collapsedGraph.NodeByName(fromNodeName)
		if !exists {
			return nil, fmt.Errorf("expected node with name %q to exist in collapsed graph", fromNodeName)
		}

		for _, to := range g.From(from) {
			node, ok := to.(*depgraph.Node)
			if !ok {
				return nil, fmt.Errorf("expected nodes in graph to be of type *depgraph.Node")
			}

			toNodeName := getFilteredNodeName(packagePrefixes, node.UniqueName)
			if fromNodeName == toNodeName {
				continue
			}

			toNode, exists := collapsedGraph.NodeByName(toNodeName)
			if !exists {
				return nil, fmt.Errorf("expected node with name %q to exist in collapsed graph", toNodeName)
			}

			if collapsedGraph.HasEdgeFromTo(fromNode, toNode) {
				continue
			}

			collapsedGraph.SetEdge(concrete.Edge{
				F: fromNode,
				T: toNode,
			}, 0)
		}
	}

	return collapsedGraph, nil
}

func getFilteredNodeName(collapsedPrefixes []string, packageName string) string {
	for _, prefix := range collapsedPrefixes {
		if strings.HasPrefix(packageName, prefix) {
			return prefix
		}
	}

	return packageName
}
