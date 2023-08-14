package watchpods

import (
	"sort"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/statetracker"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func intervalsFromEvents_PodChanges(events monitorapi.Intervals, beginning, end time.Time) monitorapi.Intervals {
	var intervals monitorapi.Intervals
	podStateTracker := statetracker.NewStateTracker(monitorapi.ConstructionOwnerPodLifecycle, beginning)
	locatorToMessageAnnotations := map[string]map[string]string{}

	for _, event := range events {
		pod := monitorapi.PodFrom(event.Locator)
		if len(pod.Name) == 0 {
			continue
		}
		reason := monitorapi.ReasonFrom(event.Message)
		switch reason {
		case monitorapi.PodPendingReason, monitorapi.PodNotPendingReason, monitorapi.PodReasonDeleted:
		default:
			continue
		}

		podLocator := pod.ToLocator()
		podPendingState := statetracker.State("Pending", "PodWasPending")

		switch reason {
		case monitorapi.PodPendingReason:
			podStateTracker.OpenInterval(podLocator, podPendingState, event.From)
		case monitorapi.PodNotPendingReason:
			intervals = append(intervals, podStateTracker.CloseIfOpenedInterval(podLocator, podPendingState, pendingPodCondition, event.From)...)
		case monitorapi.PodReasonDeleted:
			intervals = append(intervals, podStateTracker.CloseIfOpenedInterval(podLocator, podPendingState, pendingPodCondition, event.From)...)
		}
	}
	intervals = append(intervals, podStateTracker.CloseAllIntervals(locatorToMessageAnnotations, end)...)

	return intervals
}

func createPodIntervalsFromInstants(input monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, startTime, endTime time.Time) monitorapi.Intervals {
	sort.Stable(ByPodLifecycle(input))
	// these *static* locators to events. These are NOT the same as the actual event locators because nodes are not consistently assigned.
	podToStateTransitions := map[string][]monitorapi.Interval{}
	allPodTransitions := map[string][]monitorapi.Interval{}
	containerToLifecycleTransitions := map[string][]monitorapi.Interval{}
	containerToReadinessTransitions := map[string][]monitorapi.Interval{}
	containerToKubeletReadinessChecks := map[string][]monitorapi.Interval{}

	for i := range input {
		event := input[i]
		pod := monitorapi.PodFrom(event.Locator)
		if len(pod.Name) == 0 {
			continue
		}
		allPodTransitions[pod.ToLocator()] = append(allPodTransitions[pod.ToLocator()], event)
		isRecognizedPodReason := monitorapi.PodLifecycleTransitionReasons.Has(monitorapi.ReasonFrom(event.Message))

		container := monitorapi.ContainerFrom(event.Locator)
		isContainer := len(container.ContainerName) > 0
		isContainerLifecycleTransition := monitorapi.ContainerLifecycleTransitionReasons.Has(monitorapi.ReasonFrom(event.Message))
		isContainerReadyTransition := monitorapi.ContainerReadinessTransitionReasons.Has(monitorapi.ReasonFrom(event.Message))
		isKubeletReadinessCheck := monitorapi.KubeletReadinessCheckReasons.Has(monitorapi.ReasonFrom(event.Message))

		switch {
		case !isContainer && isRecognizedPodReason:
			podToStateTransitions[pod.ToLocator()] = append(podToStateTransitions[pod.ToLocator()], event)

		case isContainer && isContainerLifecycleTransition:
			containerToLifecycleTransitions[container.ToLocator()] = append(containerToLifecycleTransitions[container.ToLocator()], event)

		case isContainer && isContainerReadyTransition:
			containerToReadinessTransitions[container.ToLocator()] = append(containerToReadinessTransitions[container.ToLocator()], event)

		case isKubeletReadinessCheck:
			containerToKubeletReadinessChecks[container.ToLocator()] = append(containerToKubeletReadinessChecks[container.ToLocator()], event)

		}
	}

	overallTimeBounder := newSimpleTimeBounder(startTime, endTime)
	podTimeBounder := podLifecycleTimeBounder{
		delegate:              overallTimeBounder,
		podToStateTransitions: podToStateTransitions,
		allPodTransitions:     allPodTransitions,
		recordedPods:          recordedResources["pods"],
	}
	containerTimeBounder := containerLifecycleTimeBounder{
		delegate:                             podTimeBounder,
		podToContainerToLifecycleTransitions: containerToLifecycleTransitions,
		recordedPods:                         recordedResources["pods"],
	}
	containerReadinessTimeBounder := containerReadinessTimeBounder{
		delegate:                             containerTimeBounder,
		podToContainerToLifecycleTransitions: containerToLifecycleTransitions,
	}

	ret := monitorapi.Intervals{}
	ret = append(ret,
		buildTransitionsForCategory(podToStateTransitions,
			monitorapi.PodReasonCreated, monitorapi.PodReasonDeleted, podTimeBounder)...,
	)
	ret = append(ret,
		buildTransitionsForCategory(containerToLifecycleTransitions,
			monitorapi.ContainerReasonContainerWait, monitorapi.ContainerReasonContainerExit, containerTimeBounder)...,
	)
	ret = append(ret,
		buildTransitionsForCategory(containerToReadinessTransitions,
			monitorapi.ContainerReasonNotReady, "", containerReadinessTimeBounder)...,
	)

	// inject readiness failures.  These are done separately because they don't impact the overall ready or not ready
	// recall that a container can fail multiple readiness checks before the failure causes readyz=false on the pod overall.
	// to do this, we find all the readiness failures, make them one second long, so they appear.
	// we have to render them as a separate bar because we don't want to force the timeline for readiness to be
	// broken up and the timeline rendering logic we have
	for locator, instantEvents := range containerToKubeletReadinessChecks {
		for _, instantEvent := range instantEvents {
			ret = append(ret, monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: locator,
					Message: monitorapi.NewMessage().Constructed(monitorapi.ConstructionOwnerPodLifecycle).
						HumanMessage(instantEvent.Message).BuildString(),
				},
				From: instantEvent.From,
				To:   instantEvent.From.Add(1 * time.Second),
			})
		}
	}

	sort.Stable(ret)
	return ret
}

