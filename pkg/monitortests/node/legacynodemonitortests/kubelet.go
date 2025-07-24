package legacynodemonitortests

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/apimachinery/pkg/util/sets"
)

func testKubeletToAPIServerGracefulTermination(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] kubelet terminates kube-apiserver gracefully"

	var failures []string
	for _, event := range events {
		if strings.Contains(event.Message.HumanMessage, "did not terminate gracefully") || event.Message.Reason == "NonGracefulTermination" {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}

	// failures during a run always fail the test suite
	var tests []*junitapi.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%d kube-apiserver reports a non-graceful termination.  Probably kubelet or CRI-O is not giving the time to cleanly shut down. This can lead to connection refused and network I/O timeout errors in other components.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})

		// while waiting for https://bugzilla.redhat.com/show_bug.cgi?id=1928946 mark as flake
		tests[0].FailureOutput.Output = "Marked flake while fix for https://bugzilla.redhat.com/show_bug.cgi?id=1928946 is identified:\n\n" + tests[0].FailureOutput.Output
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	}

	if len(tests) == 0 {
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	}
	return tests
}

func testErrImagePullUnrecognizedSignatureFormat(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] kubelet logs do not contain ErrImagePull unrecognized signature format"
	success := &junitapi.JUnitTestCase{Name: testName}

	var failures []string
	for _, event := range events {
		if event.Message.Reason == "ErrImagePull" && strings.Contains(event.Message.HumanMessage, "UnrecognizedSignatureFormat") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}

	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("%d kubelet logs contain errors from ErrImagePull unrecognized signature format.\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}
	// TODO: marked flaky until we have monitored it for consistency
	return []*junitapi.JUnitTestCase{failure, success}
}

