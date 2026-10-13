package watchpods

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func stuckPendingPodsJunit(finalIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pods should not be stuck in Pending state forever"

	stuckPods := finalIntervals.Filter(func(interval monitorapi.Interval) bool {
		return interval.Source == monitorapi.SourcePodState &&
			interval.Message.Reason == "PodWasPending" &&
			interval.Message.HumanMessage == "never completed"
	})

	var tests []*junitapi.JUnitTestCase
	if len(stuckPods) > 0 {
		var failures []string
		for _, interval := range stuckPods {
			pod := interval.Locator.Keys[monitorapi.LocatorPodKey]
			namespace := interval.Locator.Keys[monitorapi.LocatorNamespaceKey]
			failures = append(failures, fmt.Sprintf("ns/%s pod/%s was Pending from %s to %s and never completed",
				namespace, pod, interval.From.UTC().Format("01-02T15:04:05Z"), interval.To.UTC().Format("01-02T15:04:05Z")))
		}

		tests = append(tests, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%d pod(s) were stuck in Pending state and never completed. "+
					"This may indicate stuck image pulls or scheduling failures.\n\n%s",
					len(failures), strings.Join(failures, "\n")),
			},
		})
		// Also add a pass entry so this shows as a flake rather than a hard failure
		// while we build confidence in this test.
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	} else {
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	}
	return tests
}
