package watchpods

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type podWatcher struct {
}

func NewPodWatcher() monitortestframework.MonitorTest {
	return &podWatcher{}
}

func (w *podWatcher) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	startPodMonitoring(ctx, recorder, kubeClient)

	return nil
}

func (w *podWatcher) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// because we are sharing a recorder that we're streaming into, we don't need to have a separate data collection step.
	return nil, nil, nil
}

func (*podWatcher) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	constructedIntervals := monitorapi.Intervals{}
	constructedIntervals = append(constructedIntervals, createPodIntervalsFromInstants(startingIntervals, recordedResources, beginning, end)...)
	constructedIntervals = append(constructedIntervals, intervalsFromEvents_PodChanges(startingIntervals, beginning, end)...)

	return constructedIntervals, nil
}

func (*podWatcher) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*podWatcher) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*podWatcher) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
