package synthetictests

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
)

func testStableSystemOperatorStateTransitions(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	return testOperatorStateTransitions(events, []configv1.ClusterStatusConditionType{configv1.OperatorAvailable, configv1.OperatorDegraded})
}

func testUpgradeOperatorStateTransitions(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	return testOperatorStateTransitions(events, []configv1.ClusterStatusConditionType{configv1.OperatorAvailable, configv1.OperatorDegraded})
}
func testOperatorStateTransitions(events monitorapi.Intervals, conditionTypes []configv1.ClusterStatusConditionType) []*junitapi.JUnitTestCase {
	ret := []*junitapi.JUnitTestCase{}

	var start, stop time.Time
	for _, event := range events {
		if start.IsZero() || event.From.Before(start) {
			start = event.From
		}
		if stop.IsZero() || event.To.After(stop) {
			stop = event.To
		}
	}
	duration := stop.Sub(start).Seconds()

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
				ret = append(ret, &junitapi.JUnitTestCase{
					Name:     testName,
					Duration: duration,
				})
				continue
			}

			failures := testOperatorState(condition, operatorEvents, e2eEventIntervals)
			if len(failures) > 0 {
				ret = append(ret, &junitapi.JUnitTestCase{
					Name:      testName,
					Duration:  duration,
					SystemOut: strings.Join(failures, "\n"),
					FailureOutput: &junitapi.FailureOutput{
						Output: fmt.Sprintf("%d unexpected clusteroperator state transitions during e2e test run \n\n%v", len(failures), strings.Join(failures, "\n")),
					},
				})
			}
			// always add a success so we flake and not fail
			ret = append(ret, &junitapi.JUnitTestCase{Name: testName})
		}
	}

	return ret
}

func testOperatorOSUpdateStaged(events monitorapi.Intervals, clientConfig *rest.Config) []*junitapi.JUnitTestCase {
	testName := "[bz-Machine Config Operator] Nodes should reach OSUpdateStaged in a timely fashion"
	success := &junitapi.JUnitTestCase{Name: testName}
	flakeThreshold := 5 * time.Minute
	failThreshold := 10 * time.Minute

	type startedStaged struct {
		// OSUpdateStarted is the event Reason emitted by the machine config operator when a node begins extracting
		// it's OS content.
		OSUpdateStarted time.Time
		// OSUpdateStaged is the event Reason emitted by the machine config operator when a node has extracted it's
		// OS content and is ready to begin the update. For the purposes of this test, we're looking for how long it
		// took from Started -> Staged to try to identify disk i/o problems that may be occurring.
		OSUpdateStaged time.Time
	}

	// Scan all OSUpdateStarted and OSUpdateStaged events, sort by node.
	nodeOSUpdateTimes := map[string]*startedStaged{}
	for _, e := range events {
		if strings.Contains(e.Message, "reason/OSUpdateStarted") {
			// locator will be of the form: node/ci-op-j34hmfqt-253f3-cq852-master-1
			_, ok := nodeOSUpdateTimes[e.Locator]
			if !ok {
				nodeOSUpdateTimes[e.Locator] = &startedStaged{}
			}
			// for this type of event, the from/to time are identical as this is a point in time event.
			ss := nodeOSUpdateTimes[e.Locator]
			ss.OSUpdateStarted = e.To
		} else if strings.Contains(e.Message, "reason/OSUpdateStaged") {
			// locator will be of the form: node/ci-op-j34hmfqt-253f3-cq852-master-1
			_, ok := nodeOSUpdateTimes[e.Locator]
			if !ok {
				nodeOSUpdateTimes[e.Locator] = &startedStaged{}
			}
			// for this type of event, the from/to time are identical as this is a point in time event.
			ss := nodeOSUpdateTimes[e.Locator]
			ss.OSUpdateStaged = e.To
		}
	}

	// Iterate the data we assembled looking for any nodes with an excessive time between Started/Staged, or those
	// missing a Staged
	slowStageMessages := []string{}
	var failTest bool // set true if we see anything over 10 minutes, our failure threshold
	for node, ss := range nodeOSUpdateTimes {
		if ss.OSUpdateStarted.IsZero() {
			// We've seen this on metal where we've got no start time, the event is in the gather-extra/events.json but
			// not in the junit/e2e-events.json the test framework writes afterwards, unclear how it's getting missed.
			slowStageMessages = append(slowStageMessages, fmt.Sprintf("%s OSUpdateStaged at %s, but no OSUpdateStarted event was recorded", node, ss.OSUpdateStaged.Format(time.RFC3339)))
			failTest = true // considering this a failure for now
		} else if ss.OSUpdateStaged.IsZero() || ss.OSUpdateStarted.After(ss.OSUpdateStaged) {
			// Watch that we don't do multiple started->staged transitions, if we see started > staged, we must have
			// failed to make it to staged on a later update:
			slowStageMessages = append(slowStageMessages, fmt.Sprintf("%s OSUpdateStarted at %s, did not make it to OSUpdateStaged", node, ss.OSUpdateStarted.Format(time.RFC3339)))
			failTest = true
		} else if ss.OSUpdateStaged.Sub(ss.OSUpdateStarted) > flakeThreshold {
			slowStageMessages = append(slowStageMessages, fmt.Sprintf("%s OSUpdateStarted at %s, OSUpdateStaged at %s: %s", node,
				ss.OSUpdateStarted.Format(time.RFC3339), ss.OSUpdateStaged.Format(time.RFC3339), ss.OSUpdateStaged.Sub(ss.OSUpdateStarted)))

			if ss.OSUpdateStaged.Sub(ss.OSUpdateStarted) > failThreshold {
				failTest = true
			}
		}
	}

	// Make sure we flake instead of fail the test on platforms that struggle to meet these thresholds.
	if failTest {
		// If an error occurs getting the platform, we're just going to let the test result stand.
		jobType, err := platformidentification.GetJobType(context.TODO(), clientConfig)
		if err == nil && (jobType.Platform == "ovirt" || jobType.Platform == "metal") {
			failTest = false
		}
	}

	if len(slowStageMessages) > 0 {
		output := fmt.Sprintf("%d nodes took over %s to stage OSUpdate:\n\n%s",
			len(slowStageMessages), flakeThreshold, strings.Join(slowStageMessages, "\n"))
		failure := &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: output,
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
		}
		if failTest {
			return []*junitapi.JUnitTestCase{failure}
		}
		return []*junitapi.JUnitTestCase{failure, success}
	}

	return []*junitapi.JUnitTestCase{success}
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