func pendingPodCondition(locator string, from, to time.Time) (monitorapi.Condition, bool) {
	if to.Sub(from) < 1*time.Minute {
		return monitorapi.Condition{}, false
	}
	return monitorapi.Condition{
		Level:   monitorapi.Warning,
		Locator: locator,
		Message: "pod has been pending longer than a minute",
	}, true
}

func newSimpleTimeBounder(startTime, endTime time.Time) timeBounder {
	return simpleTimeBounder{
		startTime: startTime,
		endTime:   endTime,
	}
}

type simpleTimeBounder struct {
	startTime time.Time
	endTime   time.Time
}

func (t simpleTimeBounder) getStartTime(locator string) time.Time {
	return t.startTime
}
func (t simpleTimeBounder) getEndTime(locator string) time.Time {
	return t.endTime
}

type podLifecycleTimeBounder struct {
	delegate              timeBounder
	podToStateTransitions map[string][]monitorapi.Interval
	allPodTransitions     map[string][]monitorapi.Interval
	recordedPods          monitorapi.InstanceMap
}

func (t podLifecycleTimeBounder) getStartTime(inLocator string) time.Time {
	podCreationTime := t.getPodCreationTime(inLocator)

	// use the earliest known event as a creation time, since it clearly existed at that point in time.
	var podCreateEventTime *time.Time
	locator := monitorapi.PodFrom(inLocator).ToLocator()
	podEvents := t.allPodTransitions[locator]
	for _, event := range podEvents {
		podCreateEventTime = &event.From
		break
	}

	switch {
	case podCreationTime == nil && podCreateEventTime == nil:
		return t.delegate.getStartTime(locator)

	case podCreationTime != nil && podCreateEventTime == nil:
		return *podCreationTime

	case podCreationTime == nil && podCreateEventTime != nil:
		return *podCreateEventTime

	case podCreationTime != nil && podCreateEventTime != nil:
		if podCreationTime.Before(*podCreateEventTime) {
			return *podCreationTime
		} else {
			return *podCreateEventTime
		}
	}

	return t.delegate.getStartTime(locator)
}

func (t podLifecycleTimeBounder) getEndTime(inLocator string) time.Time {
	podCoordinates := monitorapi.PodFrom(inLocator)
	locator := podCoordinates.ToLocator()

	// if this is a RunOnce pod that has finished running all of its containers, then the intervals chart will show that
	// pod no longer existed after the last container terminated.
	// We check this first so that actual pod deletion will not override this better time.
	if runOnceContainerTermination := t.getRunOnceContainerEnd(inLocator); runOnceContainerTermination != nil {
		return *runOnceContainerTermination
	}

	// pods will logically be gone once the pod deletion + grace period is over. Or at least they should be
	lastPossiblePodDelete := t.getPodDeletionPlusGraceTime(inLocator)

	podEvents, ok := t.podToStateTransitions[locator]
	if !ok {
		return t.delegate.getEndTime(locator)
	}
	for _, event := range podEvents {
		if monitorapi.ReasonFrom(event.Message) == monitorapi.PodReasonDeleted {
			// if the last possible pod delete is before the delete from teh watch stream, it just means our watch was delayed.
			// use the pod time instead.
			if lastPossiblePodDelete != nil && lastPossiblePodDelete.Before(event.From) {
				return *lastPossiblePodDelete
			}
			return event.From
		}
	}

	return t.delegate.getEndTime(locator)
}

