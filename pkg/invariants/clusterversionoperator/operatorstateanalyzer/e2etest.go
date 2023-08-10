package operatorstateanalyzer

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// E2ETestEventIntervals returns only Intervals for e2e tests
func E2ETestEventIntervals(events monitorapi.Intervals) monitorapi.Intervals {
	e2eEventIntervals := monitorapi.Intervals{}
	for i := range events {
		event := events[i]
		if event.From == event.To {
			continue
		}
		if !monitorapi.IsE2ETest(event.StructuredLocator) {
			continue
		}
		e2eEventIntervals = append(e2eEventIntervals, event)
	}
	return e2eEventIntervals
}

// FindOverlap finds intervals that overlap with the time between start and end.
func FindOverlap(intervals monitorapi.Intervals, start, end time.Time) monitorapi.Intervals {
	overlappingIntervals := monitorapi.Intervals{}
	for i := range intervals {
		interval := intervals[i]
		switch {
		case interval.From.After(start) && interval.From.Before(end):
			// if the interval started during the window, we overlapped
			overlappingIntervals = append(overlappingIntervals, interval)
		case interval.To.After(start) && interval.To.Before(end):
			// if the interval ended during the window, we overlapped
			overlappingIntervals = append(overlappingIntervals, interval)

		case interval.From.Before(start) && interval.To.After(end):
			// if the interval started before the window and ended after the window, we overlapped
			overlappingIntervals = append(overlappingIntervals, interval)

		default:
			// the other two cases are starting and ending before the window (no overlap) and
			// starting and ending after the window (no overlap)
		}
	}

	return overlappingIntervals
}
