package legacykubeapiservermonitortests

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

// amount of time after E2E tests start that we consider "early"
const preE2ECheckDuration = 3 * time.Minute
const earlyE2ECheckDuration = 3 * time.Minute

func testEarlyE2EAPIServerDisruption(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	var e2eStart time.Time // anchor the time period we're interested in disruption
	for _, event := range events {
		if event.Source == monitorapi.SourceE2ETest {
			e2eStart = event.From
			break
		}
	}

	const preTestName = "[Jira:\"oauth-apiserver\"] oauth servers should not experience disruption shortly before the start of E2E testing"
	const earlyTestName = "[Jira:\"kube-apiserver\"] API servers should not experience disruption during the start of E2E testing"
	if e2eStart.IsZero() {
		return []*junitapi.JUnitTestCase{
			{
				Name: preTestName,
				SkipMessage: &junitapi.SkipMessage{
					Message: "no E2E tests ran", // no point in this test
				},
			},
			{
				Name: earlyTestName,
				SkipMessage: &junitapi.SkipMessage{
					Message: "no E2E tests ran", // no point in this test
				},
			},
		}
	}

	junits := []*junitapi.JUnitTestCase{}

	// find the oauth disruption described in OCPBUGS-39021
	eventsFound := findDisruptionEvents(events, e2eStart.Add(-preE2ECheckDuration), e2eStart, regexp.MustCompile(`^(cache-)?oauth-api-`))
	if count := len(eventsFound); count > 0 {
		junits = append(junits, &junitapi.JUnitTestCase{
			Name: preTestName,
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("found %d oauth disruption events shortly before E2E test start (%s) with messages:\n%s",
					count, e2eStart.Format(monitorapi.TimeFormat), strings.Join(eventsFound, "\n")),
			},
		})
	}
	junits = append(junits, &junitapi.JUnitTestCase{Name: preTestName}) // success after fail makes a flake, to record when this is happening

	// find the api disruption described in TRT-1794
	eventsFound = findDisruptionEvents(events, e2eStart, e2eStart.Add(earlyE2ECheckDuration), regexp.MustCompile(`^(cache-)?(openshift|kube|oauth)-api-`))
	if count := len(eventsFound); count > 0 {
		junits = append(junits, &junitapi.JUnitTestCase{
			Name: earlyTestName,
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("found %d disruption events shortly after E2E test start (%s) with messages:\n%s",
					count, e2eStart.Format(monitorapi.TimeFormat), strings.Join(eventsFound, "\n")),
			},
		})
	}
	return append(junits, &junitapi.JUnitTestCase{Name: earlyTestName}) // success after fail makes a flake, to record when this is happening
}

func findDisruptionEvents(events monitorapi.Intervals, periodStart time.Time, periodEnd time.Time, backendMatcher *regexp.Regexp) []string {
	eventsFound := []string{}
	for _, event := range events {
		if event.Source != monitorapi.SourceDisruption || event.Message.Reason != monitorapi.DisruptionBeganEventReason {
			continue // only interested in disruption
		}
		if event.To.Before(periodStart) {
			continue // disruption ended before interval
		}
		if event.From.After(periodEnd) {
			break // no need to examine events entirely later than the period
		}
		// we are left with disruption events where some or all is in the period of interest
		if backendMatcher.MatchString(event.Locator.Keys["backend-disruption-name"]) {
			eventsFound = append(eventsFound, event.String())
		}
	}
	return eventsFound
}
