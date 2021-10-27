package monitorapi

import (
	"strings"
	"time"
)

// BackendDisruptionSeconds return duration disrupted, disruptionMessages, and New or Reused
func BackendDisruptionSeconds(locator string, events Intervals) (time.Duration, []string, string) {
	disruptionEvents := events.Filter(
		func(i EventInterval) bool {
			if i.Locator != locator {
				return false
			}
			if !strings.Contains(i.Message, "stopped responding") {
				return false
			}
			return true
		},
	)
	disruptionMessages := disruptionEvents.Strings()
	connectionType := "Unknown"
	for _, disruptionMessage := range disruptionMessages {
		switch {
		case strings.Contains(disruptionMessage, "over reusedconnections"):
			connectionType = "Reused"
		case strings.Contains(disruptionMessage, "over new connections"):
			connectionType = "New"
		default:
			connectionType = "Unknown"
		}
	}
	return disruptionEvents.Duration(0, 1*time.Second), disruptionMessages, connectionType
}
