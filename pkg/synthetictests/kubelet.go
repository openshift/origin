package synthetictests

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
)

const (
	probeTimeoutEventThreshold   = 5
	probeTimeoutMessageRegExpStr = "reason/ReadinessFailed.*Get.*healthz.*net/http.*request canceled while waiting for connection.*Client.Timeout exceeded"
)

func testKubeletToAPIServerGracefulTermination(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] kubelet terminates kube-apiserver gracefully"

	var failures []string
	for _, event := range events {
		if strings.Contains(event.Message, "did not terminate gracefully") || strings.Contains(event.Message, "reason/NonGracefulTermination") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
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

func testKubeAPIServerGracefulTermination(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-api-machinery] kube-apiserver terminates within graceful termination period"

	var failures []string
	for _, event := range events {
		if strings.Contains(event.Message, "reason/GracefulTerminationTimeout") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
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

func testContainerFailures(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	containerExits := make(map[string][]string)
	failures := []string{}
	for _, event := range events {
		if !strings.Contains(event.Locator, "ns/openshift-") {
			continue
		}
		switch {
		// errors during container start should be highlighted because they are unexpected
		case strings.Contains(event.Message, "reason/ContainerWait "):
			// excluded https://bugzilla.redhat.com/show_bug.cgi?id=1933760
			if strings.Contains(event.Message, "possible container status clear") || strings.Contains(event.Message, "cause/ContainerCreating ") {
				continue
			}
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))

		// workload containers should never exit non-zero during normal operations
		case strings.Contains(event.Message, "reason/ContainerExit") && !strings.Contains(event.Message, "code/0"):
			containerExits[event.Locator] = append(containerExits[event.Locator], event.Message)
		}
	}

	var excessiveExits []string
	for locator, messages := range containerExits {
		if len(messages) > 1 {
			messageSet := sets.NewString(messages...)
			excessiveExits = append(excessiveExits, fmt.Sprintf("%s restarted %d times:\n%s", locator, len(messages), strings.Join(messageSet.List(), "\n")))
		}
	}
	sort.Strings(excessiveExits)

	var testCases []*junitapi.JUnitTestCase

	const failToStartTestName = "[sig-architecture] platform pods should not fail to start"
	if len(failures) > 0 {
		testCases = append(testCases, &junitapi.JUnitTestCase{
			Name:      failToStartTestName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%d container starts had issues\n\n%s", len(failures), strings.Join(failures, "\n")),
			},
		})
	}
	// mark flaky for now while we debug
	testCases = append(testCases, &junitapi.JUnitTestCase{Name: failToStartTestName})

	const excessiveRestartTestName = "[sig-architecture] platform pods should not exit more than once with a non-zero exit code"
	if len(excessiveExits) > 0 {
		testCases = append(testCases, &junitapi.JUnitTestCase{
			Name:      excessiveRestartTestName,
			SystemOut: strings.Join(excessiveExits, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%d containers with multiple restarts\n\n%s", len(excessiveExits), strings.Join(excessiveExits, "\n\n")),
			},
		})
	}
	// mark flaky for now while we debug
	testCases = append(testCases, &junitapi.JUnitTestCase{Name: excessiveRestartTestName})

	return testCases
}

func testConfigOperatorReadinessProbe(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] openshift-config-operator readiness probe should not fail due to timeout"
	messageRegExp := regexp.MustCompile(probeTimeoutMessageRegExpStr)
	var failureOutput string
	var count int
	for _, event := range events {
		if isConfigOperatorReadinessProbeFailedMessage(event, messageRegExp) {
			// Place the failure time in the message to avoid having to extract the time from the events json file
			// (in `artifacts) when viewing the test failure output.
			failureOutput += fmt.Sprintf("%s %s\n", event.From.Format("15:04:05"), event.Message)
			count++
		}
	}
	test := &junitapi.JUnitTestCase{Name: testName}
	if count > probeTimeoutEventThreshold {
		// Flake for now.
		test.FailureOutput = &junitapi.FailureOutput{
			Output: failureOutput,
		}
		success := &junitapi.JUnitTestCase{Name: testName}
		return []*junitapi.JUnitTestCase{test, success}
	}
	return []*junitapi.JUnitTestCase{test}
}

