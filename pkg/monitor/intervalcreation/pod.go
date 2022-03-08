package intervalcreation

import (
	"fmt"
	"sort"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func CreatePodIntervalsFromInstants(input monitorapi.Intervals, startTime, endTime time.Time) monitorapi.Intervals {
	sort.Stable(ByPodLifecycle(input))
	// these *static* locators to events. These are NOT the same as the actual event locators because nodes are not consistently assigned.
	podToStateTransitions := map[string][]monitorapi.EventInterval{}
	podToContainerToLifecycleTransitions := map[string][]monitorapi.EventInterval{}
	podToContainerToReadinessTransitions := map[string][]monitorapi.EventInterval{}

	for i := range input {
		event := input[i]
		pod := monitorapi.PodFrom(event.Locator)
		if len(pod.Name) == 0 {
			continue
		}
		isRecognizedPodReason := monitorapi.PodLifecycleTransitionReasons.Has(monitorapi.ReasonFrom(event.Message))

		container := monitorapi.ContainerFrom(event.Locator)
		isContainer := len(container.ContainerName) > 0
		isContainerLifecycleTransition := monitorapi.ContainerLifecycleTransitionReasons.Has(monitorapi.ReasonFrom(event.Message))
		isContainerReadyTransition := monitorapi.ContainerReadinessTransitionReasons.Has(monitorapi.ReasonFrom(event.Message))

		switch {
		case !isContainer && isRecognizedPodReason:
			podToStateTransitions[pod.ToLocator()] = append(podToStateTransitions[pod.ToLocator()], event)

		case isContainer && isContainerLifecycleTransition:
			podToContainerToLifecycleTransitions[container.ToLocator()] = append(podToContainerToLifecycleTransitions[container.ToLocator()], event)

		case isContainer && isContainerReadyTransition:
			podToContainerToReadinessTransitions[container.ToLocator()] = append(podToContainerToReadinessTransitions[container.ToLocator()], event)

		}
	}

	overallTimeBounder := newSimpleTimeBounder(startTime, endTime)
	podTimeBounder := podLifecycleTimeBounder{
		delegate:              overallTimeBounder,
		podToStateTransitions: podToStateTransitions,
	}
	containerTimeBounder := containerLifecycleTimeBounder{
		delegate:                             podTimeBounder,
		podToContainerToLifecycleTransitions: podToContainerToLifecycleTransitions,
	}
	containerReadinessTimeBounder := containerReadinessTimeBounder{
		delegate:                             containerTimeBounder,
		podToContainerToLifecycleTransitions: podToContainerToLifecycleTransitions,
	}

	ret := monitorapi.Intervals{}
	ret = append(ret,
		buildTransitionsForCategory(podToStateTransitions,
			monitorapi.PodReasonCreated, monitorapi.PodReasonDeleted, overallTimeBounder)...,
	)
	ret = append(ret,
		buildTransitionsForCategory(podToContainerToLifecycleTransitions,
			monitorapi.ContainerReasonContainerWait, monitorapi.ContainerReasonContainerExit, podTimeBounder)...,
	)
	ret = append(ret,
		buildTransitionsForCategory(podToContainerToReadinessTransitions,
			monitorapi.ContainerReasonNotReady, "", containerReadinessTimeBounder)...,
	)

	sort.Stable(ret)
	return ret
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
	podToStateTransitions map[string][]monitorapi.EventInterval
}

func (t podLifecycleTimeBounder) getStartTime(inLocator string) time.Time {
	locator := monitorapi.PodFrom(inLocator).ToLocator()
	podEvents, ok := t.podToStateTransitions[locator]
	if !ok {
		return t.delegate.getStartTime(locator)
	}
	for _, event := range podEvents {
		if monitorapi.ReasonFrom(event.Message) == monitorapi.PodReasonCreated {
			return event.From
		}
	}

	return t.delegate.getStartTime(locator)
}

func (t podLifecycleTimeBounder) getEndTime(inLocator string) time.Time {
	locator := monitorapi.PodFrom(inLocator).ToLocator()
	podEvents, ok := t.podToStateTransitions[locator]
	if !ok {
		return t.delegate.getEndTime(locator)
	}
	for _, event := range podEvents {
		if monitorapi.ReasonFrom(event.Message) == monitorapi.PodReasonDeleted {
			return event.From
		}
	}
	return t.delegate.getEndTime(locator)
}

type containerLifecycleTimeBounder struct {
	delegate                             timeBounder
	podToContainerToLifecycleTransitions map[string][]monitorapi.EventInterval
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
	locator := monitorapi.ContainerFrom(inLocator).ToLocator()
	containerEvents, ok := t.podToContainerToLifecycleTransitions[locator]
	if !ok {
		return t.delegate.getEndTime(locator)
	}
	for i := len(containerEvents) - 1; i >= 0; i-- {
		event := containerEvents[i]
		if monitorapi.ReasonFrom(event.Message) == monitorapi.ContainerReasonContainerExit {
			return event.From
		}
	}

	// no hit, try to bound based on pod
	return t.delegate.getEndTime(locator)
}

type containerReadinessTimeBounder struct {
	delegate                             timeBounder
	podToContainerToLifecycleTransitions map[string][]monitorapi.EventInterval
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

func buildTransitionsForCategory(locatorToConditions map[string][]monitorapi.EventInterval, startReason, endReason string, timeBounder timeBounder) monitorapi.Intervals {
	ret := monitorapi.Intervals{}
	// now step through each category and build the to/from interval
	for locator, instantEvents := range locatorToConditions {
		prevEvent := emptyEvent(timeBounder.getStartTime(locator))
		for i := range instantEvents {
			hasPrev := len(prevEvent.Message) > 0
			currEvent := instantEvents[i]
			currReason := monitorapi.ReasonFrom(currEvent.Message)

			nextInterval := monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: locator,
					Message: "constructed/true " + prevEvent.Message,
				},
				From: prevEvent.From,
				To:   currEvent.From,
			}

			switch {
			case !hasPrev && currReason == startReason:
				// if we had no data and then learned about a start, do not append anything, but track prev
				prevEvent = currEvent
				continue

			case !hasPrev && currReason != startReason:
				// we missed the startReason (it probably happened before the watch was established).
				// adjust the message to indicate that we missed the start event for this locator
				nextInterval.Message = "constructed/true " + monitorapi.ReasonedMessage(startReason, fmt.Sprintf("missed real %q", startReason))
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
			nextInterval := monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: locator,
					Message: "constructed/true " + prevEvent.Message,
				},
				From: prevEvent.From,
				To:   timeBounder.getEndTime(locator),
			}
			ret = append(ret, nextInterval)
		}
	}

	return ret
}

func emptyEvent(startTime time.Time) monitorapi.EventInterval {
	return monitorapi.EventInterval{
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
