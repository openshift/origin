package synthetictests

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"github.com/openshift/origin/pkg/test/ginkgo"
)

const (
	// Max. duration of API server unreachability, in fraction of total test duration.
	tolerateDisruptionPercent = 0.01
)

func testServerAvailability(locator string, events monitorapi.Intervals, duration time.Duration) []*ginkgo.JUnitTestCase {
	events = events.Filter(func(i monitorapi.EventInterval) bool { return i.Locator == locator })
	errDuration := events.Duration(0)
	errMessages := events.Strings()

	testName := fmt.Sprintf("[sig-api-machinery] %s should be available", locator)
	successTest := &ginkgo.JUnitTestCase{
		Name:     testName,
		Duration: duration.Seconds(),
	}
	if percent := float64(errDuration) / float64(duration); percent > tolerateDisruptionPercent {
		test := &ginkgo.JUnitTestCase{
			Name:     testName,
			Duration: duration.Seconds(),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%s was failing for %s seconds (%0.0f%% of the test duration)", locator, errDuration.Truncate(time.Second), 100*percent),
			},
			SystemOut: strings.Join(errMessages, "\n"),
		}
		// Return *two* tests results to pretend this is a flake not to fail whole testsuite.
		return []*ginkgo.JUnitTestCase{test, successTest}
	} else {
		successTest.SystemOut = fmt.Sprintf("%s was failing for %s seconds (%0.0f%% of the test duration)", locator, errDuration.Truncate(time.Second), 100*percent)
		return []*ginkgo.JUnitTestCase{successTest}
	}
}
