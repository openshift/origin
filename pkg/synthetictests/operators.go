package synthetictests

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/test/ginkgo"
	"k8s.io/apimachinery/pkg/util/sets"
)

func testStableSystemOperatorStateTransitions(events monitorapi.Intervals) []*ginkgo.JUnitTestCase {
	return testOperatorStateTransitions(events, []configv1.ClusterStatusConditionType{configv1.OperatorAvailable, configv1.OperatorDegraded, configv1.OperatorProgressing})
}

func testUpgradeOperatorStateTransitions(events monitorapi.Intervals) []*ginkgo.JUnitTestCase {
	return testOperatorStateTransitions(events, []configv1.ClusterStatusConditionType{configv1.OperatorAvailable, configv1.OperatorDegraded})
}
func testOperatorStateTransitions(events monitorapi.Intervals, conditionTypes []configv1.ClusterStatusConditionType) []*ginkgo.JUnitTestCase {
	ret := []*ginkgo.JUnitTestCase{}

	knownOperators := allOperators(events)
	eventsByOperator := getEventsByOperator(events)
	e2eEventIntervals := monitor.E2ETestEventIntervals(events)
	for _, condition := range conditionTypes {
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

			failures := testOperatorState(condition, operatorEvents, e2eEventIntervals)
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

func allOperators(events monitorapi.Intervals) sets.String {
	// start with a list of known values
	knownOperators := sets.NewString(KnownOperators.List()...)

	// now add all the operators we see in the events.
	for _, event := range events {
		operatorName, ok := monitorapi.OperatorFromLocator(event.Locator)
		if !ok {
			continue
		}
		knownOperators.Insert(operatorName)
	}
	return knownOperators
}

// getEventsByOperator returns map keyed by operator locator with all events associated with it.
func getEventsByOperator(events monitorapi.Intervals) map[string]monitorapi.Intervals {
	eventsByClusterOperator := map[string]monitorapi.Intervals{}
	for _, event := range events {
		operatorName, ok := monitorapi.OperatorFromLocator(event.Locator)
		if !ok {
			continue
		}
		eventsByClusterOperator[operatorName] = append(eventsByClusterOperator[operatorName], event)
	}
	return eventsByClusterOperator
}

func testOperatorState(interestingCondition configv1.ClusterStatusConditionType, eventIntervals monitorapi.Intervals, e2eEventIntervals monitorapi.Intervals) []string {
	failures := []string{}

	for _, eventInterval := range eventIntervals {
		// ignore non-interval eventInterval intervals
		if eventInterval.From == eventInterval.To {
			continue
		}
		if !strings.Contains(eventInterval.Message, fmt.Sprintf("%v", interestingCondition)) {
			continue
		}

		// if there was any switch, it was wrong/unexpected at some point
		failures = append(failures, fmt.Sprintf("%v", eventInterval))

		overlappingE2EIntervals := monitor.FindOverlap(e2eEventIntervals, eventInterval.From, eventInterval.From)
		concurrentE2E := []string{}
		for _, overlap := range overlappingE2EIntervals {
			if overlap.Level == monitorapi.Info {
				continue
			}
			e2eTest, ok := monitorapi.E2ETestFromLocator(overlap.Locator)
			if !ok {
				continue
			}
			concurrentE2E = append(concurrentE2E, fmt.Sprintf("%v", e2eTest))
		}

		if len(concurrentE2E) > 0 {
			failures = append(failures, fmt.Sprintf("%d tests failed during this blip (%v to %v): %v", len(concurrentE2E), eventInterval.From, eventInterval.From, strings.Join(concurrentE2E, "\n")))
		}
	}
	return failures
}
