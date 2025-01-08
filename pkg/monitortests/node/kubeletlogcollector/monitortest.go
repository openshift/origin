package kubeletlogcollector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type kubeletLogCollector struct {
	adminRESTConfig *rest.Config
	startedAt       time.Time
}

func NewKubeletLogCollector() monitortestframework.MonitorTest {
	return &kubeletLogCollector{}
}

func (w *kubeletLogCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	w.startedAt = time.Now()
	return nil
}

func (w *kubeletLogCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, nil, err
	}
	// MicroShift does not have a proper journal for the node logs api.
	isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
	if err != nil {
		return nil, nil, err
	}
	if isMicroShift {
		return nil, nil, nil
	}

	intervals, err := intervalsFromNodeLogs(ctx, kubeClient, beginning, end)
	return intervals, nil, err
}

func (*kubeletLogCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *kubeletLogCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	junits = append(junits, nodeFailedLeaseErrorsInRapidSuccession(w.startedAt, finalIntervals)...)
	junits = append(junits, nodeFailedLeaseErrorsBackOff(w.startedAt, finalIntervals)...)
	return junits, nil
}

func (*kubeletLogCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*kubeletLogCollector) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

func nodeFailedLeaseErrorsInRapidSuccession(startedAt time.Time, finalIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] kubelet-log-collector detects node failed to lease events in rapid succession"
	var failures []string

	intervalsToFailOn := findLeaseIntervalsImportant(finalIntervals)
	for _, event := range intervalsToFailOn {
		if event.From.After(startedAt) {
			failures = append(failures, fmt.Sprintf("%s %v - %v", event.From.Format(time.RFC3339), event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}

	// failures during a run always fail the test suite
	var tests []*junitapi.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("kubelet-log-collector reports %d node failed to lease events.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
		return tests
	} else {
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
		return tests
	}
}

func nodeFailedLeaseErrorsBackOff(startedAt time.Time, finalIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] kubelet-log-collector detected lease failures in backoff"
	var failures []string
	intervalsToFlake := findLeaseBackOffs(finalIntervals)
	for _, event := range intervalsToFlake {
		if event.From.After(startedAt) {
			failures = append(failures, fmt.Sprintf("%s %v - %v", event.From.Format(time.RFC3339), event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}
	// failures during a run always fail the test suite
	var tests []*junitapi.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("kubelet-log-collector reports %d lease back off events.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
	}

	// Mark as a flake and monitor in 4.18.
	tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	return tests
}
