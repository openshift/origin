package kubegraph

import (
	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

// RelevantPods returns all the pods associated with the provided replication controller.
func RelevantPods(g osgraph.Graph, rc *kubegraph.ReplicationControllerNode) []*kubegraph.PodNode {
	pods := []*kubegraph.PodNode{}
	for _, uncastPodNode := range g.PredecessorNodesByEdgeKind(rc, ManagedByRCEdgeKind) {
		pods = append(pods, uncastPodNode.(*kubegraph.PodNode))
	}
	return pods
}
