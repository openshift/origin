package monitor

import (
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// EventsByE2ETest returns map keyed by e2e test name (may contain spaces).
func EventsByE2ETest(events []*monitorapi.EventInterval) map[string][]*monitorapi.EventInterval {
	eventsByE2ETest := map[string][]*monitorapi.EventInterval{}
	for _, event := range events {
		if !strings.Contains(event.Locator, "e2e-test/") {
			continue
		}
		testName, ok := monitorapi.E2ETestFromLocator(event.Locator)
		if !ok {
			continue
		}
		eventsByE2ETest[testName] = append(eventsByE2ETest[testName], event)
	}
	return eventsByE2ETest
}

// E2ETestEvents returns only events for e2e tests
func E2ETestEvents(events []*monitorapi.EventInterval) []*monitorapi.EventInterval {
	e2eEvents := []*monitorapi.EventInterval{}
	for i := range events {
		event := events[i]
		if !strings.Contains(event.Locator, "e2e-test/") {
			continue
		}
		e2eEvents = append(e2eEvents, event)
	}
	return e2eEvents
}

// E2ETestsRunningBetween returns names of e2e test that were active during the timeframe
func E2ETestsRunningBetween(events []*monitorapi.EventInterval, start, end time.Time) map[string][]*monitorapi.EventInterval {
	activeTests := map[string][]*monitorapi.EventInterval{}
	e2eTestsToEvents := EventsByE2ETest(events)
	type activeInterval struct {
		start  time.Time
		end    time.Time
		events []*monitorapi.EventInterval
	}
	for testName, events := range e2eTestsToEvents {
		runningIntervals := []activeInterval{}
		currInterval := activeInterval{}
		for _, event := range events {
			switch {
			case event.Message == "started":
				currInterval.start = event.From
				currInterval.events = append(currInterval.events, event)
			case strings.HasPrefix(event.Message, "finishedStatus/"):
				currInterval.end = event.From
				currInterval.events = append(currInterval.events, event)
				runningIntervals = append(runningIntervals, currInterval)

				// reset
				currInterval = activeInterval{}
			}
		}

		// now check each interval (you can have more than one for flakes and repeats)
		for _, interval := range runningIntervals {
			if interval.start.After(end) {
				continue
			}
			if interval.end.Before(start) {
				continue
			}
			// if the test started before the end and ended after the start, then it was active during this time.
			activeTests[testName] = append(activeTests[testName], interval.events...)
		}
	}
	return activeTests
}

func FindFailedTestsActiveBetween(events []*monitorapi.EventInterval, start, end time.Time) []string {
	e2eTestEvents := E2ETestEvents(events)
	e2eTestsActive := E2ETestsRunningBetween(e2eTestEvents, start, end)

	failedTests := []string{}
	for testName, e2eEvents := range e2eTestsActive {
		for _, event := range e2eEvents {
			if !strings.HasPrefix(event.Message, "finishedStatus/") {
				continue
			}
			parts := strings.Split(event.Message, " ")
			parts = strings.Split(parts[0], "/")
			if parts[1] != "Passed" && parts[1] != "Skipped" {
				failedTests = append(failedTests, testName)
			}
		}
	}

	return failedTests
}
