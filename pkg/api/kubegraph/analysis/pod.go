package analysis

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

const (
	RestartingPodWarning = "RestartingPod"

	RestartThreshold = 3
)

// FindRestartingPods inspects all Pods to see if they've restarted more than the threshold
func FindRestartingPods(g osgraph.Graph) []osgraph.Marker {
	markers := []osgraph.Marker{}

	for _, uncastPodNode := range g.NodesByKind(kubegraph.PodNodeKind) {
		podNode := uncastPodNode.(*kubegraph.PodNode)
		pod, ok := podNode.Object().(*kapi.Pod)
		if !ok {
			continue
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.RestartCount >= RestartThreshold {
				markers = append(markers, osgraph.Marker{
					Node: podNode,

					Severity: osgraph.WarningSeverity,
					Key:      RestartingPodWarning,
					Message: fmt.Sprintf("container %q in %s has restarted %d times", containerStatus.Name,
						podNode.ResourceString(), containerStatus.RestartCount),
				})
			}
		}
	}

	return markers
}
