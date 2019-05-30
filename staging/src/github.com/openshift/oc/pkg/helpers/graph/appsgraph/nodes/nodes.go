package nodes

import (
	"github.com/gonum/graph"

	appsv1 "github.com/openshift/api/apps/v1"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	kubegraph "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
)

// EnsureDeploymentConfigNode adds the provided deployment config to the graph if it does not exist
func EnsureDeploymentConfigNode(g osgraph.MutableUniqueGraph, dc *appsv1.DeploymentConfig) *DeploymentConfigNode {
	dcName := DeploymentConfigNodeName(dc)
	dcNode := osgraph.EnsureUnique(
		g,
		dcName,
		func(node osgraph.Node) graph.Node {
			return &DeploymentConfigNode{Node: node, DeploymentConfig: dc, IsFound: true}
		},
	).(*DeploymentConfigNode)

	if dc.Spec.Template != nil {
		podTemplateSpecNode := kubegraph.EnsurePodTemplateSpecNode(g, dc.Spec.Template, dc.Namespace, dcName)
		g.AddEdge(dcNode, podTemplateSpecNode, osgraph.ContainsEdgeKind)
	}

	return dcNode
}

func FindOrCreateSyntheticDeploymentConfigNode(g osgraph.MutableUniqueGraph, dc *appsv1.DeploymentConfig) *DeploymentConfigNode {
	return osgraph.EnsureUnique(
		g,
		DeploymentConfigNodeName(dc),
		func(node osgraph.Node) graph.Node {
			return &DeploymentConfigNode{Node: node, DeploymentConfig: dc, IsFound: false}
		},
	).(*DeploymentConfigNode)
}
