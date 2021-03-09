package synthetictests

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/test/ginkgo"
	"k8s.io/apimachinery/pkg/util/sets"
)

func testOperatorStateTransitions(events []*monitor.EventInterval) []*ginkgo.JUnitTestCase {
	ret := []*ginkgo.JUnitTestCase{}

	knownOperators := allOperators(events)
	eventsByOperator := getEventsByOperator(events)
	e2eEvents := monitor.E2ETestEvents(events)
	for _, condition := range []string{"Available", "Degraded", "Progressing"} {
		for _, operatorName := range knownOperators.List() {
			bzComponent := GetBugzillaComponentForOperator(operatorName)
			if bzComponent == "Unknown" {
				bzComponent = operatorName
			}
			testName := fmt.Sprintf("[bz-%v] clusteroperator/%v should not change condition/%v", bzComponent, operatorName, condition)
			operatorEvents := eventsByOperator[operatorName]
			if len(operatorEvents) == 0 {
				ret = append(ret, &ginkgo.JUnitTestCase{Name: testName})
				continue
			}

			failures := testOperatorState(condition, operatorEvents, e2eEvents)
			if len(failures) > 0 {
				ret = append(ret, &ginkgo.JUnitTestCase{
					Name:      testName,
					SystemOut: strings.Join(failures, "\n"),
					FailureOutput: &ginkgo.FailureOutput{
						Output: fmt.Sprintf("%d unexpected clusteroperator state transitions during e2e test run \n\n%v", len(failures), strings.Join(failures, "\n")),
					},
				})
			}
			// always add a success so we flake and not fail
			ret = append(ret, &ginkgo.JUnitTestCase{Name: testName})
		}
	}

	return ret
}

func allOperators(events []*monitor.EventInterval) sets.String {
	// start with a list of known values
	knownOperators := sets.NewString(KnownOperators.List()...)

	// now add all the operators we see in the events.
	for _, event := range events {
		operatorName := operatorFromLocator(event.Locator)
		if len(operatorName) == 0 {
			continue
		}
		knownOperators.Insert(operatorName)
	}
	return knownOperators
}

// getEventsByOperator returns map keyed by operator locator with all events associated with it.
func getEventsByOperator(events []*monitor.EventInterval) map[string][]*monitor.EventInterval {
	eventsByClusterOperator := map[string][]*monitor.EventInterval{}
	for _, event := range events {
		if !strings.Contains(event.Locator, "clusteroperator/") {
			continue
		}
		operators := strings.Split(event.Locator, "/")
		operatorName := operators[1]
		eventsByClusterOperator[operatorName] = append(eventsByClusterOperator[operatorName], event)
	}
	return eventsByClusterOperator
}

func testOperatorState(interestingCondition string, events []*monitor.EventInterval, e2eEvents []*monitor.EventInterval) []string {
	failures := []string{}

	clusterOperatorToEvents := getEventsByOperator(events)

	var previousEvent *monitor.EventInterval
	for clusterOperator, events := range clusterOperatorToEvents {
		for _, event := range events {
			condition, status, message := monitor.GetOperatorConditionStatus(event.Message)
			if condition != interestingCondition {
				continue
			}
			// if there was any switch, it was wrong/unexpected at some point
			failures = append(failures, fmt.Sprintf("%v was %v=%v, but became %v=%v at %v -- %v", clusterOperator, condition, !status, condition, status, event.From, message))

			if !isTransitionToGoodOperatorState(interestingCondition, status) {
				// we don't see a lot of these as end states, don't bother trying to find the tests that were running during this time.
				continue
			}

			startTime := time.Now().Add(4 * time.Hour)
			if previousEvent != nil {
				startTime = previousEvent.InitiatedAt
			}
			failedTests := monitor.FindFailedTestsActiveBetween(e2eEvents, startTime, event.InitiatedAt)
			if len(failedTests) > 0 {
				failures = append(failures, fmt.Sprintf("%d tests failed during this blip (%v to %v): %v", len(failedTests), startTime, event.InitiatedAt, strings.Join(failedTests, ",")))
			}
		}
	}
	return failures
}

func operatorFromLocator(locator string) string {
	if !strings.Contains(locator, "clusteroperator/") {
		return ""
	}
	operators := strings.Split(locator, "/")
	return operators[1]
}

func isTransitionToGoodOperatorState(condition string, status bool) bool {
	switch condition {
	case "Available":
		if status {
			return true
		}
		return false
	case "Degraded":
		if !status {
			return true
		}
		return false
	case "Progressing":
		if !status {
			return true
		}
		return false
	default:
		return false
	}
}
