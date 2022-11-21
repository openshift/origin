package synthetictests

import (
	"fmt"
	"regexp"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

const (
	nodeHasNoDiskPressureRegExpStr   = "reason/NodeHasNoDiskPressure.*status is now: NodeHasNoDiskPressure"
	nodeHasSufficientMemoryRegExpStr = "reason/NodeHasSufficientMemory.*status is now: NodeHasSufficientMemory"
	nodeHasSufficientPIDRegExpStr    = "reason/NodeHasSufficientPID.*status is now: NodeHasSufficientPID"
)

func makeNodeHasTest(testName string, events monitorapi.Intervals, regExStr string, eventFlakeThreshold int) []*junitapi.JUnitTestCase {
	messageRegExp := regexp.MustCompile(regExStr)
	var maxFailureOutput string
	maxTimes := 0
	for _, event := range events {
		if messageRegExp.MatchString(event.Message) {
			// Place the failure time in the message to avoid having to extract the time from the events json file
			// (in artifacts) when viewing the test failure output.
			failureOutput := fmt.Sprintf("%s %s\n", event.From.Format("15:04:05"), event.Message)

			_, times := getTimesAnEventHappened(fmt.Sprintf("%s - %s", event.Locator, event.Message))

			// find the largest grouping of these events
			if times > maxTimes {
				maxTimes = times
				maxFailureOutput = failureOutput
			}
		}
	}

	test := &junitapi.JUnitTestCase{Name: testName}

	if maxTimes < eventFlakeThreshold {
		return []*junitapi.JUnitTestCase{test}
	}

	// Flake for now.
	test.FailureOutput = &junitapi.FailureOutput{
		Output: maxFailureOutput,
	}
	success := &junitapi.JUnitTestCase{Name: testName}
	return []*junitapi.JUnitTestCase{test, success}
}
func testNodeHasNoDiskPressure(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] Test the NodeHasNoDiskPressure condition does not occur too often"
	return makeNodeHasTest(testName, events, nodeHasNoDiskPressureRegExpStr, duplicateEventThreshold)
}

func testNodeHasSufficientMemory(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] Test the NodeHasSufficeintMemory condition does not occur too often"
	return makeNodeHasTest(testName, events, nodeHasSufficientMemoryRegExpStr, duplicateEventThreshold)
}

func testNodeHasSufficientPID(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] Test the NodeHasSufficientPID condition does not occur too often"
	return makeNodeHasTest(testName, events, nodeHasSufficientPIDRegExpStr, duplicateEventThreshold)
}