func testHttpConnectionLost(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] kubelet logs do not contain http client connection lost errors"
	success := &junitapi.JUnitTestCase{Name: testName}

	var failures []string
	for _, event := range events {
		if event.Message.Reason == "HttpClientConnectionLost" {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}

	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("%d kubelet logs contain errors from http client connections lost unexpectedly.\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}
	// TODO: marked flaky until we have monitored it for consistency
	return []*junitapi.JUnitTestCase{failure, success}
}

func testLeaseUpdateError(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] kubelet logs do not contain late lease update errors"
	success := &junitapi.JUnitTestCase{Name: testName}

	var failures []string
	var firstEventTime time.Time

	for _, event := range events {

		if firstEventTime.IsZero() {
			firstEventTime = event.From
			continue
		}

		if event.Message.Reason == "FailedToUpdateLease" {
			// start with a 30 minute grace period as we do see some FailedToUpdateLease errors
			// under normal conditions, it is the late ones that indicate a node may have go unready unexpectedly
			// if we get false hits we can increase the grace period
			if event.From.After(firstEventTime.Add(30 * time.Minute)) {
				failures = append(failures, fmt.Sprintf("%s %v - %v", event.From.Format(time.RFC3339), event.Locator.OldLocator(), event.Message.OldMessage()))
			}
		}
	}

	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("%d late updating lease errors contained in kubelet logs.\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}
	// TODO: marked flaky until we have monitored it for consistency
	return []*junitapi.JUnitTestCase{failure, success}
}

func testAnonymousCertConnectionFailure(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] kubelet should not use an anonymous user"

	var failures []string
	for _, event := range events {
		if event.Source == monitorapi.SourceKubeletLog && event.Message.Reason == "FailedToAuthenticateWithOpenShiftUser" {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}

	if len(failures) == 0 {
		success := &junitapi.JUnitTestCase{Name: testName}
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("kubelet logs contain %d failures using an anonymous user .\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}
	// add success to flake the test because this fails very commonly.
	success := &junitapi.JUnitTestCase{Name: testName}
	return []*junitapi.JUnitTestCase{failure, success}
}

func testFailedToDeleteCGroupsPath(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] kubelet should be able to delete cgroups path"

	var failures []string
	for _, event := range events {
		if event.Message.Reason == "FailedToDeleteCGroupsPath" {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}

	if len(failures) == 0 {
		success := &junitapi.JUnitTestCase{Name: testName}
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("kubelet logs contain %d failures to delete cgroups path.\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}
	// add success to flake the test because this fails very commonly.
	success := &junitapi.JUnitTestCase{Name: testName}
	return []*junitapi.JUnitTestCase{failure, success}
}

func testKubeAPIServerGracefulTermination(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-api-machinery] kube-apiserver terminates within graceful termination period"

	var failures []string
	for _, event := range events {
		if event.Message.Reason == "GracefulTerminationTimeout" {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}

	// failures during a run always fail the test suite
	var tests []*junitapi.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%d kube-apiserver reports a non-graceful termination. This is a bug in kube-apiserver. It probably means that network connections are not closed cleanly, and this leads to network I/O timeout errors in other components.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
	}

	if len(tests) == 0 {
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	}
	return tests
}

func testKubeApiserverProcessOverlap(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] overlapping apiserver process detected during kube-apiserver rollout"
	success := &junitapi.JUnitTestCase{Name: testName}
	failures := []string{}
	for _, event := range events {
		if event.Message.Reason == "TerminationProcessOverlapDetected" {
			failures = append(failures, fmt.Sprintf("[%s - %s] %s", event.From, event.To, event.Locator.OldLocator()))
		}
	}

	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("The following events detected:\n\n%s", strings.Join(failures, "\n")),
		},
	}
	return []*junitapi.JUnitTestCase{failure}
}

func testDeleteGracePeriodZero(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-architecture] platform pods should not be force deleted with gracePeriod 0"
	success := &junitapi.JUnitTestCase{Name: testName}

	failures := []string{}
	for _, event := range events {
		if event.Message.Reason != "ForceDelete" {
			continue
		}
		if !platformidentification.IsPlatformNamespace(event.Locator.Keys[monitorapi.LocatorNamespaceKey]) {
			continue
		}
		if event.Message.Annotations["mirrored"] == "true" {
			continue
		}
		if len(event.Locator.Keys[monitorapi.LocatorNodeKey]) == 0 {
			continue
		}
		failures = append(failures, event.Locator.OldLocator())
	}
	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("The following pods were force deleted and should not be:\n\n%s", strings.Join(failures, "\n")),
		},
	}
	// TODO: marked flaky until has been thoroughly debugged
	return []*junitapi.JUnitTestCase{failure, success}
}

