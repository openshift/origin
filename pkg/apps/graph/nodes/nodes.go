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

// EnsureDeploymentNode adds the provided upstream deployment to the graph if it does not exist
func EnsureDeploymentNode(g osgraph.MutableUniqueGraph, deployment *kapisext.Deployment) *DeploymentNode {
	deploymentName := DeploymentNodeName(deployment)
	deploymentNode := osgraph.EnsureUnique(
		g,
		deploymentName,
		func(node osgraph.Node) graph.Node {
			return &DeploymentNode{Node: node, Deployment: deployment, IsFound: true}
		},
	).(*DeploymentNode)

	podTemplateSpecNode := kubegraph.EnsurePodTemplateSpecNode(g, &deployment.Spec.Template, deployment.Namespace, deploymentName)
	g.AddEdge(deploymentNode, podTemplateSpecNode, osgraph.ContainsEdgeKind)

	return deploymentNode
}

func FindOrCreateSyntheticDeploymentNode(g osgraph.MutableUniqueGraph, deployment *kapisext.Deployment) *DeploymentNode {
	return osgraph.EnsureUnique(
		g,
		DeploymentNodeName(deployment),
		func(node osgraph.Node) graph.Node {
			return &DeploymentNode{Node: node, Deployment: deployment, IsFound: false}
		},
	).(*DeploymentNode)
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

// EnsureReplicaSetNode adds the provided replica set to the graph if it does not exist
func EnsureReplicaSetNode(g osgraph.MutableUniqueGraph, rs *kapisext.ReplicaSet) *ReplicaSetNode {
	rsName := ReplicaSetNodeName(rs)
	rsNode := osgraph.EnsureUnique(
		g,
		rsName,
		func(node osgraph.Node) graph.Node {
			return &ReplicaSetNode{Node: node, ReplicaSet: rs, IsFound: true}
		},
	).(*ReplicaSetNode)

	podTemplateSpecNode := kubegraph.EnsurePodTemplateSpecNode(g, &rs.Spec.Template, rs.Namespace, rsName)
	g.AddEdge(rsNode, podTemplateSpecNode, osgraph.ContainsEdgeKind)

	return rsNode
}

func FindOrCreateSyntheticReplicaSetNode(g osgraph.MutableUniqueGraph, rs *kapisext.ReplicaSet) *ReplicaSetNode {
	return osgraph.EnsureUnique(
		g,
		ReplicaSetNodeName(rs),
		func(node osgraph.Node) graph.Node {
			return &ReplicaSetNode{Node: node, ReplicaSet: rs, IsFound: false}
		},
	).(*ReplicaSetNode)
}
