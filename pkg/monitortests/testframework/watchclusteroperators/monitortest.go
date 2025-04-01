package watchclusteroperators

import (
	"context"
	"time"

	configclient "github.com/openshift/client-go/config/clientset/versioned"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type operatorWatcher struct {
}

func NewOperatorWatcher() monitortestframework.MonitorTest {
	return &operatorWatcher{}
}

func (w *operatorWatcher) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *operatorWatcher) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	configClient, err := configclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	startClusterOperatorMonitoring(ctx, recorder, configClient)

	return nil
}

func (w *operatorWatcher) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// because we are sharing a recorder that we're streaming into, we don't need to have a separate data collection step.
	return nil, nil, nil
}

func (*operatorWatcher) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	constructedIntervals := monitorapi.Intervals{}

	return constructedIntervals, nil
}

func (*operatorWatcher) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*operatorWatcher) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*operatorWatcher) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