func testPodTransitions(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pods should never transition back to pending"
	success := &junitapi.JUnitTestCase{Name: testName}

	failures := []string{}
	for _, event := range events {
		if strings.Contains(event.Message.HumanMessage, "pod should not transition") || strings.Contains(event.Message.HumanMessage, "pod moved back to Pending") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}
	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("Marked as flake until https://bugzilla.redhat.com/show_bug.cgi?id=1933760 is fixed\n\n%d pods illegally transitioned to Pending\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}
	// TODO: temporarily marked flaky since it is continuously failing
	return []*junitapi.JUnitTestCase{failure, success}
}

func formatTimes(times []time.Time) []string {
	var s []string
	for _, t := range times {
		s = append(s, t.UTC().Format(time.RFC3339))
	}
	return s
}

func testNodeUpgradeTransitions(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] nodes should not go unready after being upgraded and go unready only once"

	var buf bytes.Buffer
	var testCases []*junitapi.JUnitTestCase

	for len(events) > 0 {
		nodesWentReady, nodesWentUnready := make(map[string][]time.Time), make(map[string][]time.Time)
		currentNodeReady := make(map[string]bool)

		var text string
		var failures []string
		var foundEnd bool
		for i, event := range events {
			// treat multiple sequential upgrades as distinct test failures
			if event.Locator.Keys[monitorapi.LocatorClusterVersionKey] == "cluster" &&
				(event.Message.Reason == monitorapi.UpgradeStartedReason ||
					event.Message.Reason == monitorapi.UpgradeRollbackReason) {

				text = event.Message.HumanMessage
				events = events[i+1:]
				foundEnd = true
				fmt.Fprintf(&buf, "DEBUG: found upgrade start event: %v\n", event.String())
				break
			}
			node, isNode := event.Locator.Keys[monitorapi.LocatorNodeKey]
			if !isNode {
				continue
			}
			if event.Message.Annotations[monitorapi.AnnotationCondition] != "Ready " || !strings.HasSuffix(event.Message.HumanMessage, " changed") {
				continue
			}
			if event.Message.Annotations[monitorapi.AnnotationStatus] == "True" {
				if currentNodeReady[node] {
					failures = append(failures, fmt.Sprintf("Node %s was reported ready twice in a row, this should be impossible", node))
					continue
				}
				currentNodeReady[node] = true

				// In some cases, nodes take a long time to reach Ready, occasionally reaching Ready for the first time after the upgrade
				// test suite has already started. This triggers the supposedly impossible "node went ready multiple times" thrown below,
				// when in reality the upgrade proceeded quite smoothly. As such we'll disregard a move to NodeReady, if we have not yet
				// observed a NodeNotReady.
				if len(nodesWentUnready[node]) > 0 {
					nodesWentReady[node] = append(nodesWentReady[node], event.From)
				} else {
					fmt.Fprintf(&buf, "DEBUG: detected NodeReady after UpgradeStarted, but before the node went NodeNotReady (likely indicates a node that was slow to initially install): %v\n", event.String())
				}

				fmt.Fprintf(&buf, "DEBUG: found node ready transition: %v\n", event.String())
			} else {
				if !currentNodeReady[node] && len(nodesWentUnready[node]) > 0 {
					fmt.Fprintf(&buf, "DEBUG: node already considered not ready, ignoring: %v\n", event.String())
					continue
				}
				currentNodeReady[node] = false
				nodesWentUnready[node] = append(nodesWentUnready[node], event.From)
				fmt.Fprintf(&buf, "DEBUG: found node not ready transition: %v\n", event.String())
			}
		}
		if !foundEnd {
			events = nil
		}

		abnormalNodes := sets.NewString()
		for node, unready := range nodesWentUnready {
			ready := nodesWentReady[node]

			if len(unready) > 1 {
				failures = append(failures, fmt.Sprintf("Node %s went unready multiple times: %s", node, strings.Join(formatTimes(unready), ", ")))
				abnormalNodes.Insert(node)
			}
			if len(ready) > 1 {
				failures = append(failures, fmt.Sprintf("Node %s went ready multiple times: %s", node, strings.Join(formatTimes(ready), ", ")))
				abnormalNodes.Insert(node)
			}

			switch {
			case len(ready) == 0, len(unready) == 1 && len(ready) == 1 && !unready[0].Before(ready[0]):
				failures = append(failures, fmt.Sprintf("Node %s went unready at %s, never became ready again", node, strings.Join(formatTimes(unready[:1]), ", ")))
				abnormalNodes.Insert(node)
			}
		}
		for node, ready := range nodesWentReady {
			unready := nodesWentUnready[node]
			if len(unready) > 0 {
				continue
			}

			switch {
			case len(ready) > 1:
				failures = append(failures, fmt.Sprintf("Node %s went ready multiple times without going unready: %s", node, strings.Join(formatTimes(ready), ", ")))
				abnormalNodes.Insert(node)
			case len(ready) == 1:
				failures = append(failures, fmt.Sprintf("Node %s went ready at %s but had no record of going unready, may not have upgraded", node, strings.Join(formatTimes(ready), ", ")))
				abnormalNodes.Insert(node)
			}
		}

		if len(failures) == 0 {
			continue
		}

		testCases = append(testCases, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: fmt.Sprintf("%s\n\n%s", strings.Join(failures, "\n"), buf.String()),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%d nodes violated upgrade expectations:\n\n%s\n\n%s", abnormalNodes.Len(), strings.Join(failures, "\n"), text),
			},
		})
	}
	if len(testCases) == 0 {
		testCases = append(testCases, &junitapi.JUnitTestCase{Name: testName})
	}
	return testCases
}

