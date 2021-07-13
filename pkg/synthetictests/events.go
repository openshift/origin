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

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func testDuplicatedEvents(events monitorapi.Intervals) []*ginkgo.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically"

	knownEvents := combinedRegexp(
		// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should not deadlock when a pod's predecessor fails [Suite:openshift/conformance/parallel] [Suite:k8s]
		// PauseNewPods causes readiness probe to fail.
		regexp.MustCompile(`ns/e2e-statefulset-[0-9]+ pod/ss-[0-9] node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),

		// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should perform rolling updates and roll backs of template modifications [Conformance] [Suite:openshift/conformance/parallel/minimal] [Suite:k8s]
		regexp.MustCompile(`ns/e2e-statefulset-[0-9]+ pod/ss2-[0-9] node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: HTTP probe failed with statuscode: 404`),

		// [sig-node] Probing container should be restarted startup probe fails [Suite:openshift/conformance/parallel] [Suite:k8s]
		regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ pod/startup-[0-9a-f-]+ node/[a-z0-9.-]+ - reason/Unhealthy Startup probe failed: `),

		// [sig-node] Probing container should *not* be restarted with a non-local redirect http liveness probe [Suite:openshift/conformance/parallel] [Suite:k8s]
		regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ pod/liveness-[0-9a-f-]+ node/[a-z0-9.-]+ - reason/ProbeWarning Liveness probe warning: <a href="http://0\.0\.0\.0/">Found</a>\.\n\n`),
	)

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
	failures := []string{}
	for display, count := range displayToCount {
		failures = append(failures, fmt.Sprintf("event happened %d times, something is wrong: %v", count, display))
	}

	// failures during a run always fail the test suite
	var tests []*ginkgo.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &ginkgo.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%d events happened too frequently\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
	}

	if len(tests) == 0 {
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
