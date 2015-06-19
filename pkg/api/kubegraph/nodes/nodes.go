package nodes

import (
	"github.com/gonum/graph"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
)

func EnsurePodNode(g osgraph.MutableUniqueGraph, pod *kapi.Pod) *PodNode {
	return osgraph.EnsureUnique(g,
		PodNodeName(pod),
		func(node osgraph.Node) graph.Node {
			return &PodNode{node, pod}
		},
	).(*PodNode)
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

// EnsureReplicationControllerNode adds a graph node for the ReplicationController if it does not already exist.
func EnsureReplicationControllerNode(g osgraph.MutableUniqueGraph, rc *kapi.ReplicationController) *ReplicationControllerNode {
	return osgraph.EnsureUnique(g,
		ReplicationControllerNodeName(rc),
		func(node osgraph.Node) graph.Node {
			return &ReplicationControllerNode{node, rc}
		},
	).(*ReplicationControllerNode)
}