func (t podLifecycleTimeBounder) getPodCreationTime(inLocator string) *time.Time {
	podCoordinates := monitorapi.PodFrom(inLocator)
	instanceKey := monitorapi.InstanceKey{
		Namespace: podCoordinates.Namespace,
		Name:      podCoordinates.Name,
		UID:       podCoordinates.UID,
	}

	// no hit for deleted, but if it's a RunOnce pod with all terminated containers, the logical "this pod is over"
	// happens when the last container is terminated.
	recordedPodObj, ok := t.recordedPods[instanceKey]
	if !ok {
		return nil
	}
	pod, ok := recordedPodObj.(*corev1.Pod)
	if !ok {
		return nil
	}
	if pod.CreationTimestamp.Time.IsZero() {
		return nil
	}

	// static pods can have a creation time that is actually after their first observed time.  In a weird quirk of the API,
	// it's possible to see the first appearance using annotations[kubernetes.io/config.seen].  This may be coincidence,
	// but it's handy for now to make a slightly more useful graph
	if staticPodSeen, ok := pod.Annotations["kubernetes.io/config.seen"]; ok {
		staticPodSeenTime, err := time.Parse(time.RFC3339Nano, staticPodSeen)
		if err != nil {
			panic(err)
		}
		return &staticPodSeenTime
	}

	temp := pod.CreationTimestamp
	return &temp.Time
}

func (t podLifecycleTimeBounder) getPodDeletionPlusGraceTime(inLocator string) *time.Time {
	podCoordinates := monitorapi.PodFrom(inLocator)
	instanceKey := monitorapi.InstanceKey{
		Namespace: podCoordinates.Namespace,
		Name:      podCoordinates.Name,
		UID:       podCoordinates.UID,
	}

	// no hit for deleted, but if it's a RunOnce pod with all terminated containers, the logical "this pod is over"
	// happens when the last container is terminated.
	recordedPodObj, ok := t.recordedPods[instanceKey]
	if !ok {
		return nil
	}
	pod, ok := recordedPodObj.(*corev1.Pod)
	if !ok {
		return nil
	}
	if pod.DeletionTimestamp == nil {
		return nil
	}

	deletionTime := pod.DeletionTimestamp.Time
	if pod.Spec.TerminationGracePeriodSeconds != nil {
		deletionTime = deletionTime.Add(time.Duration(*pod.Spec.TerminationGracePeriodSeconds * int64(time.Second)))
	}
	return &deletionTime
}

func (t podLifecycleTimeBounder) getRunOnceContainerEnd(inLocator string) *time.Time {
	podCoordinates := monitorapi.PodFrom(inLocator)
	instanceKey := monitorapi.InstanceKey{
		Namespace: podCoordinates.Namespace,
		Name:      podCoordinates.Name,
		UID:       podCoordinates.UID,
	}

	// no hit for deleted, but if it's a RunOnce pod with all terminated containers, the logical "this pod is over"
	// happens when the last container is terminated.
	recordedPodObj, ok := t.recordedPods[instanceKey]
	if !ok {
		return nil
	}
	pod, ok := recordedPodObj.(*corev1.Pod)
	if !ok {
		return nil
	}
	if pod.Spec.RestartPolicy != corev1.RestartPolicyNever {
		return nil
	}
	if len(pod.Status.ContainerStatuses) == 0 {
		return nil
	}
	mostRecentTerminationTime := metav1.Time{}
	for _, containerStatus := range pod.Status.ContainerStatuses {
		// if any container is not terminated, then this pod is logically still present
		if containerStatus.State.Terminated == nil {
			return nil
		}
		if mostRecentTerminationTime.Before(&containerStatus.State.Terminated.FinishedAt) {
			mostRecentTerminationTime = containerStatus.State.Terminated.FinishedAt
		}
	}

	// if a RunConce pod has finished running all of its containers, then the intervals chart will show that
	// pod no longer existed after the last container terminated.
	return &mostRecentTerminationTime.Time
}

