package synthetictests

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/duplicateevents"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

const (
	msgNodeReboot        = "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started"
	msgAPIServerShutdown = "reason/ShutdownInitiated Received signal to terminate, becoming unready, but keeping serving"
)

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

func testOauthApiserverProbeErrorLiveness(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on liveiness probe due to timeout"
	return makeProbeTest(testName, events, duplicateevents.ProbeErrorLivenessMessageRegExpStr, "openshift-oauth-apiserver", duplicateevents.DuplicateEventThreshold)
}

func testOauthApiserverProbeErrorReadiness(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on readiiness probe due to timeout"
	return makeProbeTest(testName, events, duplicateevents.ProbeErrorReadinessMessageRegExpStr, "openshift-oauth-apiserver", duplicateevents.DuplicateEventThreshold)
}

func testOauthApiserverProbeErrorConnectionRefused(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on readiiness probe due to connection refused"
	return makeProbeTest(testName, events, duplicateevents.ProbeErrorConnectionRefusedRegExpStr, "openshift-oauth-apiserver", duplicateevents.DuplicateEventThreshold)
}

func testAPIServerRecievedShutdownSignal(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-api-machinery] kube-apiserver pod should receive shutdown signal when master nodes reboot"

	rebootEvents := []*monitorapi.EventInterval{}
	apiShutdownEvents := []*monitorapi.EventInterval{}
	for i := range events {
		event := &events[i]
		if strings.Contains(event.Message, msgNodeReboot) {
			rebootEvents = append(rebootEvents, event)
		}
		if strings.Contains(event.Message, msgAPIServerShutdown) {
			apiShutdownEvents = append(apiShutdownEvents, event)
		}
	}

	failures := []string{}
	found := false
	for _, rebootEvent := range rebootEvents {
		found = false
		for _, apiEvent := range apiShutdownEvents {
			if apiEvent.From.Before(rebootEvent.To) && apiEvent.From.After(rebootEvent.From) {
				found = true
				break
			}
		}
		if !found {
			failures = append(failures, fmt.Sprintf("missing apiserver shutdown event for %v from %s to %s", rebootEvent.Locator, rebootEvent.From.Format("15:04:05Z"), rebootEvent.To.Format("15:04:05Z")))
		}
	}
	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{{Name: testName}}
	}

	return []*junitapi.JUnitTestCase{{
		Name: testName,
		FailureOutput: &junitapi.FailureOutput{
			Output: strings.Join(failures, "\n"),
		},
	}}
}
