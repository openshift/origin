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

	ret := monitorapi.Intervals{}
	ret = append(ret,
		buildTransitionsForCategory(podToStateTransitions,
			monitorapi.PodReasonCreated, monitorapi.PodReasonDeleted, startTime, endTime)...,
	)
	ret = append(ret,
		buildTransitionsForCategory(podToContainerToLifecycleTransitions,
			monitorapi.ContainerReasonContainerWait, monitorapi.ContainerReasonContainerExit, startTime, endTime)...,
	)
	ret = append(ret,
		buildTransitionsForCategory(podToContainerToReadinessTransitions,
			monitorapi.ContainerReasonNotReady, monitorapi.ContainerReasonReady, startTime, endTime)...,
	)

	sort.Stable(ret)
	return ret
}

func buildTransitionsForCategory(locatorToConditions map[string][]monitorapi.EventInterval, startReason, endReason string, startTime, endTime time.Time) monitorapi.Intervals {
	ret := monitorapi.Intervals{}
	// now step through each category and build the to/from interval
	for locator, instantEvents := range locatorToConditions {
		prevEvent := emptyEvent(startTime)
		for i := range instantEvents {
			hasPrev := len(prevEvent.Message) > 0
			currEvent := instantEvents[i]
			currReason := monitorapi.ReasonFrom(currEvent.Message)

			nextInterval := monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: locator,
					Message: prevEvent.Message,
				},
				From: prevEvent.From,
				To:   currEvent.From,
			}

			switch {
			case !hasPrev && currReason == startReason:
				// this is a default case, nothing to do

			case !hasPrev && currReason != startReason:
				// we missed the startReason (it probably happened before the watch was established)
				nextInterval.Message = monitorapi.ReasonedMessage(startReason, fmt.Sprintf("missed real %s", startReason))
			}

			// if the current reason is deleted, reset to an empty previous
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
					Message: prevEvent.Message,
				},
				From: prevEvent.From,
				To:   endTime,
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