type containerLifecycleTimeBounder struct {
	delegate                             timeBounder
	podToContainerToLifecycleTransitions map[string][]monitorapi.Interval
	recordedPods                         monitorapi.InstanceMap
}

func (t containerLifecycleTimeBounder) getStartTime(inLocator string) time.Time {
	locator := monitorapi.ContainerFrom(inLocator).ToLocator()
	containerEvents, ok := t.podToContainerToLifecycleTransitions[locator]
	if !ok {
		return t.delegate.getStartTime(locator)
	}
	for _, event := range containerEvents {
		if monitorapi.ReasonFrom(event.Message) == monitorapi.ContainerReasonContainerWait {
			return event.From
		}
	}

	// no hit, try to bound based on pod
	return t.delegate.getStartTime(locator)
}

func (t containerLifecycleTimeBounder) getEndTime(inLocator string) time.Time {
	// if this is a a terminated container that isn't restarting, then its end time is when the container was terminated.
	if containerTermination := t.getContainerEnd(inLocator); containerTermination != nil {
		return *containerTermination
	}

	locator := monitorapi.ContainerFrom(inLocator).ToLocator()
	containerEvents, ok := t.podToContainerToLifecycleTransitions[locator]
	if !ok {
		return t.delegate.getEndTime(locator)
	}
	// if the last event is a containerExit, then that's as long as the container lasted.
	// if the last event isn't a containerExit, then the last time we're aware of for the container is parent.
	lastEvent := containerEvents[len(containerEvents)-1]
	if monitorapi.ReasonFrom(lastEvent.Message) == monitorapi.ContainerReasonContainerExit {
		return lastEvent.From
	}

	// no hit, try to bound based on pod
	return t.delegate.getEndTime(locator)
}

func (t containerLifecycleTimeBounder) getContainerEnd(inLocator string) *time.Time {
	containerCoordinates := monitorapi.ContainerFrom(inLocator)
	instanceKey := monitorapi.InstanceKey{
		Namespace: containerCoordinates.Pod.Namespace,
		Name:      containerCoordinates.Pod.Name,
		UID:       containerCoordinates.Pod.UID,
	}

	recordedPodObj, ok := t.recordedPods[instanceKey]
	if !ok {
		return nil
	}
	pod, ok := recordedPodObj.(*corev1.Pod)
	if !ok {
		return nil
	}
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name != containerCoordinates.ContainerName {
			continue
		}

		// if we're running, then we're still running
		if containerStatus.State.Running != nil {
			return nil
		}
		// if we're wait, then we're going to be running again
		if containerStatus.State.Waiting != nil {
			return nil
		}
		// if any container is not terminated, then we have no additional data
		if containerStatus.State.Terminated == nil {
			return nil
		}

		// if we get here, then the container is terminated and not in a state where it is actively restarting
		t := containerStatus.State.Terminated.FinishedAt
		return &t.Time
	}
	for _, containerStatus := range pod.Status.InitContainerStatuses {
		if containerStatus.Name != containerCoordinates.ContainerName {
			continue
		}

		// if we're running, then we're still running
		if containerStatus.State.Running != nil {
			return nil
		}
		// if we're wait, then we're going to be running again
		if containerStatus.State.Waiting != nil {
			return nil
		}
		// if any container is not terminated, then we have no additional data
		if containerStatus.State.Terminated == nil {
			return nil
		}

		// if we get here, then the container is terminated and not in a state where it is actively restarting
		t := containerStatus.State.Terminated.FinishedAt
		return &t.Time
	}

	return nil
}

type containerReadinessTimeBounder struct {
	delegate                             timeBounder
	podToContainerToLifecycleTransitions map[string][]monitorapi.Interval
}

func (t containerReadinessTimeBounder) getStartTime(inLocator string) time.Time {
	locator := monitorapi.ContainerFrom(inLocator).ToLocator()
	containerEvents, ok := t.podToContainerToLifecycleTransitions[locator]
	if !ok {
		return t.delegate.getStartTime(locator)
	}
	for _, event := range containerEvents {
		// you can only be ready from the time your container is started.
		if monitorapi.ReasonFrom(event.Message) == monitorapi.ContainerReasonContainerStart {
			return event.From
		}
	}

	// no hit, try to bound based on pod
	return t.delegate.getStartTime(locator)
}

func (t containerReadinessTimeBounder) getEndTime(inLocator string) time.Time {
	return t.delegate.getEndTime(inLocator)
}

