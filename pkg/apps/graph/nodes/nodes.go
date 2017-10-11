package nodes

import (
	"github.com/gonum/graph"

	kapisext "k8s.io/kubernetes/pkg/apis/extensions"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

// EnsureDaemonSetNode adds the provided daemon set to the graph if it does not exist
func EnsureDaemonSetNode(g osgraph.MutableUniqueGraph, ds *kapisext.DaemonSet) *DaemonSetNode {
	dsName := DaemonSetNodeName(ds)
	dsNode := osgraph.EnsureUnique(
		g,
		dsName,
		func(node osgraph.Node) graph.Node {
			return &DaemonSetNode{Node: node, DaemonSet: ds, IsFound: true}
		},
	).(*DaemonSetNode)

	podTemplateSpecNode := kubegraph.EnsurePodTemplateSpecNode(g, &ds.Spec.Template, ds.Namespace, dsName)
	g.AddEdge(dsNode, podTemplateSpecNode, osgraph.ContainsEdgeKind)

	return dsNode
}

func FindOrCreateSyntheticDaemonSetNode(g osgraph.MutableUniqueGraph, ds *kapisext.DaemonSet) *DaemonSetNode {
	return osgraph.EnsureUnique(
		g,
		DaemonSetNodeName(ds),
		func(node osgraph.Node) graph.Node {
			return &DaemonSetNode{Node: node, DaemonSet: ds, IsFound: false}
		},
	).(*DaemonSetNode)
}

// EnsureDeploymentConfigNode adds the provided deployment config to the graph if it does not exist
func EnsureDeploymentConfigNode(g osgraph.MutableUniqueGraph, dc *deployapi.DeploymentConfig) *DeploymentConfigNode {
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

func FindOrCreateSyntheticDeploymentConfigNode(g osgraph.MutableUniqueGraph, dc *deployapi.DeploymentConfig) *DeploymentConfigNode {
	return osgraph.EnsureUnique(
		g,
		DeploymentConfigNodeName(dc),
		func(node osgraph.Node) graph.Node {
			return &DeploymentConfigNode{Node: node, DeploymentConfig: dc, IsFound: false}
		},
	).(*DeploymentConfigNode)
}
