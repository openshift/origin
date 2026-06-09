package disruptionfilter

import (
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// FilterOutKnownDisruptiveTestIntervals removes disruption intervals that overlap with
// known-disruptive serial tests like NoExecuteTaintManager, which applies NoExecute taints
// to worker nodes where its test pods land, evicting pods and causing expected unavailability.
func FilterOutKnownDisruptiveTestIntervals(intervals monitorapi.Intervals) monitorapi.Intervals {
	knownDisruptiveTests := intervals.Filter(func(i monitorapi.Interval) bool {
		if i.Source != monitorapi.SourceE2ETest {
			return false
		}
		testName := i.Locator.Keys[monitorapi.LocatorE2ETestKey]
		return strings.Contains(testName, "NoExecuteTaintManager")
	})

	if len(knownDisruptiveTests) == 0 {
		return intervals
	}

	return intervals.Filter(func(i monitorapi.Interval) bool {
		for _, disruptiveTest := range knownDisruptiveTests {
			if intervalsOverlap(i, disruptiveTest) && monitorapi.IsErrorEvent(i) {
				return false
			}
		}
		return true
	})
}

func intervalsOverlap(interval1, interval2 monitorapi.Interval) bool {
	end1 := interval1.To
	if end1.IsZero() {
		end1 = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	}

	end2 := interval2.To
	if end2.IsZero() {
		end2 = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	}

	return (interval1.From.Before(end2)) && (interval2.From.Before(end1))
}
