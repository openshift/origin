package conditions

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/kubernetes/pkg/kubectl"
)

// ErrContainerTerminated is returned by PodContainerRunning in the intermediate
// state where the pod indicates it's still running, but its container is already terminated
var ErrContainerTerminated = fmt.Errorf("container terminated")

// PodContainerRunning returns false until the named container has ContainerStatus running (at least once),
// and will return an error if the pod is deleted, runs to completion, or the container pod is not available.
func PodContainerRunning(containerName string) watchtools.ConditionFunc {
	return func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Deleted:
			return false, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
		}
		switch t := event.Object.(type) {
		case *corev1.Pod:
			switch t.Status.Phase {
			case corev1.PodRunning, corev1.PodPending:
			case corev1.PodFailed, corev1.PodSucceeded:
				return false, kubectl.ErrPodCompleted
			default:
				return false, nil
			}
			for _, s := range t.Status.ContainerStatuses {
				if s.Name != containerName {
					continue
				}
				if s.State.Terminated != nil {
					return false, ErrContainerTerminated
				}
				return s.State.Running != nil, nil
			}
			for _, s := range t.Status.InitContainerStatuses {
				if s.Name != containerName {
					continue
				}
				if s.State.Terminated != nil {
					return false, ErrContainerTerminated
				}
				return s.State.Running != nil, nil
			}
			return false, nil
		}
		return false, nil
	}
}
