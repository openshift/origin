package synthetictests

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/duplicateevents"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type eventRecognizerFunc func(event monitorapi.EventInterval) bool

func matchEventForRegexOrDie(regex string) eventRecognizerFunc {
	regExp := regexp.MustCompile(regex)
	return func(e monitorapi.EventInterval) bool {
		return regExp.MatchString(e.Message)
	}
}

type singleEventCheckRegex struct {
	testName       string
	recognizer     eventRecognizerFunc
	failThreshold  int
	flakeThreshold int
}

// test goes through the events, looks for a match using the s.recognizer function,
// if a match is found, marks it as failure or flake depending on if the pattern occurs
// above the fail/flake thresholds (this allows us to track the occurence as a specific
// test. If the fail threshold is set to -1, the test will only flake.
func (s *singleEventCheckRegex) test(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	success := &junitapi.JUnitTestCase{Name: s.testName}
	var failureOutput, flakeOutput []string
	for _, e := range events {
		if s.recognizer(e) {

			msg := fmt.Sprintf("%s - %s", e.Locator, e.Message)
			eventDisplayMessage, times := getTimesAnEventHappened(msg)
			switch {
			case s.failThreshold > 0 && times > s.failThreshold:
				failureOutput = append(failureOutput, fmt.Sprintf("event [%s] happened %d times", eventDisplayMessage, times))
			case times > s.flakeThreshold:
				flakeOutput = append(flakeOutput, fmt.Sprintf("event [%s] happened %d times", eventDisplayMessage, times))
			}
		}
	}
	if len(failureOutput) > 0 {
		totalOutput := failureOutput
		failure := &junitapi.JUnitTestCase{
			Name:      s.testName,
			SystemOut: strings.Join(totalOutput, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: strings.Join(totalOutput, "\n"),
			},
		}

		return []*junitapi.JUnitTestCase{failure}
	}
	if len(flakeOutput) > 0 {
		failure := &junitapi.JUnitTestCase{
			Name:      s.testName,
			SystemOut: strings.Join(flakeOutput, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: strings.Join(flakeOutput, "\n"),
			},
		}
		return []*junitapi.JUnitTestCase{failure, success}
	}

	return []*junitapi.JUnitTestCase{success}
}

func newSingleEventCheckRegex(testName, regex string, failThreshold, flakeThreshold int) *singleEventCheckRegex {
	return &singleEventCheckRegex{
		testName:       testName,
		recognizer:     matchEventForRegexOrDie(regex),
		failThreshold:  failThreshold,
		flakeThreshold: flakeThreshold,
	}
}

// testBackoffPullingRegistryRedhatImage looks for this symptom:
//
//	reason/ContainerWait ... Back-off pulling image "registry.redhat.io/openshift4/ose-oauth-proxy:latest"
//	reason/BackOff Back-off pulling image "registry.redhat.io/openshift4/ose-oauth-proxy:latest"
//
// to happen over a certain threshold and marks it as a failure or flake accordingly.
func testBackoffPullingRegistryRedhatImage(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[sig-arch] should not see excessive pull back-off on registry.redhat.io"
	return newSingleEventCheckRegex(testName, duplicateevents.ImagePullRedhatRegEx, math.MaxInt, duplicateevents.ImagePullRedhatFlakeThreshold).test(events)
}

// testRequiredInstallerResourcesMissing looks for this symptom:
//
//	reason/RequiredInstallerResourcesMissing secrets: etcd-all-certs-3
//
// and fails if it happens more than the failure threshold count of 20 and flakes more than the
// flake threshold.  See https://bugzilla.redhat.com/show_bug.cgi?id=2031564.
func testRequiredInstallerResourcesMissing(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[bz-etcd] should not see excessive RequiredInstallerResourcesMissing secrets"
	return newSingleEventCheckRegex(testName, duplicateevents.RequiredResourcesMissingRegEx, duplicateevents.DuplicateEventThreshold, duplicateevents.RequiredResourceMissingFlakeThreshold).test(events)
}

// testBackoffStartingFailedContainer looks for this symptom in core namespaces:
//
//	reason/BackOff Back-off restarting failed container
func testBackoffStartingFailedContainer(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[sig-cluster-lifecycle] should not see excessive Back-off restarting failed containers"

	return newSingleEventCheckRegex(testName, duplicateevents.BackoffRestartingFailedRegEx, duplicateevents.DuplicateEventThreshold, duplicateevents.BackoffRestartingFlakeThreshold).
		test(events.Filter(monitorapi.Not(monitorapi.IsInE2ENamespace)))
}

