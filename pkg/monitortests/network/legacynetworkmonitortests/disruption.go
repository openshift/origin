package legacynetworkmonitortests

import (
	"fmt"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/kustomize/kyaml/sets"
)

func TestMultipleSingleSecondDisruptions(events monitorapi.Intervals, clientConfig *rest.Config) []*junitapi.JUnitTestCase {
	// multipleFailuresTestPrefix is for tests that track a few single second disruptions
	const multipleFailuresTestPrefix = "[sig-network] there should be nearly zero single second disruptions for "
	// manyFailureTestPrefix is for tests that track a lot of single second disruptions (more severe than the above)
	const manyFailureTestPrefix = "[sig-network] there should be reasonably few single second disruptions for "

	platform := configv1.NonePlatformType
	if clientConfig != nil {
		if actualPlatform, err := getPlatformType(clientConfig); err == nil {
			platform = actualPlatform
		}
	}

	allServers := sets.String{}
	allDisruptionEventsIntervals := events.Filter(monitorapi.IsDisruptionEvent).Filter(monitorapi.HasRealLoadBalancer)
	logrus.Infof("filtered %d intervals down to %d disruption intervals", len(events), len(allDisruptionEventsIntervals))
	for _, eventInterval := range allDisruptionEventsIntervals {
		backend := eventInterval.Locator.Keys[monitorapi.LocatorBackendDisruptionNameKey]
		loadbalancer := eventInterval.Locator.Keys[monitorapi.LocatorLoadBalancerKey]
		connection := eventInterval.Locator.Keys[monitorapi.LocatorConnectionKey]

		switch {
		case strings.HasPrefix(backend, "ingress-"):
			allServers.Insert(backend)
		case strings.Contains(backend, "-api-"):
			if loadbalancer == "internal-lb" && connection == "reused" && platform == configv1.AWSPlatformType {
				// OCPBUGS-43483: ignore internal-lb disruptions on AWS
				continue
			}
			allServers.Insert(backend)
		}
	}
	logrus.Infof("allServers = %s", allServers)

	ret := []*junitapi.JUnitTestCase{}
	for _, backend := range allServers.List() {
		allDisruptionEvents := events.Filter(
			monitorapi.And(
				monitorapi.IsForDisruptionBackend(backend),
				monitorapi.IsErrorEvent,
			),
		)
		logrus.Infof("found %d disruption events for backend %s", len(allDisruptionEvents), backend)

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

		multipleFailuresTestName := multipleFailuresTestPrefix + backend
		manyFailuresTestName := manyFailureTestPrefix + backend
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
				Output: fmt.Sprintf("%s had %v single second disruptions", backend, len(disruptionEvents)),
			},
			SystemOut: strings.Join(disruptionEvents.Strings(), "\n"),
		}
		manyFailuresFail := &junitapi.JUnitTestCase{
			Name: manyFailuresTestName,
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%s had %v single second disruptions", backend, len(disruptionEvents)),
			},
			SystemOut: strings.Join(disruptionEvents.Strings(), "\n"),
		}

		switch {
		case len(disruptionEvents) >= 50: // 50 chosen to be big enough that we should not hit this unless something is really really wrong
			ret = append(ret, multipleFailuresFail, manyFailuresFail)

		case len(disruptionEvents) > 20: // 20 chosen to be big enough that we should not hit this unless something is weird
			ret = append(ret, multipleFailuresFail, manyFailuresPass)

		default: // Less than 20 = pass both tests
			ret = append(ret, multipleFailuresPass, manyFailuresPass)
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
		if event.Message.Reason == "DisruptionSamplerOutageBegan" {
			dnsIntervals = append(dnsIntervals, event)
		}
		// real disruption
		if event.Message.Reason == "DisruptionBegan" {
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
