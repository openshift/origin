package monitorapi

import (
	"time"
)

// BackendDisruptionSeconds return duration of disruption observed (rounded to nearest second),
// disruptionMessages, and New or Reused connection type.
func BackendDisruptionSeconds(locator string, events Intervals) (time.Duration, []string, string) {
	disruptionEvents := events.Filter(
		And(
			IsEventForLocator(locator),
			IsErrorEvent,
		),
	)
	disruptionMessages := disruptionEvents.Strings()
	connectionType := DisruptionConnectionTypeFrom(LocatorParts(locator))

	return disruptionEvents.Duration(1 * time.Second).Round(time.Second), disruptionMessages, connectionType
}

func IsDisruptionEvent(eventInterval EventInterval) bool {
	if disruptionBackend := DisruptionFrom(LocatorParts(eventInterval.Locator)); len(disruptionBackend) > 0 {
		return true
	}
	return false
}
