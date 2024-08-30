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
	machineDeletePhases := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason != monitorapi.MachinePhase {
			return false
		}
		if eventInterval.Message.Annotations[monitorapi.AnnotationPhase] == "Deleting" {
			return true
		}
		return false
	})

	nodeNameToMachineName := map[string]string{}
	machineNameToDeletePhases := map[string][]monitorapi.Interval{}
	for _, machineDeletePhase := range machineDeletePhases {
		machineName := machineDeletePhase.Locator.Keys[monitorapi.LocatorMachineKey]
		nodeName := machineDeletePhase.Message.Annotations[monitorapi.AnnotationNode]
		machineNameToDeletePhases[machineName] = append(machineNameToDeletePhases[machineName], machineDeletePhase)
		nodeNameToMachineName[nodeName] = machineName
	}

	// Fail tests when monitor test flags this as an error
	junits := []*junitapi.JUnitTestCase{}
	junits = append(junits, unexpectedNodeNotReadyJunit(finalIntervals, nodeNameToMachineName, machineNameToDeletePhases)...)
	junits = append(junits, unreachableNodeTaint(finalIntervals, nodeNameToMachineName, machineNameToDeletePhases)...)
	return junits, nil
}

func (*nodeWatcher) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*nodeWatcher) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

func unexpectedNodeNotReadyJunit(finalIntervals monitorapi.Intervals, nodeNameToMachineName map[string]string, machinesToDeletePhases map[string][]monitorapi.Interval) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] node-lifecycle detects unexpected not ready node"

	unexpectedNodeUnreadies := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason == monitorapi.NodeUnexpectedReadyReason {
			return true
		}
		return false
	})

	var failures []string
	for _, unexpectedNodeUnready := range unexpectedNodeUnreadies {
		nodeName := unexpectedNodeUnready.Locator.Keys[monitorapi.LocatorNodeKey]
		machineNameForNode := nodeNameToMachineName[nodeName]
		machineDeletingIntervals := machinesToDeletePhases[machineNameForNode]

		if intervalStartDuring(unexpectedNodeUnready, machineDeletingIntervals) {
			failures = append(failures, fmt.Sprintf("%v - %v at from: %v - to: %v", unexpectedNodeUnready.Locator.OldLocator(), unexpectedNodeUnready.Message.OldMessage(), unexpectedNodeUnready.From, unexpectedNodeUnready.To))
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

func unreachableNodeTaint(finalIntervals monitorapi.Intervals, nodeNameToMachineName map[string]string, machinesToDeletePhases map[string][]monitorapi.Interval) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] node-lifecycle detects unreachable state on node"
	var failures []string

	unexpectedNodeUnreachables := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason == monitorapi.NodeUnexpectedUnreachableReason {
			return true
		}
		return false
	})

	for _, unexpectedNodeUnreachable := range unexpectedNodeUnreachables {
		nodeName := unexpectedNodeUnreachable.Locator.Keys[monitorapi.LocatorNodeKey]
		machineNameForNode := nodeNameToMachineName[nodeName]
		machineDeletingIntervals := machinesToDeletePhases[machineNameForNode]

		if intervalStartDuring(unexpectedNodeUnreachable, machineDeletingIntervals) {
			failures = append(failures, fmt.Sprintf("%v - %v from %v to %v", unexpectedNodeUnreachable.Locator.OldLocator(), unexpectedNodeUnreachable.Message.OldMessage(), unexpectedNodeUnreachable.From, unexpectedNodeUnreachable.To))
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

func intervalStartDuring(needle monitorapi.Interval, haystack monitorapi.Intervals) bool {
	for _, curr := range haystack {
		needleStartEqualOrAfterFrom := needle.From.Equal(curr.From) || needle.From.After(curr.From)
		needleStartEqualOrBeforeTo := needle.From.Equal(curr.To) || needle.From.Before(curr.To)
		if needleStartEqualOrAfterFrom || needleStartEqualOrBeforeTo {
			return true
		}
	}
	return false
}
