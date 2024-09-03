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
const earlyE2ECheckDuration = 3 * time.Minute

func testEarlyE2EAPIServerDisruption(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[Jira:\"kube-apiserver\"] API servers should not experience disruption near the start of E2E testing"

	// regex to match the backends we care about
	var earlyE2EBackendRe = regexp.MustCompile(`^(cache-)?(openshift|kube|oauth)-api-`)

	var periodStart, periodEnd time.Time // the time period we're interested in disruption
	for _, event := range events {
		if event.Source == monitorapi.SourceE2ETest {
			periodStart = event.From
			periodEnd = event.From.Add(earlyE2ECheckDuration)
			break
		}
	}

	if periodStart.IsZero() {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				SkipMessage: &junitapi.SkipMessage{
					Message: "no E2E tests ran", // no point in this test
				},
			},
		}
	}

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
		if earlyE2EBackendRe.MatchString(event.Locator.Keys["backend-disruption-name"]) {
			eventsFound = append(eventsFound, event.String())
		}
	}

	junits := []*junitapi.JUnitTestCase{}
	if count := len(eventsFound); count > 0 {
		junits = []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("found %d disruption events near E2E test start (%s) with messages:\n%s",
						count, periodStart.Format(monitorapi.TimeFormat), strings.Join(eventsFound, "\n")),
				},
			},
		}
	}
	successTest := &junitapi.JUnitTestCase{Name: testName}
	return append(junits, successTest) // only flake, never fail; just want to see how much this is happening
}
