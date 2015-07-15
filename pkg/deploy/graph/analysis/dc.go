package analysis

import (
	"github.com/gonum/graph"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	"github.com/openshift/origin/pkg/api/graph/graphview"
	kubeanalysis "github.com/openshift/origin/pkg/api/kubegraph/analysis"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
)

// DescendentNodesByNodeKind starts at the root navigates down the root.  Every edge is checked against the edgeChecker
// to determine whether or not to follow it.  The nodes at the tail end of every chased edge are then checked against the
// the targetNodeKind.  Matches are added to the return and every checked node then has its edges checked: lather, rinse, repeat
func DescendentNodesByNodeKind(g osgraph.Graph, visitedNodes graphview.IntSet, node graph.Node, targetNodeKind string, edgeChecker osgraph.EdgeFunc) []graph.Node {
	if visitedNodes.Has(node.ID()) {
		return []graph.Node{}
	}
	visitedNodes.Insert(node.ID())

	ret := []graph.Node{}
	for _, successor := range g.Successors(node) {
		edge := g.EdgeBetween(node, successor)

		if edgeChecker(osgraph.New(), node, successor, g.EdgeKinds(edge)) {
			if g.Kind(successor) == targetNodeKind {
				ret = append(ret, successor)
			}

			ret = append(ret, DescendentNodesByNodeKind(g, visitedNodes, successor, targetNodeKind, edgeChecker)...)
		}
	}

	return ret
}

// CheckMountedSecrets checks to be sure that all the referenced secrets are mountable (by service account) and present (not synthetic)
func CheckMountedSecrets(g osgraph.Graph, dcNode *deploygraph.DeploymentConfigNode) ( /*unmountable secrets*/ []*kubegraph.SecretNode /*unresolved secrets*/, []*kubegraph.SecretNode) {
	podSpecs := DescendentNodesByNodeKind(g, graphview.IntSet{}, dcNode, kubegraph.PodSpecNodeKind, func(g osgraph.Interface, head, tail graph.Node, edgeKinds util.StringSet) bool {
		if edgeKinds.Has(osgraph.ContainsEdgeKind) {
			return true
		}
		return false
	})

	if len(podSpecs) > 0 {
		return kubeanalysis.CheckMountedSecrets(g, podSpecs[0].(*kubegraph.PodSpecNode))
	}

	return []*kubegraph.SecretNode{}, []*kubegraph.SecretNode{}
}