func testSystemDTimeout(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pods should not fail on systemd timeouts"
	success := &junitapi.JUnitTestCase{Name: testName}

	failures := []string{}
	for _, event := range events {
		if strings.Contains(event.Message.HumanMessage, "systemd timed out for pod") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}
	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("%d systemd timed out for pod occurrences\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}

	// write a passing test to trigger detection of this issue as a flake. Doing this first to try to see how frequent the issue actually is
	return []*junitapi.JUnitTestCase{failure, success}
}

var errImagePullTimeoutRE = regexp.MustCompile("ErrImagePull.*read: connection timed out")
var errImagePullGenericRE = regexp.MustCompile("ErrImagePull")
var errImagePullQPSExceededRE = regexp.MustCompile("ErrImagePull.*pull QPS exceeded")
var errImagePullManifestUnknownRE = regexp.MustCompile("ErrImagePull.*manifest unknown")
var invalidImagesRE = []*regexp.Regexp{

	// See this test: "should not be able to pull image from invalid registry [NodeConformance]"
	regexp.MustCompile("invalid.com"),

	// See this test: "should not be able to pull from private registry without secret [NodeConformance]"
	regexp.MustCompile("3.7 in gcr.io/authenticated-image-pulling/alpine"),

	// See this test: "deployment should support proportional scaling"
	// and "Updating deployment %q with a non-existent image" where they use webserver:404
	regexp.MustCompile("404 in docker.io/library/webserver"),
}

// namespaceRestriction is an enum for clearly indicating if were only interested in events in openshift- namespaces,
// or those that are not.
type namespaceRestriction int

const (
	InOpenShiftNS    namespaceRestriction = 0
	NotInOpenshiftNS namespaceRestriction = 1
)

func testErrImagePullConnTimeoutOpenShiftNamespaces(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] should not encounter ErrImagePull read connection timeout in openshift namespace pods"
	return buildTestsFailIfRegexMatch(testName, errImagePullTimeoutRE, []*regexp.Regexp{}, InOpenShiftNS, []*regexp.Regexp{}, events)
}

func testErrImagePullConnTimeout(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] should not encounter ErrImagePull read connection timeout in non-openshift namespace pods"
	return buildTestsFailIfRegexMatch(testName, errImagePullTimeoutRE, []*regexp.Regexp{}, NotInOpenshiftNS, []*regexp.Regexp{}, events)
}

func testErrImagePullQPSExceededOpenShiftNamespaces(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] should not encounter ErrImagePull QPS exceeded error in openshift namespace pods"
	return buildTestsFailIfRegexMatch(testName, errImagePullQPSExceededRE, []*regexp.Regexp{}, InOpenShiftNS, invalidImagesRE, events)
}

func testErrImagePullQPSExceeded(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] should not encounter ErrImagePull QPS exceeded error in non-openshift namespace pods"
	return buildTestsFailIfRegexMatch(testName, errImagePullQPSExceededRE, []*regexp.Regexp{}, NotInOpenshiftNS, invalidImagesRE, events)
}

func testErrImagePullManifestUnknownOpenShiftNamespaces(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] should not encounter ErrImagePull manifest unknown error in openshift namespace pods"
	return buildTestsFailIfRegexMatch(testName, errImagePullManifestUnknownRE, []*regexp.Regexp{}, InOpenShiftNS, invalidImagesRE, events)
}

