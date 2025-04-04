package legacystoragemonitortests

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type legacyMonitorTests struct {
	adminRESTConfig *rest.Config
}

func NewLegacyTests() monitortestframework.MonitorTest {
	return &legacyMonitorTests{}
}

func (w *legacyMonitorTests) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *legacyMonitorTests) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *legacyMonitorTests) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*legacyMonitorTests) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *legacyMonitorTests) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	junits = append(junits, testAPIQuotaEvents(finalIntervals)...)

	return junits, nil
}

func (*legacyMonitorTests) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*legacyMonitorTests) Cleanup(ctx context.Context) error {
	return nil
}
