package synthetictests

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/test/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/sets"
)

func testKubeAPIServerGracefulTermination(events []*monitor.EventInterval) []*ginkgo.JUnitTestCase {
	const testName = "[sig-node] kubelet terminates kube-apiserver gracefully"
	const testNamePre = "[sig-node] kubelet terminates kube-apiserver gracefully before suite execution"

	ignoreEventsBefore := exutil.LimitTestsToStartTime()
	// suiteStartTime can be inferred from the monitor interval, we can't use exutil.SuiteStartTime because that
	// is run within sub processes
	var suiteStartTime time.Time
	if len(events) > 0 {
		suiteStartTime = events[0].From.Add((-time.Second))
	}

	var preFailures, failures []string
	for _, event := range events {
		// from https://github.com/openshift/kubernetes/blob/1f35e4f63be8fbb19e22c9ff1df31048f6b42ddf/cmd/watch-termination/main.go#L96
		if strings.Contains(event.Message, "did not terminate gracefully") {
			if event.From.Before(ignoreEventsBefore) {
				// we explicitly ignore graceful terminations when we are told to check up to a certain point
				continue
			}
			if event.From.Before(suiteStartTime) {
				preFailures = append(preFailures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
			} else {
				failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
			}
		}
	}

	// failures during a run always fail the test suite
	var tests []*ginkgo.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &ginkgo.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%d kube-apiserver reports a non-graceful termination. Probably kubelet or CRI-O is not giving the time to cleanly shut down. This can lead to connection refused and network I/O timeout errors in other components.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
	} else {
		tests = append(tests, &ginkgo.JUnitTestCase{Name: testName})
	}

	// pre-failures are flakes for now
	if len(preFailures) > 0 {
		tests = append(tests,
			&ginkgo.JUnitTestCase{Name: testNamePre},
			&ginkgo.JUnitTestCase{
				Name:      testNamePre,
				SystemOut: strings.Join(preFailures, "\n"),
				FailureOutput: &ginkgo.FailureOutput{
					Output: fmt.Sprintf("%d kube-apiserver reports a non-graceful termination outside of the test suite (during install or upgrade), which is reported as a flake. Probably kubelet or CRI-O is not giving the time to cleanly shut down. This can lead to connection refused and network I/O timeout errors in other components.\n\n%v", len(preFailures), strings.Join(preFailures, "\n")),
				},
			},
		)
	} else {
		tests = append(tests, &ginkgo.JUnitTestCase{Name: testNamePre})
	}
	return tests
}

func testPodTransitions(events []*monitor.EventInterval) []*ginkgo.JUnitTestCase {
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
			Output: fmt.Sprintf("%d pods illegally transitioned to Pending\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}

	// TODO an upgrade job that starts before 4.6 may need to make this test flake instead of fail.  This will depend on which `openshift-tests`
	//  is used to run that upgrade test.  I recommend waiting to a flake until we know and even then find a way to constrain it.
	return []*ginkgo.JUnitTestCase{failure}
}

func formatTimes(times []time.Time) []string {
	var s []string
	for _, t := range times {
		s = append(s, t.UTC().Format(time.RFC3339))
	}
	return s
}

func testNodeUpgradeTransitions(events []*monitor.EventInterval) ([]*ginkgo.JUnitTestCase, bool) {
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
			if !strings.HasPrefix(event.Locator, "node/") {
				continue
			}
			if !strings.HasPrefix(event.Message, "condition/Ready ") || !strings.HasSuffix(event.Message, " changed") {
				continue
			}
			node := strings.TrimPrefix(event.Locator, "node/")
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
		return testCases, true
	}
	return testCases, false
}

func testSystemDTimeout(events []*monitor.EventInterval) []*ginkgo.JUnitTestCase {
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
