package analyze

import (
	"github.com/openshift/origin/tools/depcheck/pkg/graph"
)

func FindExclusiveDependencies(g *graph.MutableDirectedGraph, targetNodes []*graph.Node) []*graph.Node {
	newGraph := g.Copy()
	for _, target := range targetNodes {
		newGraph.RemoveNode(target)
	}

	return newGraph.PruneOrphans()
}
