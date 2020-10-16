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
	syntheticTestResults = append(syntheticTestResults, testKubeAPIServerGracefulTermination(events)...)
	syntheticTestResults = append(syntheticTestResults, testPodTransitions(events)...)
	syntheticTestResults = append(syntheticTestResults, testSystemDTimeout(events)...)
	syntheticTestResults = append(syntheticTestResults, testPodSandboxCreation(events)...)

	return syntheticTestResults, buf, errBuf
}

func testKubeAPIServerGracefulTermination(events []*monitor.EventInterval) []*JUnitTestCase {
	const testName = "[sig-node] kubelet terminates kube-apiserver gracefully"
	success := &JUnitTestCase{Name: testName}

	failures := []string{}
	for _, event := range events {
		// from https://github.com/openshift/kubernetes/blob/1f35e4f63be8fbb19e22c9ff1df31048f6b42ddf/cmd/watch-termination/main.go#L96
		if strings.Contains(event.Message, "did not terminate gracefully") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
		}
	}
	if len(failures) == 0 {
		return []*JUnitTestCase{success}
	}

	failure := &JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &FailureOutput{
			Output: fmt.Sprintf("%d kube-apiserver reports a non-graceful termination. Probably kubelet or CRI-O is not giving the time to cleanly shut down. This can lead to connection refused and network I/O timeout errors in other components.\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}

	// This should fail a CI run, not flake it.
	return []*JUnitTestCase{failure}

}

func testPodTransitions(events []*monitor.EventInterval) []*JUnitTestCase {
	const testName = "[sig-node] pods should never transition back to pending"
	success := &JUnitTestCase{Name: testName}

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
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &FailureOutput{
			Output: fmt.Sprintf("%d pods illegally transitioned to Pending\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}

	// TODO an upgrade job that starts before 4.6 may need to make this test flake instead of fail.  This will depend on which `openshift-tests`
	//  is used to run that upgrade test.  I recommend waiting to a flake until we know and even then find a way to constrain it.
	return []*JUnitTestCase{failure}
}

func testSystemDTimeout(events []*monitor.EventInterval) []*JUnitTestCase {
	const testName = "[sig-node] pods should not fail on systemd timeouts"
	success := &JUnitTestCase{Name: testName}

	failures := []string{}
	for _, event := range events {
		if strings.Contains(event.Message, "systemd timed out for pod") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
		}
	}
	if len(failures) == 0 {
		return []*JUnitTestCase{success}
	}

	failure := &JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &FailureOutput{
			Output: fmt.Sprintf("%d systemd timed out for pod occurrences\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}

	// write a passing test to trigger detection of this issue as a flake. Doing this first to try to see how frequent the issue actually is
	return []*JUnitTestCase{failure, success}
}

func testPodSandboxCreation(events []*monitor.EventInterval) []*JUnitTestCase {
	const testName = "[sig-network] pods should successfully create sandboxes"
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
		successes = append(successes, &JUnitTestCase{Name: testName + by.by})
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
			Name:      testName + by,
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
