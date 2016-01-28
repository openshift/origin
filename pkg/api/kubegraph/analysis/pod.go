package analysis

import (
	"fmt"
	"time"

	. "github.com/MakeNowJust/heredoc/dot"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

const (
	CrashLoopingPodError = "CrashLoopingPod"
	RestartingPodWarning = "RestartingPod"

	RestartThreshold      = 5
	RestartRecentDuration = 10 * time.Minute
)

// exposed for testing
var nowFn = unversioned.Now

// FindRestartingPods inspects all Pods to see if they've restarted more than the threshold
func FindRestartingPods(g osgraph.Graph, f osgraph.Namer) []osgraph.Marker {
	markers := []osgraph.Marker{}

	for _, uncastPodNode := range g.NodesByKind(kubegraph.PodNodeKind) {
		podNode := uncastPodNode.(*kubegraph.PodNode)
		pod, ok := podNode.Object().(*kapi.Pod)
		if !ok {
			continue
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			switch {
			case containerCrashLoopBackOff(containerStatus):
				var suggestion string
				switch {
				case containerIsNonRoot(pod, containerStatus.Name):
					suggestion = D(`
						The container is starting and exiting repeatedly, which usually means the container cannot
						start, is misconfigured, or is unable to perform an action due to security restrictions on
						the container. The container logs may contain messages indicating the reason the pod cannot
						start.

						This container is being run as as a non-root user due to administrative policy, and
						some images may fail expecting to be able to change ownership or set permissions on
						directories. Your administrator may need to grant permission for you to run root
						containers.`)
				default:
					suggestion = D(`
						The container is starting and exiting repeatedly, which usually means the container cannot
						start, is misconfigured, or is unable to perform an action due to security restrictions on
						the container. The container logs may contain messages indicating the reason the pod cannot
						start.`)
				}
				markers = append(markers, osgraph.Marker{
					Node: podNode,

					Severity: osgraph.ErrorSeverity,
					Key:      CrashLoopingPodError,
					Message: fmt.Sprintf("container %q in %s is crash-looping", containerStatus.Name,
						f.ResourceName(podNode)),
					Suggestion: osgraph.Suggestion(suggestion),
				})
			case containerRestartedRecently(containerStatus, nowFn()):
				markers = append(markers, osgraph.Marker{
					Node: podNode,

					Severity: osgraph.WarningSeverity,
					Key:      RestartingPodWarning,
					Message: fmt.Sprintf("container %q in %s has restarted within the last 10 minutes", containerStatus.Name,
						f.ResourceName(podNode)),
				})
			case containerRestartedFrequently(containerStatus):
				markers = append(markers, osgraph.Marker{
					Node: podNode,

					Severity: osgraph.WarningSeverity,
					Key:      RestartingPodWarning,
					Message: fmt.Sprintf("container %q in %s has restarted %d times", containerStatus.Name,
						f.ResourceName(podNode), containerStatus.RestartCount),
				})
			}
		}
	}

	return markers
}

func containerIsNonRoot(pod *kapi.Pod, container string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name != container || c.SecurityContext == nil {
			continue
		}
		switch {
		case c.SecurityContext.RunAsUser != nil && *c.SecurityContext.RunAsUser != 0:
			//c.SecurityContext.RunAsNonRoot != nil && *c.SecurityContext.RunAsNonRoot,
			return true
		}
	}
	return false
}

func containerCrashLoopBackOff(status kapi.ContainerStatus) bool {
	return status.State.Waiting != nil && status.State.Waiting.Reason == "CrashLoopBackOff"
}

func containerRestartedRecently(status kapi.ContainerStatus, now unversioned.Time) bool {
	if status.RestartCount == 0 {
		return false
	}
	if status.LastTerminationState.Terminated != nil && now.Sub(status.LastTerminationState.Terminated.FinishedAt.Time) < RestartRecentDuration {
		return true
	}
	return false
}

func containerRestartedFrequently(status kapi.ContainerStatus) bool {
	return status.RestartCount > RestartThreshold
}
