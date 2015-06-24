package nodes

import (
	"github.com/gonum/graph"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
)

func EnsurePodNode(g osgraph.MutableUniqueGraph, pod *kapi.Pod) *PodNode {
	podNodeName := PodNodeName(pod)
	podNode := osgraph.EnsureUnique(g,
		podNodeName,
		func(node osgraph.Node) graph.Node {
			return &PodNode{node, pod}
		},
	).(*PodNode)

	podSpecNode := EnsurePodSpecNode(g, &pod.Spec, podNodeName)
	g.AddEdge(podNode, podSpecNode, osgraph.ContainsEdgeKind)

	return podNode
}

func EnsurePodSpecNode(g osgraph.MutableUniqueGraph, podSpec *kapi.PodSpec, ownerName osgraph.UniqueName) *PodSpecNode {
	return osgraph.EnsureUnique(g,
		PodSpecNodeName(podSpec, ownerName),
		func(node osgraph.Node) graph.Node {
			return &PodSpecNode{node, podSpec, ownerName}
		},
	).(*PodSpecNode)
}

// EnsureServiceNode adds the provided service to the graph if it does not already exist.
func EnsureServiceNode(g osgraph.MutableUniqueGraph, svc *kapi.Service) *ServiceNode {
	return osgraph.EnsureUnique(g,
		ServiceNodeName(svc),
		func(node osgraph.Node) graph.Node {
			return &ServiceNode{node, svc}
		},
	).(*ServiceNode)
}

func EnsureServiceAccountNode(g osgraph.MutableUniqueGraph, o *kapi.ServiceAccount) *ServiceAccountNode {
	return osgraph.EnsureUnique(g,
		ServiceAccountNodeName(o),
		func(node osgraph.Node) graph.Node {
			return &ServiceAccountNode{node, o, true}
		},
	).(*ServiceAccountNode)
}

func FindOrCreateSyntheticServiceAccountNode(g osgraph.MutableUniqueGraph, o *kapi.ServiceAccount) *ServiceAccountNode {
	return osgraph.EnsureUnique(g,
		ServiceAccountNodeName(o),
		func(node osgraph.Node) graph.Node {
			return &ServiceAccountNode{node, o, false}
		},
	).(*ServiceAccountNode)
}

func EnsureSecretNode(g osgraph.MutableUniqueGraph, o *kapi.Secret) *SecretNode {
	return osgraph.EnsureUnique(g,
		SecretNodeName(o),
		func(node osgraph.Node) graph.Node {
			return &SecretNode{node, o, true}
		},
	).(*SecretNode)
}

func FindOrCreateSyntheticSecretNode(g osgraph.MutableUniqueGraph, o *kapi.Secret) *SecretNode {
	return osgraph.EnsureUnique(g,
		SecretNodeName(o),
		func(node osgraph.Node) graph.Node {
			return &SecretNode{node, o, false}
		},
	).(*SecretNode)
}

// EnsureReplicationControllerNode adds a graph node for the ReplicationController if it does not already exist.
func EnsureReplicationControllerNode(g osgraph.MutableUniqueGraph, rc *kapi.ReplicationController) *ReplicationControllerNode {
	rcNodeName := ReplicationControllerNodeName(rc)
	rcNode := osgraph.EnsureUnique(g,
		rcNodeName,
		func(node osgraph.Node) graph.Node {
			return &ReplicationControllerNode{node, rc}
		},
	).(*ReplicationControllerNode)

	rcSpecNode := EnsureReplicationControllerSpecNode(g, &rc.Spec, rcNodeName)
	g.AddEdge(rcNode, rcSpecNode, osgraph.ContainsEdgeKind)

	return rcNode
}

func EnsureReplicationControllerSpecNode(g osgraph.MutableUniqueGraph, rcSpec *kapi.ReplicationControllerSpec, ownerName osgraph.UniqueName) *ReplicationControllerSpecNode {
	rcSpecName := ReplicationControllerSpecNodeName(rcSpec, ownerName)
	rcSpecNode := osgraph.EnsureUnique(g,
		rcSpecName,
		func(node osgraph.Node) graph.Node {
			return &ReplicationControllerSpecNode{node, rcSpec, ownerName}
		},
	).(*ReplicationControllerSpecNode)

	if rcSpec.Template != nil {
		ptSpecNode := EnsurePodTemplateSpecNode(g, rcSpec.Template, rcSpecName)
		g.AddEdge(rcSpecNode, ptSpecNode, osgraph.ContainsEdgeKind)
	}

	return rcSpecNode
}

func EnsurePodTemplateSpecNode(g osgraph.MutableUniqueGraph, ptSpec *kapi.PodTemplateSpec, ownerName osgraph.UniqueName) *PodTemplateSpecNode {
	ptSpecName := PodTemplateSpecNodeName(ptSpec, ownerName)
	ptSpecNode := osgraph.EnsureUnique(g,
		ptSpecName,
		func(node osgraph.Node) graph.Node {
			return &PodTemplateSpecNode{node, ptSpec, ownerName}
		},
	).(*PodTemplateSpecNode)

	podSpecNode := EnsurePodSpecNode(g, &ptSpec.Spec, ptSpecName)
	g.AddEdge(ptSpecNode, podSpecNode, osgraph.ContainsEdgeKind)

	return ptSpecNode
}