func testErrImagePullManifestUnknown(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] should not encounter ErrImagePull manifest unknown error in non-openshift namespace pods"
	return buildTestsFailIfRegexMatch(testName, errImagePullManifestUnknownRE, []*regexp.Regexp{}, NotInOpenshiftNS, invalidImagesRE, events)
}

func testErrImagePullGenericOpenShiftNamespaces(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] should not encounter ErrImagePull in openshift namespace pods"
	dontMatchREs := []*regexp.Regexp{}
	dontMatchREs = append(dontMatchREs, errImagePullTimeoutRE)
	dontMatchREs = append(dontMatchREs, errImagePullQPSExceededRE)
	dontMatchREs = append(dontMatchREs, errImagePullManifestUnknownRE)
	return buildTestsFailIfRegexMatch(testName, errImagePullGenericRE, dontMatchREs, InOpenShiftNS, invalidImagesRE, events)
}

func testErrImagePullGeneric(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] should not encounter ErrImagePull in non-openshift namespace pods"
	dontMatchREs := []*regexp.Regexp{}
	dontMatchREs = append(dontMatchREs, errImagePullTimeoutRE)
	dontMatchREs = append(dontMatchREs, errImagePullQPSExceededRE)
	dontMatchREs = append(dontMatchREs, errImagePullManifestUnknownRE)
	return buildTestsFailIfRegexMatch(testName, errImagePullGenericRE, dontMatchREs, NotInOpenshiftNS, invalidImagesRE, events)
}

func buildTestsFailIfRegexMatch(testName string, matchRE *regexp.Regexp, dontMatchREs []*regexp.Regexp,
	nsRestriction namespaceRestriction, expectedErrPatterns []*regexp.Regexp, events monitorapi.Intervals) []*junitapi.JUnitTestCase {

	var matchedIntervals monitorapi.Intervals
	for _, event := range events {
		estr := event.String()

		// Skip if the namespace doesn't match what the test was interested in:
		if (nsRestriction == InOpenShiftNS && !strings.Contains(estr, "ns/openshift-")) ||
			(nsRestriction == NotInOpenshiftNS && strings.Contains(estr, "ns/openshift-")) {
			continue
		}

		// Skip if we don't match the search regex:
		if !matchRE.MatchString(estr) {
			continue
		}

		// Skip if we *do* match the list of don't match regex:
		found := false
		for i := 0; i < len(dontMatchREs); i++ {
			if dontMatchREs[i].MatchString(estr) {
				found = true
				break
			}
		}
		if found {
			// Skip those since they might be captured by other tests
			continue
		}

		// Skip if this ErrImagePull problem is part of a negative test (i.e., if any of these
		// patterns match the event string, it is an expected ErrImagePull failure).
		for i := 0; i < len(expectedErrPatterns); i++ {
			if expectedErrPatterns[i].MatchString(estr) {
				found = true
				break
			}
		}
		if found {
			// ErrImagePull was found but also one of the expected error patters was found so don't
			// count this as an ErrImagePull failure.
			continue
		}

		matchedIntervals = append(matchedIntervals, event)
	}

	success := &junitapi.JUnitTestCase{Name: testName}
	if len(matchedIntervals) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	matchedIntervalMsgs := make([]string, 0, len(matchedIntervals))
	for _, ei := range matchedIntervals {
		matchedIntervalMsgs = append(matchedIntervalMsgs, fmt.Sprintf("%s: %s", ei.Locator.OldLocator(), ei.Message.OldMessage()))
	}
	sort.Strings(matchedIntervalMsgs)

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(matchedIntervals.Strings(), "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("Found %d ErrImagePull intervals for: \n\n%s",
				len(matchedIntervalMsgs), strings.Join(matchedIntervalMsgs, "\n")),
		},
	}

	// Always including a flake for now because we're unsure what the results of this test will be. In future
	// we hope to drop this.
	return []*junitapi.JUnitTestCase{failure, success}
}
