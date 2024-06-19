package metricsendpointdown

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type metricsEndpointDown struct {
	adminRESTConfig *rest.Config
}

func NewMetricsEndpointDown() monitortestframework.MonitorTest {
	return &metricsEndpointDown{}
}

func (w *metricsEndpointDown) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *metricsEndpointDown) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	intervals, err := buildIntervalsForMetricsEndpointsDown(ctx, w.adminRESTConfig, beginning)
	return intervals, nil, err
}

func (*metricsEndpointDown) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*metricsEndpointDown) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*metricsEndpointDown) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*metricsEndpointDown) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
