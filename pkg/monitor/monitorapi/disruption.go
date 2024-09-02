package monitorapi

import (
	"time"
)

const EventDir = "monitor-events"

// BackendDisruptionSeconds return duration of disruption observed (rounded to nearest second),
// disruptionMessages, and New or Reused connection type.
func BackendDisruptionSeconds(backendDisruptionName string, events Intervals) (time.Duration, []string) {
	disruptionEvents := events.Filter(
		And(
			IsErrorEvent,
			IsEventForBackendDisruptionName(backendDisruptionName),
		),
	)
	disruptionMessages := disruptionEvents.Strings()

	return disruptionEvents.Duration(1 * time.Second).Round(time.Second), disruptionMessages
}

func IsDisruptionEvent(eventInterval Interval) bool {
	return eventInterval.Source == SourceDisruption
}

func HasRealLoadBalancer(eventInterval Interval) bool {
	return eventInterval.Locator.Keys[LocatorLoadBalancerKey] != "localhost"
}
