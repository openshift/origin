package nodes

import (
	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	depoyapi "github.com/openshift/origin/pkg/deploy/api"
)

// EnsureDeploymentConfigNode adds the provided deployment config to the graph if it does not exist
func EnsureDeploymentConfigNode(g osgraph.MutableUniqueGraph, dc *depoyapi.DeploymentConfig) *DeploymentConfigNode {
	dcName := DeploymentConfigNodeName(dc)
	dcNode := osgraph.EnsureUnique(
		g,
		dcName,
		func(node osgraph.Node) graph.Node {
			return &DeploymentConfigNode{Node: node, DeploymentConfig: dc}
		},
	).(*DeploymentConfigNode)

	rcSpecNode := kubegraph.EnsureReplicationControllerSpecNode(g, &dc.Template.ControllerTemplate, dcName)
	g.AddEdge(dcNode, rcSpecNode, osgraph.ContainsEdgeKind)

	return dcNode
}
