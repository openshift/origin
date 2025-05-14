package monitorapi

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func LocatePod(pod *corev1.Pod) Locator {
	return Locator{
		Type: LocatorTypePod,
		Keys: map[LocatorKey]string{
			LocatorNamespaceKey: pod.Namespace,
			LocatorPodKey:       pod.Name,
			LocatorNodeKey:      pod.Spec.NodeName,
			LocatorUIDKey:       string(pod.UID),
		},
	}
}

// NonUniquePodLocatorFrom produces an inexact locator based on namespace and name.  This is useful when dealing with events
// that are produced that do not contain UIDs.  Ultimately, we should use UIDs everywhere, but this is will keep some our
// matching working until then.
func NonUniquePodLocatorFrom(locator Locator) string {
	namespace := locator.Keys[LocatorNamespaceKey]
	pod := locator.Keys[LocatorPodKey]
	return fmt.Sprintf("ns/%s pod/%s", namespace, pod)
}

// PodFrom is used to strip down a locator to just a pod. (as it may contain additional keys like container or node)
// that we do not want for some uses.
func PodFrom(locator Locator) PodReference {
	namespace := locator.Keys[LocatorNamespaceKey]
	name := locator.Keys[LocatorPodKey]
	uid := locator.Keys[LocatorUIDKey]
	if len(namespace) == 0 || len(name) == 0 {
		return PodReference{}
	}
	return PodReference{
		NamespacedReference: NamespacedReference{
			Namespace: namespace,
			Name:      name,
			UID:       uid,
		},
	}
}

func ContainerFrom(locator Locator) ContainerReference {
	pod := PodFrom(locator)
	name := locator.Keys[LocatorContainerKey]
	if len(name) == 0 || len(pod.UID) == 0 {
		return ContainerReference{}
	}
	return ContainerReference{
		Pod:           pod,
		ContainerName: name,
	}
}

// TODO: delete all these
type PodReference struct {
	NamespacedReference
}

func (r PodReference) ToLocator() Locator {
	return NewLocator().PodFromNames(r.Namespace, r.Name, r.UID)
}

type ContainerReference struct {
	Pod           PodReference
	ContainerName string
}

func (r ContainerReference) ToLocator() Locator {
	return NewLocator().ContainerFromNames(r.Pod.Namespace, r.Pod.Name, r.Pod.UID, r.ContainerName)
}

func AnnotationsFromMessage(message string) map[AnnotationKey]string {
	tokens := strings.Split(message, " ")
	annotations := map[AnnotationKey]string{}
	for _, curr := range tokens {
		if !strings.Contains(curr, "/") {
			return annotations
		}
		annotationTokens := strings.Split(curr, "/")
		annotations[AnnotationKey(annotationTokens[0])] = annotationTokens[1]
	}
	return annotations
}

func NonAnnotationMessage(message string) string {
	tokens := strings.Split(message, " ")
	for i, curr := range tokens {
		if !strings.Contains(curr, "/") {
			return strings.Join(tokens[i:], " ")
		}
	}
	return ""
}

func ReasonFrom(message string) IntervalReason {
	annotations := AnnotationsFromMessage(message)
	return IntervalReason(annotations[AnnotationReason])
}

func ConstructionOwnerFrom(message string) IntervalReason {
	annotations := AnnotationsFromMessage(message)
	return IntervalReason(annotations[AnnotationConstructed])
}

func PhaseFrom(message string) string {
	annotations := AnnotationsFromMessage(message)
	return annotations[AnnotationPhase]
}

const (
	// PodIPReused means the same pod IP is in use by two pods at the same time.
	PodIPReused = "ReusedPodIP"

	ContainerErrImagePull                = "ErrImagePull"
	ContainerUnrecognizedSignatureFormat = "UnrecognizedSignatureFormat"
)

var (
	// PodLifecycleTransitionReasons are the reasons associated with non-overlapping pod lifecycle states.
	// A pod is logically identified by UID (I bet it's a name right now).
	// Pods don't exist before create and don't exist after delete.
	// Between those two states, each of these reasons can be ordered by time and used to create a contiguous view
	// into the lifecycle of a pod.
	PodLifecycleTransitionReasons = sets.New[IntervalReason](
		PodReasonCreated,
		PodReasonScheduled,
		PodReasonGracefulDeleteStarted,
		PodReasonDeleted,
	)

	// ContainerLifecycleTransitionReasons are the reasons associated with non-overlapping container lifecycle states.
	// The logical beginning and end are based on ContainerWait and ContainerExit.
	// A container is logically identified by a Pod plus a container name.
	ContainerLifecycleTransitionReasons = sets.New[IntervalReason](
		ContainerReasonContainerWait,
		ContainerReasonContainerStart,
		ContainerReasonContainerExit,
	)

	// ContainerReadinessTransitionReasons are the reasons associated with non-overlapping container readiness states.
	// A container is logically identified by a Pod plus a container name.
	// The logical beginning and end are based on ContainerStart and ContainerExit, with initial state of ready=false and final state of ready=false.
	// Each of these reasons can be ordered by time and used to create a contiguous view into the lifecycle of a pod.
	ContainerReadinessTransitionReasons = sets.New[IntervalReason](
		ContainerReasonReady,
		ContainerReasonNotReady,
	)

	KubeletReadinessCheckReasons = sets.New[IntervalReason](
		ContainerReasonReadinessFailed,
		ContainerReasonReadinessErrored,
		ContainerReasonStartupProbeFailed,
	)
)

type ByTimeWithNamespacedPods []Interval

func (intervals ByTimeWithNamespacedPods) Less(i, j int) bool {
	lhsIsPodConstructed := len(intervals[i].Message.Annotations[AnnotationConstructed]) > 0 && len(intervals[i].Locator.Keys[LocatorPodKey]) > 0
	rhsIsPodConstructed := len(intervals[j].Message.Annotations[AnnotationConstructed]) > 0 && len(intervals[j].Locator.Keys[LocatorPodKey]) > 0
	switch {
	case lhsIsPodConstructed && rhsIsPodConstructed:
		lhsNamespace := NamespaceFromLocator(intervals[i].Locator)
		rhsNamespace := NamespaceFromLocator(intervals[j].Locator)
		if lhsNamespace < rhsNamespace {
			return true
		} else if lhsNamespace > rhsNamespace {
			return false
		} else {
			// sort on time, so fall through.
		}
	case lhsIsPodConstructed && !rhsIsPodConstructed:
		return true
	case !lhsIsPodConstructed && rhsIsPodConstructed:
		return false
	case !lhsIsPodConstructed && !rhsIsPodConstructed:
		// fall through
	}

	switch d := intervals[i].From.Sub(intervals[j].From); {
	case d < 0:
		return true
	case d > 0:
		return false
	}
	switch d := intervals[i].To.Sub(intervals[j].To); {
	case d < 0:
		return true
	case d > 0:
		return false
	}
	return intervals[i].Message.HumanMessage < intervals[j].Message.HumanMessage
}

func (intervals ByTimeWithNamespacedPods) Len() int { return len(intervals) }

func (intervals ByTimeWithNamespacedPods) Swap(i, j int) {
	intervals[i], intervals[j] = intervals[j], intervals[i]
}
