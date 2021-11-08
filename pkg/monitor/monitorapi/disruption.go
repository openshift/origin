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
			switch {
			case strings.Contains(i.Message, "stopped responding to"):
				return true
			case strings.Contains(i.Message, "is not responding to"):
				return true
			default:
				return false
			}
		},
	)
	disruptionMessages := disruptionEvents.Strings()
	connectionType := "Unknown"
	for _, disruptionMessage := range disruptionMessages {
		switch {
		case strings.Contains(disruptionMessage, "over reused connections"):
			connectionType = "Reused"
		case strings.Contains(disruptionMessage, "over new connections"):
			connectionType = "New"
		default:
			connectionType = "Unknown"
		}
	}
	return disruptionEvents.Duration(0, 1*time.Second), disruptionMessages, connectionType
}
