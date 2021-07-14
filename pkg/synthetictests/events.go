package synthetictests

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo"
)

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

var eventCountExtractor = regexp.MustCompile(`(.*)\((\d+) times\).*`)

func getTimesAnEventHappened(message string) (string, int) {
	matches := eventCountExtractor.FindAllStringSubmatch(message, -1)
	if len(matches) != 1 { // not present or weird
		return "", 0
	}
	if len(matches[0]) < 2 { // no capture
		return "", 0
	}
	times, err := strconv.ParseInt(matches[0][2], 10, 32)
	if err != nil { // not an int somehow
		return "", 0
	}
	return matches[0][1], int(times)
}