// timeBounder takes a locator and returns the earliest time for an interval about that item and latest time for an interval about that item.
// this is useful when you might not have seen every event and need to compensate for missing the first create or missing the final delete
type timeBounder interface {
	getStartTime(locator string) time.Time
	getEndTime(locator string) time.Time
}

func buildTransitionsForCategory(locatorToConditions map[string][]monitorapi.Interval, startReason, endReason monitorapi.IntervalReason, timeBounder timeBounder) monitorapi.Intervals {
	ret := monitorapi.Intervals{}
	// now step through each category and build the to/from interval
	for locator, instantEvents := range locatorToConditions {
		startTime := timeBounder.getStartTime(locator)
		endTime := timeBounder.getEndTime(locator)
		prevEvent := emptyEvent(timeBounder.getStartTime(locator))
		for i := range instantEvents {
			hasPrev := len(prevEvent.Message) > 0
			currEvent := instantEvents[i]
			currReason := monitorapi.ReasonFrom(currEvent.Message)
			prevAnnotations := monitorapi.AnnotationsFromMessage(prevEvent.Message)
			prevBareMessage := monitorapi.NonAnnotationMessage(prevEvent.Message)

			nextInterval := monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: locator,
					Message: monitorapi.NewMessage().Constructed(monitorapi.ConstructionOwnerPodLifecycle).WithAnnotations(prevAnnotations).HumanMessage(prevBareMessage).BuildString(),
				},
				From: prevEvent.From,
				To:   currEvent.From,
			}
			nextInterval = sanitizeTime(nextInterval, startTime, endTime)

			switch {
			case !hasPrev && currReason == startReason:
				// if we had no data and then learned about a start, do not append anything, but track prev
				// we need to be sure we get the times from nextInterval because they are not all event times,
				// but we need the message from the currEvent
				prevEvent = nextInterval
				prevEvent.Message = currEvent.Message
				continue

			case !hasPrev && currReason != startReason:
				// we missed the startReason (it probably happened before the watch was established).
				// adjust the message to indicate that we missed the start event for this locator
				nextInterval.Message = monitorapi.NewMessage().Constructed(monitorapi.ConstructionOwnerPodLifecycle).Reason(startReason).HumanMessagef("missed real %q", startReason).BuildString()
			}

			// if the current reason is a logical ending point, reset to an empty previous
			if currReason == endReason {
				prevEvent = emptyEvent(currEvent.From)
			} else {
				prevEvent = currEvent
			}
			ret = append(ret, nextInterval)
		}
		if len(prevEvent.Message) > 0 {
			nextInterval := monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: locator,
					Message: monitorapi.ExpandMessage(prevEvent.Message).Constructed(monitorapi.ConstructionOwnerPodLifecycle).BuildString(),
				},
				From: prevEvent.From,
				To:   timeBounder.getEndTime(locator),
			}
			nextInterval = sanitizeTime(nextInterval, startTime, endTime)
			ret = append(ret, nextInterval)
		}
	}

	return ret
}

func sanitizeTime(nextInterval monitorapi.Interval, startTime, endTime time.Time) monitorapi.Interval {
	if !endTime.IsZero() && nextInterval.To.After(endTime) {
		nextInterval.To = endTime
	}
	if nextInterval.From.Before(startTime) {
		nextInterval.From = startTime
	}
	if !nextInterval.To.IsZero() && nextInterval.To.Before(nextInterval.From) {
		nextInterval.From = nextInterval.To
	}
	return nextInterval
}

func emptyEvent(startTime time.Time) monitorapi.Interval {
	return monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level: monitorapi.Info,
		},
		From: startTime,
	}
}

type ByPodLifecycle monitorapi.Intervals

func (n ByPodLifecycle) Len() int {
	return len(n)
}

func (n ByPodLifecycle) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n ByPodLifecycle) Less(i, j int) bool {
	switch d := n[i].From.Sub(n[j].From); {
	case d < 0:
		return true
	case d > 0:
		return false
	}
	lhsReason := monitorapi.ReasonFrom(n[i].Message)
	rhsReason := monitorapi.ReasonFrom(n[j].Message)

	switch {
	case lhsReason == monitorapi.PodReasonCreated && rhsReason == monitorapi.PodReasonScheduled:
		return true
	case lhsReason == monitorapi.PodReasonScheduled && rhsReason == monitorapi.PodReasonCreated:
		return false
	}

	switch d := n[i].To.Sub(n[j].To); {
	case d < 0:
		return true
	case d > 0:
		return false
	}
	return n[i].Message < n[j].Message
}
