package ginkgo

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor"
)

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

	// check events
	syntheticTestResults = append(syntheticTestResults, testPodTransitions(events)...)
	syntheticTestResults = append(syntheticTestResults, testPodSandboxCreation(events)...)

	return syntheticTestResults, buf, errBuf
}

func testPodTransitions(events []*monitor.EventInterval) []*JUnitTestCase {
	success := &JUnitTestCase{Name: "[sig-node] pods should never transition back to pending"}

	failures := []string{}
	for _, event := range events {
		if strings.Contains(event.Message, "pod should not transition") || strings.Contains(event.Message, "pod moved back to Pending") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
		}
	}
	if len(failures) == 0 {
		return []*JUnitTestCase{success}
	}

	failure := &JUnitTestCase{
		Name:      "[sig-node] pods should never transition back to pending",
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &FailureOutput{
			Output: fmt.Sprintf("%d pods illegally transitioned to Pending\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}

	// write a passing test to trigger detection of this issue as a flake. Doing this first to try to see how frequent the issue actually is
	return []*JUnitTestCase{failure, success}
}

func testPodSandboxCreation(events []*monitor.EventInterval) []*JUnitTestCase {
	// we can further refine this signal by subdividing different failure modes if it is pertinent.  Right now I'm seeing
	// 1. error reading container (probably exited) json message: EOF
	// 2. dial tcp 10.0.76.225:6443: i/o timeout
	// 3. error getting pod: pods "terminate-cmd-rpofb45fa14c-96bb-40f7-bd9e-346721740cac" not found
	// 4. write child: broken pipe
	bySubStrings := []struct {
		by        string
		substring string
	}{
		{by: " by reading container", substring: "error reading container (probably exited) json message: EOF"},
		{by: " by not timing out", substring: "i/o timeout"},
		{by: " by writing network status", substring: "error setting the networks status"},
		{by: " by getting pod", substring: " error getting pod: pods"},
		{by: " by writing child", substring: "write child: broken pipe"},
		{by: " by other", substring: " "}, // always matches
	}

	successes := []*JUnitTestCase{}
	for _, by := range bySubStrings {
		successes = append(successes, &JUnitTestCase{Name: "[sig-network] pods should successfully create sandboxes" + by.by})
	}

	failures := []string{}
	for _, event := range events {
		if strings.Contains(event.Message, "reason/FailedCreatePodSandBox Failed to create pod sandbox") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
		}
	}
	if len(failures) == 0 {
		return successes
	}

	ret := []*JUnitTestCase{}
	failuresBySubtest := map[string][]string{}
	for _, failure := range failures {
		for _, by := range bySubStrings {
			if strings.Contains(failure, by.substring) {
				failuresBySubtest[by.by] = append(failuresBySubtest[by.by], failure)
				break // break after first match so we only add each failure one bucket
			}
		}
	}

	// now iterate the individual failures to create failure entries
	for by, subFailures := range failuresBySubtest {
		failure := &JUnitTestCase{
			Name:      "[sig-network] pods should successfully create sandboxes" + by,
			SystemOut: strings.Join(subFailures, "\n"),
			FailureOutput: &FailureOutput{
				Output: fmt.Sprintf("%d failures to create the sandbox\n\n%v", len(subFailures), strings.Join(subFailures, "\n")),
			},
		}
		ret = append(ret, failure)
	}

	// write a passing test to trigger detection of this issue as a flake. Doing this first to try to see how frequent the issue actually is
	return append(ret, successes...)
}
