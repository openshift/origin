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

func (w *nodeWatcher) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
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

	junits := []*junitapi.JUnitTestCase{}
	junits = append(junits, unexpectedNodeNotReadyJunit(finalIntervals)...)
	junits = append(junits, unreachableNodeTaint(finalIntervals)...)
	junits = append(junits, nodeDiskPressure(finalIntervals)...)
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

	failures := reportUnexpectedNodeDownFailures(finalIntervals, monitorapi.NodeUnexpectedReadyReason)
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
	failures := reportUnexpectedNodeDownFailures(finalIntervals, monitorapi.NodeUnexpectedUnreachableReason)

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

func intervalStartDuring(needle monitorapi.Interval, haystack monitorapi.Intervals) bool {
	if len(haystack) == 0 {
		// If there are no deleted intervals
		// we can assume that the unexpected event is significant.
		return false
	}
	for _, curr := range haystack {
		needleStartEqualOrAfterFrom := needle.From.Equal(curr.From) || needle.From.After(curr.From)
		needleStartEqualOrBeforeTo := needle.From.Equal(curr.To) || needle.From.Before(curr.To)
		if needleStartEqualOrAfterFrom || needleStartEqualOrBeforeTo {
			return true
		}
	}
	return false
}

func nodeDiskPressure(finalIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[Jira:\"Test Framework\"] kubelet should not report DiskPressure"

	diskPressureIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Message.Reason == monitorapi.NodeDiskPressure
	})

	var failures []string
	for _, dpi := range diskPressureIntervals {
		failures = append(failures, dpi.String())
	}

	var tests []*junitapi.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("found %d intervals where a node began reporting DiskPressure:\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
	} else {
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	}

	return tests
}
