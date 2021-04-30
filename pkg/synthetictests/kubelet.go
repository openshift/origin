package synthetictests

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"github.com/openshift/origin/pkg/test/ginkgo"
	"k8s.io/apimachinery/pkg/util/sets"
)

func testKubeletToAPIServerGracefulTermination(events monitorapi.Intervals) []*ginkgo.JUnitTestCase {
	const testName = "[sig-node] kubelet terminates kube-apiserver gracefully"

	var failures []string
	for _, event := range events {
		if strings.Contains(event.Message, "did not terminate gracefully") || strings.Contains(event.Message, "reason/NonGracefulTermination") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
		}
	}

	// failures during a run always fail the test suite
	var tests []*ginkgo.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &ginkgo.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%d kube-apiserver reports a non-graceful termination.  Probably kubelet or CRI-O is not giving the time to cleanly shut down. This can lead to connection refused and network I/O timeout errors in other components.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})

		// while waiting for https://bugzilla.redhat.com/show_bug.cgi?id=1928946 mark as flake
		tests[0].FailureOutput.Output = "Marked flake while fix for https://bugzilla.redhat.com/show_bug.cgi?id=1928946 is identified:\n\n" + tests[0].FailureOutput.Output
		tests = append(tests, &ginkgo.JUnitTestCase{Name: testName})
	}

	if len(tests) == 0 {
		tests = append(tests, &ginkgo.JUnitTestCase{Name: testName})
	}
	return tests
}

func testKubeAPIServerGracefulTermination(events monitorapi.Intervals) []*ginkgo.JUnitTestCase {
	const testName = "[sig-api-machinery] kube-apiserver terminates within graceful termination period"

	var failures []string
	for _, event := range events {
		if strings.Contains(event.Message, "reason/GracefulTerminationTimeout") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
		}
	}

	// failures during a run always fail the test suite
	var tests []*ginkgo.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &ginkgo.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%d kube-apiserver reports a non-graceful termination. This is a bug in kube-apiserver. It probably means that network connections are not closed cleanly, and this leads to network I/O timeout errors in other components.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
	}

	if len(tests) == 0 {
		tests = append(tests, &ginkgo.JUnitTestCase{Name: testName})
	}
	return tests
}

func testContainerFailures(events monitorapi.Intervals) []*ginkgo.JUnitTestCase {
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

	var testCases []*ginkgo.JUnitTestCase

	const failToStartTestName = "[sig-architecture] platform pods should not fail to start"
	if len(failures) > 0 {
		testCases = append(testCases, &ginkgo.JUnitTestCase{
			Name:      failToStartTestName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%d container starts had issues\n\n%s", len(failures), strings.Join(failures, "\n")),
			},
		})
	}
	// mark flaky for now while we debug
	testCases = append(testCases, &ginkgo.JUnitTestCase{Name: failToStartTestName})

	const excessiveRestartTestName = "[sig-architecture] platform pods should not exit more than once with a non-zero exit code"
	if len(excessiveExits) > 0 {
		testCases = append(testCases, &ginkgo.JUnitTestCase{
			Name:      excessiveRestartTestName,
			SystemOut: strings.Join(excessiveExits, "\n"),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%d containers with multiple restarts\n\n%s", len(excessiveExits), strings.Join(excessiveExits, "\n\n")),
			},
		})
	}
	// mark flaky for now while we debug
	testCases = append(testCases, &ginkgo.JUnitTestCase{Name: excessiveRestartTestName})

	return testCases
}

func testDeleteGracePeriodZero(events monitorapi.Intervals) []*ginkgo.JUnitTestCase {
	const testName = "[sig-architecture] platform pods should not be force deleted with gracePeriod 0"
	success := &ginkgo.JUnitTestCase{Name: testName}

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
		return []*ginkgo.JUnitTestCase{success}
	}

	failure := &ginkgo.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &ginkgo.FailureOutput{
			Output: fmt.Sprintf("The following pods were force deleted and should not be:\n\n%s", strings.Join(failures, "\n")),
		},
	}
	// TODO: marked flaky until has been thoroughly debugged
	return []*ginkgo.JUnitTestCase{failure, success}
}

func testPodTransitions(events monitorapi.Intervals) []*ginkgo.JUnitTestCase {
	const testName = "[sig-node] pods should never transition back to pending"
	success := &ginkgo.JUnitTestCase{Name: testName}

	failures := []string{}
	for _, event := range events {
		if strings.Contains(event.Message, "pod should not transition") || strings.Contains(event.Message, "pod moved back to Pending") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
		}
	}
	if len(failures) == 0 {
		return []*ginkgo.JUnitTestCase{success}
	}

	failure := &ginkgo.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &ginkgo.FailureOutput{
			Output: fmt.Sprintf("Marked as flake until https://bugzilla.redhat.com/show_bug.cgi?id=1933760 is fixed\n\n%d pods illegally transitioned to Pending\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}
	// TODO: temporarily marked flaky since it is continously failing
	return []*ginkgo.JUnitTestCase{failure, success}
}

func formatTimes(times []time.Time) []string {
	var s []string
	for _, t := range times {
		s = append(s, t.UTC().Format(time.RFC3339))
	}
	return s
}

func testNodeUpgradeTransitions(events monitorapi.Intervals) []*ginkgo.JUnitTestCase {
	const testName = "[sig-node] nodes should not go unready after being upgraded and go unready only once"

	var buf bytes.Buffer
	var testCases []*ginkgo.JUnitTestCase
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
			if !monitorapi.IsNode(event.Locator) {
				continue
			}
			if !strings.HasPrefix(event.Message, "condition/Ready ") || !strings.HasSuffix(event.Message, " changed") {
				continue
			}
			node, _ := monitorapi.NodeFromLocator(event.Locator)
			if strings.Contains(event.Message, " status/True ") {
				if currentNodeReady[node] {
					failures = append(failures, fmt.Sprintf("Node %s was reported ready twice in a row, this should be impossible", node))
					continue
				}
				currentNodeReady[node] = true
				nodesWentReady[node] = append(nodesWentReady[node], event.From)
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

		testCases = append(testCases, &ginkgo.JUnitTestCase{
			Name:      testName,
			SystemOut: fmt.Sprintf("%s\n\n%s", strings.Join(failures, "\n"), buf.String()),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%d nodes violated upgrade expectations:\n\n%s\n\n%s", abnormalNodes.Len(), strings.Join(failures, "\n"), text),
			},
		})
	}
	if len(testCases) == 0 {
		testCases = append(testCases, &ginkgo.JUnitTestCase{Name: testName})
	}
	return testCases
}

func testSystemDTimeout(events monitorapi.Intervals) []*ginkgo.JUnitTestCase {
	const testName = "[sig-node] pods should not fail on systemd timeouts"
	success := &ginkgo.JUnitTestCase{Name: testName}

	failures := []string{}
	for _, event := range events {
		if strings.Contains(event.Message, "systemd timed out for pod") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
		}
	}
	if len(failures) == 0 {
		return []*ginkgo.JUnitTestCase{success}
	}

	failure := &ginkgo.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &ginkgo.FailureOutput{
			Output: fmt.Sprintf("%d systemd timed out for pod occurrences\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}

	// write a passing test to trigger detection of this issue as a flake. Doing this first to try to see how frequent the issue actually is
	return []*ginkgo.JUnitTestCase{failure, success}
}
