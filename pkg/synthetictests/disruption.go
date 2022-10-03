package synthetictests

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/synthetictests/allowedbackenddisruption"
	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/extended/util/disruption"
	"github.com/openshift/origin/test/extended/util/disruption/externalservice"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

func testServerAvailability(
	owner, locator string,
	events monitorapi.Intervals,
	jobRunDuration time.Duration,
	restConfig *rest.Config) []*junitapi.JUnitTestCase {

	testName := fmt.Sprintf("[%s] %s should be available throughout the test", owner, locator)

	// Lookup allowed disruption based on historical data:
	locatorParts := monitorapi.LocatorParts(locator)
	disruptionName := monitorapi.DisruptionFrom(locatorParts)
	connType := monitorapi.DisruptionConnectionTypeFrom(locatorParts)
	backendName := fmt.Sprintf("%s-%s-connections", disruptionName, connType)
	jobType, err := platformidentification.GetJobType(context.TODO(), restConfig)
	if err != nil {
		return []*junitapi.JUnitTestCase{
			{
				Name:     testName,
				Duration: jobRunDuration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("error in platform identification: %s", err),
				},
			},
		}
	}

	allowedDisruption, disruptionDetails, err :=
		allowedbackenddisruption.GetAllowedDisruption(backendName, *jobType)
	if err != nil {
		return []*junitapi.JUnitTestCase{
			{
				Name:     testName,
				Duration: jobRunDuration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("error in getting allowed disruption: %s", err),
				},
			},
		}
	}

	// Check if we got an empty result, which signals we did not have historical data for this NURP and thus
	// do not want to run the test.
	if allowedDisruption == nil {
		// An empty StatisticalDuration implies we did not find any data and thus do not want to run the disruption
		// test. We'll mark it as a flake and explain why so we can find these tests should anyone need to look.
		return []*junitapi.JUnitTestCase{
			{
				Name:     testName,
				Duration: jobRunDuration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("skipping test due to no historical disruption data: %s", disruptionDetails),
				},
			},
			{
				Name: testName,
			},
		}
	}

	roundedAllowedDisruption := allowedDisruption.Round(time.Second)
	if allowedDisruption.Milliseconds() == disruption.DefaultAllowedDisruption {
		// don't round if we're using the default value so we can find this.
		roundedAllowedDisruption = *allowedDisruption
	}
	framework.Logf("allowedDisruption for backend %s: %s, details: disruptionDetails",
		backendName, roundedAllowedDisruption, disruptionDetails)

	observedDisruption, disruptionMsgs, _ := monitorapi.BackendDisruptionSeconds(locator, events)

	resultsStr := fmt.Sprintf(
		"%s was unreachable during disruption testing for at least %s of %s (maxAllowed=%s):\n\n%s",
		backendName, observedDisruption, jobRunDuration.Round(time.Second), roundedAllowedDisruption, disruptionDetails)
	successTest := &junitapi.JUnitTestCase{
		Name:     testName,
		Duration: jobRunDuration.Seconds(),
	}
	if observedDisruption > roundedAllowedDisruption {
		test := &junitapi.JUnitTestCase{
			Name:     testName,
			Duration: jobRunDuration.Seconds(),
			FailureOutput: &junitapi.FailureOutput{
				Output: resultsStr,
			},
			SystemOut: strings.Join(disruptionMsgs, "\n"),
		}
		return []*junitapi.JUnitTestCase{test}
	} else {
		successTest.SystemOut = resultsStr
		return []*junitapi.JUnitTestCase{successTest}
	}
}

func testAllAPIBackendsForDisruption(events monitorapi.Intervals, jobRunDuration time.Duration, restConfig *rest.Config) []*junitapi.JUnitTestCase {
	disruptLocators := sets.String{}
	allDisruptionEventsIntervals := events.Filter(monitorapi.IsDisruptionEvent)
	for _, eventInterval := range allDisruptionEventsIntervals {
		backend := monitorapi.DisruptionFrom(monitorapi.LocatorParts(eventInterval.Locator))
		if strings.HasSuffix(backend, "-api") {
			disruptLocators.Insert(eventInterval.Locator)
		}
	}

	ret := []*junitapi.JUnitTestCase{}
	for _, locator := range disruptLocators.List() {
		ret = append(ret, testServerAvailability("sig-api-machinery", locator, allDisruptionEventsIntervals, jobRunDuration, restConfig)...)
	}

	return ret
}

func testAllIngressBackendsForDisruption(events monitorapi.Intervals, jobRunDuration time.Duration, restConfig *rest.Config) []*junitapi.JUnitTestCase {
	disruptLocators := sets.String{}
	allDisruptionEventsIntervals := events.Filter(monitorapi.IsDisruptionEvent)
	for _, eventInterval := range allDisruptionEventsIntervals {
		backend := monitorapi.DisruptionFrom(monitorapi.LocatorParts(eventInterval.Locator))
		if strings.HasPrefix(backend, "ingress-") {
			disruptLocators.Insert(eventInterval.Locator)
		}
	}

	ret := []*junitapi.JUnitTestCase{}
	for _, locator := range disruptLocators.List() {
		ret = append(ret, testServerAvailability("sig-network-edge", locator, allDisruptionEventsIntervals, jobRunDuration, restConfig)...)
	}

	return ret
}

