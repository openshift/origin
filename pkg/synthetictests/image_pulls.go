package synthetictests

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

const (
	imagePullRedhatRegEx          = `reason/[a-zA-Z]+ .*Back-off pulling image .*registry.redhat.io`
	imagePullRedhatFlakeThreshold = 5
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
// if a match is found, and occurs more than s.failThreshold times, we mark it as a
// flake (this allows us to see how often this symptom is happening).
//
func (s *singleEventCheckRegex) test(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	success := &junitapi.JUnitTestCase{Name: s.testName}
	var failureOutput, flakeOutput []string
	for _, e := range events {
		if s.recognizer(e) {

			msg := fmt.Sprintf("%s - %s", e.Locator, e.Message)
			eventDisplayMessage, times := getTimesAnEventHappened(msg)
			switch {
			case times > s.failThreshold:
				failureOutput = append(failureOutput, fmt.Sprintf("event [%s] happened %d times", eventDisplayMessage, times))
			case times > s.flakeThreshold:
				flakeOutput = append(flakeOutput, fmt.Sprintf("event [%s] happened %d times", eventDisplayMessage, times))
			}
		}
	}
	if len(failureOutput) > 0 {
		totalOutput := failureOutput
		if len(flakeOutput) >= 0 {
			totalOutput = append(totalOutput, flakeOutput...)
		}
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

func newSingleEventCheckRegex(testName, regex string) *singleEventCheckRegex {
	return &singleEventCheckRegex{
		testName:       testName,
		recognizer:     matchEventForRegexOrDie(regex),
		failThreshold:  math.MaxInt32,
		flakeThreshold: imagePullRedhatFlakeThreshold,
	}
}

// testBackoffPullingRegistryRedhatImage looks for this symptom:
//   reason/ContainerWait ... Back-off pulling image "registry.redhat.io/openshift4/ose-oauth-proxy:latest"
//   reason/BackOff Back-off pulling image "registry.redhat.io/openshift4/ose-oauth-proxy:latest"
// to happen over a certain threshold and marks it as a failure or flake accordingly.
//
func testBackoffPullingRegistryRedhatImage(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[sig-arch] should not see excessive pull back-off on registry.redhat.io"
	return newSingleEventCheckRegex(testName, imagePullRedhatRegEx).test(events)
}
