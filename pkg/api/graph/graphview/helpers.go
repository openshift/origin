package graphview

import (
	"github.com/openshift/origin/pkg/api/kubegraph/analysis"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// MaxRecentContainerRestartsForPods returns the maximum container restarts for given
// pods.
func MaxRecentContainerRestartsForPods(pods []*kubegraph.PodNode) int32 {
	var maxRestarts int32
	for _, pod := range pods {
		for _, status := range pod.Status.ContainerStatuses {
			if status.RestartCount > maxRestarts && analysis.ContainerRestartedRecently(status, unversioned.Now()) {
				maxRestarts = status.RestartCount
			}
		}
	}
	return maxRestarts
}
