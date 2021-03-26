package synthetictests

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/test/ginkgo"
	"k8s.io/apimachinery/pkg/util/sets"
)

func testOperatorStateTransitions(events []*monitorapi.EventInterval) []*ginkgo.JUnitTestCase {
	ret := []*ginkgo.JUnitTestCase{}

	knownOperators := allOperators(events)
	eventsByOperator := getEventsByOperator(events)
	e2eEvents := monitor.E2ETestEvents(events)
	for _, condition := range []configv1.ClusterStatusConditionType{configv1.OperatorAvailable, configv1.OperatorDegraded, configv1.OperatorProgressing} {
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

func allOperators(events []*monitorapi.EventInterval) sets.String {
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
func getEventsByOperator(events []*monitorapi.EventInterval) map[string][]*monitorapi.EventInterval {
	eventsByClusterOperator := map[string][]*monitorapi.EventInterval{}
	for _, event := range events {
		operatorName, ok := monitorapi.OperatorFromLocator(event.Locator)
		if !ok {
			continue
		}
		eventsByClusterOperator[operatorName] = append(eventsByClusterOperator[operatorName], event)
	}
	return eventsByClusterOperator
}

func testOperatorState(interestingCondition configv1.ClusterStatusConditionType, events []*monitorapi.EventInterval, e2eEvents []*monitorapi.EventInterval) []string {
	failures := []string{}

	clusterOperatorToEvents := getEventsByOperator(events)

	var previousEvent *monitorapi.EventInterval

	for clusterOperator, events := range clusterOperatorToEvents {
		for _, event := range events {
			condition := monitor.GetOperatorConditionStatus(event.Message)
			if condition == nil || condition.Type != interestingCondition {
				continue
			}
			// if there was any switch, it was wrong/unexpected at some point
			failures = append(failures, fmt.Sprintf("%v became %v=%v at %v -- reason/%v: %v", clusterOperator, condition.Type, condition.Status, event.From, condition.Reason, condition.Message))

			if !isTransitionToGoodOperatorState(condition) {
				// we don't see a lot of these as end states, don't bother trying to find the tests that were running during this time.
				continue
			}

			startTime := time.Now().Add(4 * time.Hour)
			if previousEvent != nil {
				startTime = previousEvent.From
			}
			failedTests := monitor.FindFailedTestsActiveBetween(e2eEvents, startTime, event.From)
			if len(failedTests) > 0 {
				failures = append(failures, fmt.Sprintf("%d tests failed during this blip (%v to %v): %v", len(failedTests), startTime, event.From, strings.Join(failedTests, ",")))
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

func isTransitionToGoodOperatorState(condition *configv1.ClusterOperatorStatusCondition) bool {
	switch condition.Type {
	case configv1.OperatorAvailable:
		return condition.Status == configv1.ConditionTrue
	case configv1.OperatorDegraded:
		return condition.Status == configv1.ConditionFalse
	case configv1.OperatorProgressing:
		return condition.Status == configv1.ConditionFalse
	default:
		return false
	}
}