func testKubeApiserverProcessOverlap(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] overlapping apiserver process detected during kube-apiserver rollout"
	success := &junitapi.JUnitTestCase{Name: testName}
	failures := []string{}
	for _, event := range events {
		if strings.Contains(event.Message, "reason/TerminationProcessOverlapDetected") {
			failures = append(failures, fmt.Sprintf("[%s - %s] %s", event.From, event.To, event.Locator))
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
		if !strings.Contains(event.Message, "reason/ForceDelete") {
			continue
		}
		if !strings.Contains(event.Locator, "ns/openshift-") {
			continue
		}
		if strings.Contains(event.Message, "mirrored/true") {
			continue
		}
		if strings.Contains(event.Message, "node/ ") {
			continue
		}
		failures = append(failures, event.Locator)
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
		if strings.Contains(event.Message, "pod should not transition") || strings.Contains(event.Message, "pod moved back to Pending") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
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
	// TODO: temporarily marked flaky since it is continously failing
	return []*junitapi.JUnitTestCase{failure, success}
}

func formatTimes(times []time.Time) []string {
	var s []string
	for _, t := range times {
		s = append(s, t.UTC().Format(time.RFC3339))
	}
	return s
}

func testNodeUpgradeTransitions(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
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
			if strings.HasSuffix(event.Locator, "clusterversion/cluster") && (strings.Contains(event.Message, "reason/UpgradeStarted") || strings.Contains(event.Message, "reason/UpgradeRollback")) {
				text = event.Message
				events = events[i+1:]
				foundEnd = true
				fmt.Fprintf(&buf, "DEBUG: found upgrade start event: %v\n", event.String())
				break
			}
			node, isNode := monitorapi.NodeFromLocator(event.Locator)
			if !isNode {
				continue
			}
			if !strings.HasPrefix(event.Message, "condition/Ready ") || !strings.HasSuffix(event.Message, " changed") {
				continue
			}
			if strings.Contains(event.Message, " status/True ") {
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
		if strings.Contains(event.Message, "systemd timed out for pod") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
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
	return buildTestsFailIfRegexMatch(testName, errImagePullTimeoutRE, nil, InOpenShiftNS, []*regexp.Regexp{}, events)
}

func testErrImagePullConnTimeout(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] should not encounter ErrImagePull read connection timeout in non-openshift namespace pods"
	return buildTestsFailIfRegexMatch(testName, errImagePullTimeoutRE, nil, NotInOpenshiftNS, []*regexp.Regexp{}, events)
}

func testErrImagePullGenericOpenShiftNamespaces(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] should not encounter ErrImagePull in openshift namespace pods"
	return buildTestsFailIfRegexMatch(testName, errImagePullGenericRE, errImagePullTimeoutRE, InOpenShiftNS, invalidImagesRE, events)
}

func testErrImagePullGeneric(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] should not encounter ErrImagePull in non-openshift namespace pods"
	return buildTestsFailIfRegexMatch(testName, errImagePullGenericRE, errImagePullTimeoutRE, NotInOpenshiftNS, invalidImagesRE, events)
}

func buildTestsFailIfRegexMatch(testName string, matchRE, dontMatchRE *regexp.Regexp,
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

		// Skip if we *do* match the don't match regex:
		if dontMatchRE != nil && dontMatchRE.MatchString(estr) {
			continue
		}

		// Skip if this ErrImagePull problem is part of a negative test (i.e., if any of these
		// patterns match the event string, it is an expected ErrImagePull failure).
		found := false
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
		matchedIntervalMsgs = append(matchedIntervalMsgs, fmt.Sprintf("%s: %s", ei.Locator, ei.Message))
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
