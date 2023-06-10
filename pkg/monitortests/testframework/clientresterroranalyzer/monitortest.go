package clientresterroranalyzer

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type clientRestErrorSerializer struct {
	adminRESTConfig *rest.Config
}

func NewClientRestErrorSerializer() monitortestframework.MonitorTest {
	return &clientRestErrorSerializer{}
}

func (c *clientRestErrorSerializer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	c.adminRESTConfig = adminRESTConfig
	return nil
}

func (c *clientRestErrorSerializer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	intervals, err := fetchEventIntervalsForRestClientError(ctx, c.adminRESTConfig, beginning)
	return intervals, nil, err
}

func (*clientRestErrorSerializer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*clientRestErrorSerializer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*clientRestErrorSerializer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*clientRestErrorSerializer) Cleanup(ctx context.Context) error {
	return nil
}