// testBackoffStartingFailedContainerForE2ENamespaces looks for this symptom in e2e namespaces:
//
//	reason/BackOff Back-off restarting failed container
func testBackoffStartingFailedContainerForE2ENamespaces(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[sig-cluster-lifecycle] should not see excessive Back-off restarting failed containers in e2e namespaces"

	// always flake for now
	return newSingleEventCheckRegex(testName, duplicateevents.BackoffRestartingFailedRegEx, math.MaxInt, duplicateevents.BackoffRestartingFlakeThreshold).
		test(events.Filter(monitorapi.IsInE2ENamespace))
}

func testErrorUpdatingEndpointSlices(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[sig-networking] should not see excessive FailedToUpdateEndpointSlices Error updating Endpoint Slices"

	return newSingleEventCheckRegex(testName, duplicateevents.ErrorUpdatingEndpointSlicesRegex, duplicateevents.ErrorUpdatingEndpointSlicesFailedThreshold, duplicateevents.ErrorUpdatingEndpointSlicesFlakeThreshold).
		test(events.Filter(monitorapi.IsInNamespaces(sets.NewString("openshift-ovn-kubernetes"))))
}

func testConfigOperatorProbeErrorReadinessProbe(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] openshift-config-operator should not get probe error on readiness probe due to timeout"
	return makeProbeTest(testName, events, "openshift-config-operator", duplicateevents.ProbeErrorReadinessMessageRegExpStr, duplicateevents.DuplicateEventThreshold)
}

func testConfigOperatorProbeErrorLivenessProbe(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] openshift-config-operator should not get probe error on liveness probe due to timeout"
	return makeProbeTest(testName, events, "openshift-config-operator", duplicateevents.ProbeErrorLivenessMessageRegExpStr, duplicateevents.DuplicateEventThreshold)
}

func testConfigOperatorReadinessProbe(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] openshift-config-operator readiness probe should not fail due to timeout"
	return makeProbeTest(testName, events, "openshift-config-operator", duplicateevents.ReadinessFailedMessageRegExpStr, duplicateevents.DuplicateEventThreshold)
}

func testNodeHasNoDiskPressure(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] Test the NodeHasNoDiskPressure condition does not occur too often"
	return eventExprMatchThresholdTest(testName, events, duplicateevents.NodeHasNoDiskPressureRegExpStr, duplicateevents.DuplicateEventThreshold)
}

func testNodeHasSufficientMemory(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] Test the NodeHasSufficeintMemory condition does not occur too often"
	return eventExprMatchThresholdTest(testName, events, duplicateevents.NodeHasSufficientMemoryRegExpStr, duplicateevents.DuplicateEventThreshold)
}

func testNodeHasSufficientPID(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] Test the NodeHasSufficientPID condition does not occur too often"
	return eventExprMatchThresholdTest(testName, events, duplicateevents.NodeHasSufficientPIDRegExpStr, duplicateevents.DuplicateEventThreshold)
}

func makeProbeTest(testName string, events monitorapi.Intervals, operatorName string, regExStr string, eventFlakeThreshold int) []*junitapi.JUnitTestCase {
	messageRegExp := regexp.MustCompile(regExStr)
	return eventMatchThresholdTest(testName, events, func(event monitorapi.EventInterval) bool {
		return duplicateevents.IsOperatorMatchRegexMessage(event, operatorName, messageRegExp)
	}, eventFlakeThreshold)
}

func eventExprMatchThresholdTest(testName string, events monitorapi.Intervals, regExStr string, eventFlakeThreshold int) []*junitapi.JUnitTestCase {
	messageRegExp := regexp.MustCompile(regExStr)
	return eventMatchThresholdTest(testName, events, func(event monitorapi.EventInterval) bool { return messageRegExp.MatchString(event.Message) }, eventFlakeThreshold)
}

func eventMatchThresholdTest(testName string, events monitorapi.Intervals, eventMatch eventRecognizerFunc, eventFlakeThreshold int) []*junitapi.JUnitTestCase {
	var maxFailureOutput string
	maxTimes := 0
	for _, event := range events {
		if eventMatch(event) {
			// Place the failure time in the message to avoid having to extract the time from the events json file
			// (in artifacts) when viewing the test failure output.
			failureOutput := fmt.Sprintf("%s %s\n", event.From.UTC().Format("15:04:05"), event.Message)

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
