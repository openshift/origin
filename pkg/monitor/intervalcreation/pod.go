package intervalcreation

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func findEventsForPod(events []*monitorapi.Event, podName string) []*monitorapi.Event {
	podEvents := []*monitorapi.Event{}
	for i := range events {
		event := events[i]
		_, currPodName, ok := monitorapi.PodFromLocator(event.Locator)
		if !ok {
			continue
		}
		if currPodName == podName {
			podEvents = append(podEvents, event)
		}
	}
	return podEvents
}

func IntervalsFromEvents_NonGracefulTermination(events []*monitorapi.Event, beginning, end time.Time) monitorapi.EventIntervals {
	ret := monitorapi.EventIntervals{}

	var mostRecentPodStartBeforeObservation *monitorapi.Event
	for _, event := range events {
		if !strings.Contains(event.Message, "NonGracefulTermination") {
			continue
		}
		// if we have a non-graceful termination, then we know that the kube-server as unexpectedly down until a pod
		// with the same name has started and the container with the name kube-apiserver has started.
		// the message in the event looks about like: Previous pod kube-apiserver-ci-op-9pj4lsci-c2d91-qffns-master-0 started at 2021-04-05 15:43:23.459538334 +0000 UTC did not terminate gracefully

		// recall that we observe these when the kube-apiserver pod starts again
		// TODO pull the time from this message, not the event to narrow it further
		// TODO limit the window to time based on the pod being shutdown
		timeGracefulFailureObserved := event.At
		parts := strings.SplitN(event.Message, "Previous pod ", 2)
		if len(parts) < 2 {
			continue
		}
		nameParts := strings.Split(parts[1], " ")
		podName := nameParts[0]
		podEvents := findEventsForPod(events, podName)

		// now we find the pod start event immediately before the current event
		for i := range podEvents {
			podEvent := podEvents[i]
			if !strings.Contains(podEvent.Message, "container/kube-apiserver reason/Started") {
				continue
			}
			if podEvent.At.After(timeGracefulFailureObserved) {
				break
			}
			mostRecentPodStartBeforeObservation = podEvent
		}

		from := beginning
		message := event.Message
		if mostRecentPodStartBeforeObservation != nil {
			from = mostRecentPodStartBeforeObservation.At
		} else {
			message = "missing pod start for kube-apiserver\n" + event.Message
		}
		ret = append(ret, &monitorapi.EventInterval{
			Condition: &monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: event.Locator,
				Message: message,
			},
			From: from,
			To:   event.At,
		})
	}

	return ret
}

type containerKey struct {
	namespace     string
	podName       string
	containerName string
	locator       string
}

type containerStateChange struct {
	containerState     string
	lastTransitionTime time.Time
}

// In a normal e2e run, pods in openshift-* should never go unready.  This is not true for upgrades where pods are
// expected to go not ready whenver a node restarts and when they are first create.
func IntervalsFromEvents_OpenShiftPodNotReady(events []*monitorapi.Event, beginning, end time.Time) monitorapi.EventIntervals {
	ret := monitorapi.EventIntervals{}
	containerToInterestingBadState := map[containerKey]containerStateChange{}

	goodReadyState := `reason/Ready`
	badReadyState := `reason/NotReady`
	for _, event := range events {
		namespace, podName, containerName, ok := monitorapi.ContainerFromLocator(event.Locator)
		if !ok {
			continue
		}
		if !strings.HasPrefix(namespace, "openshift-") {
			continue
		}
		containerKey := containerKey{
			namespace:     namespace,
			podName:       podName,
			containerName: containerName,
			locator:       event.Locator,
		}

		currentState := event.Message
		lastState, hasLastState := containerToInterestingBadState[containerKey]
		if hasLastState && lastState.containerState == currentState {
			// if the status didn't actually change (imagine degraded just changing reasons)
			// don't count as the interval
			continue
		}
		if currentState != goodReadyState {
			// don't overwrite a previous condition in a bad state
			if !hasLastState {
				// force the last transition time, since we think we just transitioned at this instant
				containerToInterestingBadState[containerKey] = containerStateChange{
					containerState:     currentState,
					lastTransitionTime: event.At,
				}
			}
			continue
		}

		// at this point we have transitioned to a good state.  Remove the previous "bad" state
		delete(containerToInterestingBadState, containerKey)

		from := beginning
		lastStatus := "Unknown"
		if hasLastState {
			from = lastState.lastTransitionTime
			lastStatus = fmt.Sprintf("%v", lastState.containerState)
		} else {
			// if we're in a good state now, then we were probably in a bad state before.  Let's start by assuming that anyway
			lastStatus = badReadyState
		}
		ret = append(ret, &monitorapi.EventInterval{
			Condition: &monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: event.Locator,
				Message: lastStatus,
			},
			From: from,
			To:   event.At,
		})
	}

	for containerKey, lastCondition := range containerToInterestingBadState {
		ret = append(ret, &monitorapi.EventInterval{
			Condition: &monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: containerKey.locator,
				Message: lastCondition.containerState,
			},
			From: lastCondition.lastTransitionTime,
			To:   end,
		})
	}

	return ret
}
