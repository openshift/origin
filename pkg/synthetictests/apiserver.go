package synthetictests

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

func testServerAvailability(owner, locator string, events monitorapi.Intervals, jobRunDuration time.Duration) []*junitapi.JUnitTestCase {
	errDuration, errMessages, _ := monitorapi.BackendDisruptionSeconds(locator, events)

	testName := fmt.Sprintf("[%s] %s should be available throughout the test", owner, locator)
	successTest := &junitapi.JUnitTestCase{
		Name:     testName,
		Duration: jobRunDuration.Seconds(),
	}
	if errDuration > 0 {
		test := &junitapi.JUnitTestCase{
			Name:     testName,
			Duration: jobRunDuration.Seconds(),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%s was failing for %s seconds (test duration: %s)", locator, errDuration.Round(time.Second), jobRunDuration.Round(time.Second)),
			},
			SystemOut: strings.Join(errMessages, "\n"),
		}
		// Return *two* tests results to pretend this is a flake not to fail whole testsuite.
		return []*junitapi.JUnitTestCase{test, successTest}

	} else {
		successTest.SystemOut = fmt.Sprintf("%s was failing for %s seconds (test duration: %s)", locator, errDuration.Round(time.Second), jobRunDuration.Round(time.Second))
		return []*junitapi.JUnitTestCase{successTest}
	}
}

func testAllAPIAvailability(events monitorapi.Intervals, jobRunDuration time.Duration) []*junitapi.JUnitTestCase {
	allAPIServerLocators := sets.String{}
	allDisruptionEventsIntervals := events.Filter(monitorapi.IsDisruptionEvent)
	for _, eventInterval := range allDisruptionEventsIntervals {
		backend := monitorapi.DisruptionFrom(monitorapi.LocatorParts(eventInterval.Locator))
		if strings.HasSuffix(backend, "-api") {
			allAPIServerLocators.Insert(eventInterval.Locator)
		}
	}

	ret := []*junitapi.JUnitTestCase{}
	for _, apiServerLocator := range allAPIServerLocators.List() {
		ret = append(ret, testServerAvailability("sig-api-machinery", apiServerLocator, allDisruptionEventsIntervals, jobRunDuration)...)
	}

	return ret
}

func testAllIngressAvailability(events monitorapi.Intervals, jobRunDuration time.Duration) []*junitapi.JUnitTestCase {
	allAPIServerLocators := sets.String{}
	allDisruptionEventsIntervals := events.Filter(monitorapi.IsDisruptionEvent)
	for _, eventInterval := range allDisruptionEventsIntervals {
		backend := monitorapi.DisruptionFrom(monitorapi.LocatorParts(eventInterval.Locator))
		if strings.HasPrefix(backend, "ingress-") {
			allAPIServerLocators.Insert(eventInterval.Locator)
		}
	}

	ret := []*junitapi.JUnitTestCase{}
	for _, apiServerLocator := range allAPIServerLocators.List() {
		ret = append(ret, testServerAvailability("sig-network-edge", apiServerLocator, allDisruptionEventsIntervals, jobRunDuration)...)
	}

	return ret
}

func testPodNodeNameIsImmutable(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-api-machinery] the pod.spec.nodeName field is immutable, once set cannot be changed"

	failures := []string{}
	for _, event := range events {
		if strings.Contains(event.Message, "pod once assigned to a node must stay on it") {
			failures = append(failures, fmt.Sprintf("%v %v", event.Locator, event.Message))
		}
	}
	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{{Name: testName}}
	}

	return []*junitapi.JUnitTestCase{{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("Please report in https://bugzilla.redhat.com/show_bug.cgi?id=2042657\n\n%d pods had their immutable field (spec.nodeName) changed\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}}
}
