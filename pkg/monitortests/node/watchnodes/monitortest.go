package watchnodes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type nodeWatcher struct {
}

func NewNodeWatcher() monitortestframework.MonitorTest {
	return &nodeWatcher{}
}

func (w *nodeWatcher) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	startNodeMonitoring(ctx, recorder, kubeClient)

	return nil
}

func (w *nodeWatcher) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// because we are sharing a recorder that we're streaming into, we don't need to have a separate data collection step.
	return nil, nil, nil
}

func (*nodeWatcher) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	constructedIntervals := monitorapi.Intervals{}

	return constructedIntervals, nil
}

func (*nodeWatcher) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	// Fail tests when monitor test flags this as an error
	junits := []*junitapi.JUnitTestCase{}
	junits = append(junits, unexpectedNodeNotReadyJunit(finalIntervals)...)
	junits = append(junits, unreachableNodeTaint(finalIntervals)...)
	return junits, nil
}

func (*nodeWatcher) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*nodeWatcher) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

func unexpectedNodeNotReadyJunit(finalIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] node-lifecycle detects unexpected not ready node"
	var failures []string
	for _, event := range finalIntervals {
		if errorOnInterval(event, finalIntervals) && event.Message.Reason == monitorapi.NodeUnexpectedReadyReason {
			failures = append(failures, fmt.Sprintf("%v - %v at from: %v - to: %v", event.Locator.OldLocator(), event.Message.OldMessage(), event.From, event.To))
		}
	}

	// failures during a run always fail the test suite
	var tests []*junitapi.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("node-lifecycle reports %d unexpected notReady events. The node went NotReady in an unpredicted way.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
	}

	if len(tests) == 0 {
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	}
	return tests
}

func unreachableNodeTaint(finalIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] node-lifecycle detects unreachable state on node"
	var failures []string
	for _, event := range finalIntervals {
		if errorOnInterval(event, finalIntervals) && event.Message.Reason == monitorapi.NodeUnexpectedUnreachableReason {
			failures = append(failures, fmt.Sprintf("%v - %v from %v to %v", event.Locator.OldLocator(), event.Message.OldMessage(), event.From, event.To))
		}
	}

	// failures during a run always fail the test suite
	var tests []*junitapi.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("node-lifecycle reports %d unexpected node unreachable events. The node went unreachable in an unpredicted way.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
	}

	if len(tests) == 0 {
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	}
	return tests
}

func errorOnInterval(interval monitorapi.Interval, machineIntervals monitorapi.Intervals) bool {
	// There are a few cases we need to catch.
	for _, val := range machineIntervals {
		// case 1:
		// Interval is between the machine phase change - no overlap
		if interval.From.After(val.From) && interval.To.Before(val.To) {
			return false
		}
		// case 2:
		// Interval is after machine phase change but it lasts beyond the interval
		if interval.From.After(val.From) && interval.To.Before(val.To) {
			return false
		}
		// case 3:
		// Interval is before machine phase change but it ends before the interval ends.
		if interval.From.Before(val.From) && interval.To.After(val.To) {
			return false
		}
	}
	return true
}