// testExternalBackendsForDisruption runs synthetic tests for disruption backends that don't fit into the above two categories.
func testExternalBackendsForDisruption(events monitorapi.Intervals, jobRunDuration time.Duration, restConfig *rest.Config) []*junitapi.JUnitTestCase {
	disruptLocators := sets.String{}
	allDisruptionEventsIntervals := events.Filter(monitorapi.IsDisruptionEvent)
	for _, eventInterval := range allDisruptionEventsIntervals {
		backend := monitorapi.DisruptionFrom(monitorapi.LocatorParts(eventInterval.Locator))
		if backend == externalservice.LivenessProbeBackend {
			disruptLocators.Insert(eventInterval.Locator)
		}
	}

	ret := []*junitapi.JUnitTestCase{}
	for _, locator := range disruptLocators.List() {
		ret = append(ret, testServerAvailability("sig-trt", locator, allDisruptionEventsIntervals, jobRunDuration, restConfig)...)
	}

	return ret
}

func testMultipleSingleSecondDisruptions(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const multipleFailuresTestPrefix = "[sig-network] there should be nearly zero single second disruptions for "
	const manyFailureTestPrefix = "[sig-network] there should be reasonably few single second disruptions for "

	allServers := sets.String{}
	allDisruptionEventsIntervals := events.Filter(monitorapi.IsDisruptionEvent)
	for _, eventInterval := range allDisruptionEventsIntervals {
		backend := monitorapi.DisruptionFrom(monitorapi.LocatorParts(eventInterval.Locator))
		switch {
		case strings.HasPrefix(backend, "ingress-"):
			allServers.Insert(eventInterval.Locator)
		case strings.HasSuffix(backend, "-api"):
			allServers.Insert(eventInterval.Locator)
		}
	}

	ret := []*junitapi.JUnitTestCase{}
	for _, serverLocator := range allServers.List() {
		allDisruptionEvents := events.Filter(
			monitorapi.And(
				monitorapi.IsEventForLocator(serverLocator),
				monitorapi.IsErrorEvent,
			),
		)

		disruptionEvents := monitorapi.Intervals{}
		for i, interval := range allDisruptionEvents {
			if !isOneSecondEvent(interval) {
				continue
			}
			if i > 0 {
				prev := allDisruptionEvents[i-1]
				// if the previous disruption interval for this backend is within one second of when this one started,
				// then we're looking at a contiguous outage that is longer than one second.
				// this can happen when we have contiguous failures for different reasons.
				if prev.To.Add(1 * time.Second).After(interval.From) {
					continue
				}
			}
			if i < len(allDisruptionEvents)-1 {
				next := allDisruptionEvents[i+1]
				// if the next disruption interval for this backend is within one second of when this one ended,
				// then we're looking at a contiguous outage that is longer than one second.
				// this can happen when we have contiguous failures for different reasons.
				if interval.To.Add(1 * time.Second).After(next.From) {
					continue
				}
			}

			disruptionEvents = append(disruptionEvents, allDisruptionEvents[i])
		}

		multipleFailuresTestName := multipleFailuresTestPrefix + serverLocator
		manyFailuresTestName := manyFailureTestPrefix + serverLocator
		multipleFailuresPass := &junitapi.JUnitTestCase{
			Name:      multipleFailuresTestName,
			SystemOut: strings.Join(disruptionEvents.Strings(), "\n"),
		}
		manyFailuresPass := &junitapi.JUnitTestCase{
			Name:      manyFailuresTestName,
			SystemOut: strings.Join(disruptionEvents.Strings(), "\n"),
		}
		multipleFailuresFail := &junitapi.JUnitTestCase{
			Name: multipleFailuresTestName,
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%s had %v single second disruptions", serverLocator, len(disruptionEvents)),
			},
			SystemOut: strings.Join(disruptionEvents.Strings(), "\n"),
		}
		manyFailuresFail := &junitapi.JUnitTestCase{
			Name: manyFailuresTestName,
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%s had %v single second disruptions", serverLocator, len(disruptionEvents)),
			},
			SystemOut: strings.Join(disruptionEvents.Strings(), "\n"),
		}

		switch {
		case len(disruptionEvents) < 20:
			ret = append(ret, multipleFailuresPass, manyFailuresPass)

		case len(disruptionEvents) > 20: // chosen to be big enough that we should not hit this unless something is weird
			ret = append(ret, multipleFailuresFail, manyFailuresPass)

		case len(disruptionEvents) > 50: // chosen to be big enough that we should not hit this unless something is really really wrong
			ret = append(ret, multipleFailuresFail, manyFailuresFail)
		}
	}

	return ret
}

func isOneSecondEvent(eventInterval monitorapi.EventInterval) bool {
	duration := eventInterval.To.Sub(eventInterval.From)
	switch {
	case duration <= 0:
		return false
	case duration > time.Second:
		return false
	default:
		return true
	}
}
