package ginkgo

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/openshift/origin/pkg/monitor"
)

// JUnitsForEvents returns a set of JUnit results for the provided events encountered
// during a test suite run.
type JUnitsForEvents interface {
	// JUnitsForEvents returns a set of additional test passes or failures implied by the
	// events sent during the test suite run. If passed is false, the entire suite is failed.
	// To set a test as flaky, return a passing and failing JUnitTestCase with the same name.
	JUnitsForEvents(events monitor.EventIntervals, duration time.Duration) (results []*JUnitTestCase, passed bool)
}

// JUnitForEventsFunc converts a function into the JUnitForEvents interface.
type JUnitForEventsFunc func(events monitor.EventIntervals, duration time.Duration) (results []*JUnitTestCase, passed bool)

func (fn JUnitForEventsFunc) JUnitsForEvents(events monitor.EventIntervals, duration time.Duration) (results []*JUnitTestCase, passed bool) {
	return fn(events, duration)
}

// JUnitsForAllEvents aggregates multiple JUnitsForEvent interfaces and returns
// the result of all invocations. It ignores nil interfaces.
type JUnitsForAllEvents []JUnitsForEvents

func (a JUnitsForAllEvents) JUnitsForEvents(events monitor.EventIntervals, duration time.Duration) (all []*JUnitTestCase, passed bool) {
	passed = true
	for _, obj := range a {
		if obj == nil {
			continue
		}
		results, passed := obj.JUnitsForEvents(events, duration)
		if !passed {
			passed = false
		}
		all = append(all, results...)
	}
	return all, passed
}

func createEventsForTests(tests []*testCase) []*monitor.EventInterval {
	eventsForTests := []*monitor.EventInterval{}
	for _, test := range tests {
		if !test.failed {
			continue
		}
		eventsForTests = append(eventsForTests,
			&monitor.EventInterval{
				From: test.start,
				To:   test.end,
				Condition: &monitor.Condition{
					Level:   monitor.Info,
					Locator: fmt.Sprintf("test=%q", test.name),
					Message: "running",
				},
			},
			&monitor.EventInterval{
				From: test.end,
				To:   test.end,
				Condition: &monitor.Condition{
					Level:   monitor.Info,
					Locator: fmt.Sprintf("test=%q", test.name),
					Message: "failed",
				},
			},
		)
	}
	return eventsForTests
}

func createSyntheticTestsFromMonitor(m *monitor.Monitor, eventsForTests []*monitor.EventInterval, monitorDuration time.Duration) ([]*JUnitTestCase, *bytes.Buffer, *bytes.Buffer) {
	var syntheticTestResults []*JUnitTestCase

	events := m.Events(time.Time{}, time.Time{})
	events = append(events, eventsForTests...)
	sort.Sort(events)

	buf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	fmt.Fprintf(buf, "\nTimeline:\n\n")
	errorCount := 0
	for _, event := range events {
		if event.Level == monitor.Error {
			errorCount++
			fmt.Fprintln(errBuf, event.String())
		}
		fmt.Fprintln(buf, event.String())
	}
	fmt.Fprintln(buf)

	if errorCount > 0 {
		syntheticTestResults = append(
			syntheticTestResults,
			&JUnitTestCase{
				Name:      "[sig-arch] Monitor cluster while tests execute",
				SystemOut: buf.String(),
				Duration:  monitorDuration.Seconds(),
				FailureOutput: &FailureOutput{
					Output: fmt.Sprintf("%d error level events were detected during this test run:\n\n%s", errorCount, errBuf.String()),
				},
			},
			// write a passing test to trigger detection of this issue as a flake, indicating we have no idea whether
			// these are actual failures or not
			&JUnitTestCase{
				Name:     "[sig-arch] Monitor cluster while tests execute",
				Duration: monitorDuration.Seconds(),
			},
		)
	}

	return syntheticTestResults, buf, errBuf
}
