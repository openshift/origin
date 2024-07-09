package legacykubeapiservermonitortests

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/library-go/test/library/junitapi"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// amount of time near E2E test start that we consider "early"
const preE2ECheckDuration = 3 * time.Minute
const earlyE2ECheckDuration = 3 * time.Minute

func testEarlyE2EAPIServerDisruption(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	var periodStart, periodEnd time.Time // the time period we're interested in disruption
	for _, event := range events {
		if event.Source == monitorapi.SourceE2ETest {
			periodStart = event.From.Add(-preE2ECheckDuration)
			periodEnd = event.From.Add(earlyE2ECheckDuration)
			break
		}
	}

	const kubeTestName = "[Jira:\"kube-apiserver\"] kube API servers should not experience disruption near the start of E2E testing"
	const oauthTestName = "[Jira:\"oauth-apiserver\"] oauth API servers should not experience disruption near the start of E2E testing"
	if periodStart.IsZero() {
		msg := &junitapi.SkipMessage{
			Message: "no E2E tests ran", // no point in this test
		}
		return []*junitapi.JUnitTestCase{
			{Name: kubeTestName, SkipMessage: msg},
			{Name: oauthTestName, SkipMessage: msg},
		}
	}

	kubeMatcher := regexp.MustCompile(`^(cache-)?(openshift|kube)-api-`)
	kubeEvents := []string{} // the api disruption described in TRT-1794
	oauthMatcher := regexp.MustCompile(`^(cache-)?oauth-api-`)
	oauthEvents := []string{} // the oauth disruption described in OCPBUGS-39021

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
		if backend := event.Locator.Keys["backend-disruption-name"]; kubeMatcher.MatchString(backend) {
			kubeEvents = append(kubeEvents, event.String())
		} else if oauthMatcher.MatchString(backend) {
			oauthEvents = append(oauthEvents, event.String())
		}
	}

	junits := []*junitapi.JUnitTestCase{}
	if count := len(kubeEvents); count > 0 {
		junits = append(junits, &junitapi.JUnitTestCase{
			Name: kubeTestName,
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("found %d apiserver disruption events near E2E test start (%s) with messages:\n%s",
					count, periodStart.Format(monitorapi.TimeFormat), strings.Join(kubeEvents, "\n")),
			},
		})
	}
	junits = append(junits, &junitapi.JUnitTestCase{Name: kubeTestName}) // success after fail makes a flake, to record when this is happening

	if count := len(oauthEvents); count > 0 {
		junits = append(junits, &junitapi.JUnitTestCase{
			Name: oauthTestName,
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("found %d oauthserver disruption events near E2E test start (%s) with messages:\n%s",
					count, periodStart.Format(monitorapi.TimeFormat), strings.Join(oauthEvents, "\n")),
			},
		})
	}
	return append(junits, &junitapi.JUnitTestCase{Name: oauthTestName}) // success after fail makes a flake, to record when this is happening
}
