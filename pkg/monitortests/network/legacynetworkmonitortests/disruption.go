package legacynetworkmonitortests

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"sigs.k8s.io/kustomize/kyaml/sets"
)

func testMultipleSingleSecondDisruptions(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const multipleFailuresTestPrefix = "[sig-network] there should be nearly zero single second disruptions for "
	const manyFailureTestPrefix = "[sig-network] there should be reasonably few single second disruptions for "

	allServers := sets.String{}
	allDisruptionEventsIntervals := events.Filter(monitorapi.IsDisruptionEvent)
	for _, eventInterval := range allDisruptionEventsIntervals {
		backend := monitorapi.ThisDisruptionInstanceFrom(monitorapi.LocatorParts(eventInterval.Locator))
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

func isOneSecondEvent(eventInterval monitorapi.Interval) bool {
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

func testDNSOverlapDisruption(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-network] Disruption should not overlap with DNS problems in cluster running tests"
	failures := []string{}
	dnsIntervals := []monitorapi.Interval{}
	disruptionIntervals := []monitorapi.Interval{}
	for _, event := range events {
		// DNS outage
		if reason := monitorapi.ReasonFrom(event.Message); reason == "DisruptionSamplerOutageBegan" {
			dnsIntervals = append(dnsIntervals, event)
		}
		// real disruption
		if reason := monitorapi.ReasonFrom(event.Message); reason == "DisruptionBegan" {
			disruptionIntervals = append(disruptionIntervals, event)
		}
	}
	errorCount := 0
	for _, r := range disruptionIntervals {
		for _, d := range dnsIntervals {
			if (r.From.Before(d.To) && d.From.Before(r.To)) || (r.To.Add(10*time.Second).After(d.From) && d.To.Add(10*time.Second).After(r.From)) {
				errorCount = errorCount + 1
			}
		}
	}
	if errorCount > 0 {
		failures = append(failures, fmt.Sprintf("Overlap or interval within 10 seconds occured %d times.", errorCount))
	}
	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{
			{Name: testName},
		}
	}

	output := "These failures imply disruption overlapped, or occurred in very close proximity to DNS problems in the cluster running tests.\n" + strings.Join(failures, "\n")

	return []*junitapi.JUnitTestCase{
		{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
			SystemOut: strings.Join(failures, "\n"),
		},
		{
			Name: testName,
		},
	}
}
