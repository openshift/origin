package ginkgo

import (
	"bytes"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// JUnitsForEvents returns a set of JUnit results for the provided events encountered
// during a test suite run.
type JUnitsForEvents interface {
	// JUnitsForEvents returns a set of additional test passes or failures implied by the
	// events sent during the test suite run. If passed is false, the entire suite is failed.
	// To set a test as flaky, return a passing and failing JUnitTestCase with the same name.
	JUnitsForEvents(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string, recordedResource *monitorapi.ResourcesMap) []*junitapi.JUnitTestCase
}

// JUnitForEventsFunc converts a function into the JUnitForEvents interface.
// kubeClientConfig may or may not be present.  The JUnit evaluation needs to tolerate a missing *rest.Config
// and an unavailable cluster without crashing.
type JUnitForEventsFunc func(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string, recordedResource *monitorapi.ResourcesMap) []*junitapi.JUnitTestCase

func (fn JUnitForEventsFunc) JUnitsForEvents(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string, recordedResource *monitorapi.ResourcesMap) []*junitapi.JUnitTestCase {
	return fn(events, duration, kubeClientConfig, testSuite, recordedResource)
}

// JUnitsForAllEvents aggregates multiple JUnitsForEvent interfaces and returns
// the result of all invocations. It ignores nil interfaces.
type JUnitsForAllEvents []JUnitsForEvents

func (a JUnitsForAllEvents) JUnitsForEvents(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string, recordedResource *monitorapi.ResourcesMap) []*junitapi.JUnitTestCase {
	var all []*junitapi.JUnitTestCase
	for _, obj := range a {
		if obj == nil {
			continue
		}
		results := obj.JUnitsForEvents(events, duration, kubeClientConfig, testSuite, recordedResource)
		all = append(all, results...)
	}
	return all
}

func createSyntheticTestsFromMonitor(events monitorapi.Intervals, monitorDuration time.Duration) ([]*junitapi.JUnitTestCase, *bytes.Buffer, *bytes.Buffer) {
	var syntheticTestResults []*junitapi.JUnitTestCase

	buf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	fmt.Fprintf(buf, "\nTimeline:\n\n")
	errorCount := 0
	for _, event := range events {
		if event.Level == monitorapi.Error {
			errorCount++
			fmt.Fprintln(errBuf, event.String())
		}
		fmt.Fprintln(buf, event.String())
	}
	fmt.Fprintln(buf)

	monitorTestName := "[sig-arch] Monitor cluster while tests execute"
	if errorCount > 0 {
		syntheticTestResults = append(
			syntheticTestResults,
			&junitapi.JUnitTestCase{
				Name:      monitorTestName,
				SystemOut: buf.String(),
				Duration:  monitorDuration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("%d error level events were detected during this test run:\n\n%s", errorCount, errBuf.String()),
				},
			},
			// write a passing test to trigger detection of this issue as a flake, indicating we have no idea whether
			// these are actual failures or not
			&junitapi.JUnitTestCase{
				Name:     monitorTestName,
				Duration: monitorDuration.Seconds(),
			},
		)
	} else {
		// even if no error events, add a passed test including the output so we can scan with search.ci:
		syntheticTestResults = append(
			syntheticTestResults,
			&junitapi.JUnitTestCase{
				Name:      monitorTestName,
				Duration:  monitorDuration.Seconds(),
				SystemOut: buf.String(),
			},
		)
	}

	return syntheticTestResults, buf, errBuf
}
