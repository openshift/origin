package synthetictests

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo"
)

func combinedRegexp(arr ...*regexp.Regexp) *regexp.Regexp {
	s := ""
	for _, r := range arr {
		if s != "" {
			s += "|"
		}
		s += r.String()
	}
	return regexp.MustCompile(s)
}

var knownEvents = combinedRegexp(
	// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should not deadlock when a pod's predecessor fails [Suite:openshift/conformance/parallel] [Suite:k8s]
	// PauseNewPods intentionally causes readiness probe to fail.
	regexp.MustCompile(`ns/e2e-statefulset-[0-9]+ pod/ss-[0-9] node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),

	// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should perform rolling updates and roll backs of template modifications [Conformance] [Suite:openshift/conformance/parallel/minimal] [Suite:k8s]
	// breakPodHTTPProbe intentionally causes readiness probe to fail.
	regexp.MustCompile(`ns/e2e-statefulset-[0-9]+ pod/ss2-[0-9] node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: HTTP probe failed with statuscode: 404`),

	// [sig-node] Probing container should be restarted startup probe fails [Suite:openshift/conformance/parallel] [Suite:k8s]
	regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ pod/startup-[0-9a-f-]+ node/[a-z0-9.-]+ - reason/Unhealthy Startup probe failed: `),

	// [sig-node] Probing container should *not* be restarted with a non-local redirect http liveness probe [Suite:openshift/conformance/parallel] [Suite:k8s]
	regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ pod/liveness-[0-9a-f-]+ node/[a-z0-9.-]+ - reason/ProbeWarning Liveness probe warning: <a href="http://0\.0\.0\.0/">Found</a>\.\n\n`),

	// Operators that use library-go can report about multiple versions during upgrades.
	regexp.MustCompile(`ns/openshift-etcd-operator deployment/etcd-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-apiserver-operator deployment/kube-apiserver-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-controller-manager-operator deployment/kube-controller-manager-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-scheduler-operator deployment/openshift-kube-scheduler-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),

	// etcd-quorum-guard can fail during upgrades.
	regexp.MustCompile(`ns/openshift-etcd pod/etcd-quorum-guard-[a-z0-9-]+ node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),
)

var knownEventProblems = []struct {
	Regexp *regexp.Regexp
	BZ     string
}{
	{
		Regexp: regexp.MustCompile(`ns/openshift-sdn pod/sdn-[a-z0-9]+ node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: command timed out`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1978268",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-multus pod/network-metrics-daemon-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-network-diagnostics pod/network-check-target-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-machine-api machine/[a-z0-9.-]+ - reason/Updated Updated machine "[a-z0-9.-]+"`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1988992",
	},
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func testDuplicatedEvents(events monitorapi.Intervals) []*ginkgo.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically"

	displayToCount := map[string]int{}
	for _, event := range events {
		eventDisplayMessage, times := getTimesAnEventHappened(fmt.Sprintf("%s - %s", event.Locator, event.Message))
		if times > 20 {
			if knownEvents.MatchString(eventDisplayMessage) {
				continue
			}
			displayToCount[eventDisplayMessage] = times
		}
	}

	var failures []string
	var flakes []string
	for display, count := range displayToCount {
		msg := fmt.Sprintf("event happened %d times, something is wrong: %v", count, display)

		flake := false
		for _, kp := range knownEventProblems {
			if kp.Regexp.MatchString(display) {
				msg += " - " + kp.BZ
				flake = true
			}
		}

		if flake {
			flakes = append(flakes, msg)
		} else {
			failures = append(failures, msg)
		}
	}

	// failures during a run always fail the test suite
	var tests []*ginkgo.JUnitTestCase
	if len(failures) > 0 || len(flakes) > 0 {
		var output string
		if len(failures) > 0 {
			output = fmt.Sprintf("%d events happened too frequently\n\n%v", len(failures), strings.Join(failures, "\n"))
		}
		if len(flakes) > 0 {
			if output != "" {
				output += "\n\n"
			}
			output += fmt.Sprintf("%d events with known BZs\n\n%v", len(flakes), strings.Join(flakes, "\n"))
		}
		tests = append(tests, &ginkgo.JUnitTestCase{
			Name: testName,
			FailureOutput: &ginkgo.FailureOutput{
				Output: output,
			},
		})
	}

	if len(tests) == 0 || len(failures) == 0 {
		// Add a successful result to mark the test as flaky if there are no
		// unknown problems.
		tests = append(tests, &ginkgo.JUnitTestCase{Name: testName})
	}
	return tests
}

var eventCountExtractor = regexp.MustCompile(`(?s)(.*) \((\d+) times\).*`)

func getTimesAnEventHappened(message string) (string, int) {
	matches := eventCountExtractor.FindAllStringSubmatch(message, -1)
	if len(matches) != 1 { // not present or weird
		return "", 0
	}
	if len(matches[0]) < 2 { // no capture
		return "", 0
	}
	times, err := strconv.ParseInt(matches[0][2], 10, 0)
	if err != nil { // not an int somehow
		return "", 0
	}
	return matches[0][1], int(times)
}
