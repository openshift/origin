package legacyetcdmonitortests

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type legacyMonitorTests struct {
	adminRESTConfig *rest.Config
	jobType         *platformidentification.JobType
}

func NewLegacyTests() monitortestframework.MonitorTest {
	return &legacyMonitorTests{}
}

func (w *legacyMonitorTests) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	jobType, err := platformidentification.GetJobType(ctx, adminRESTConfig)
	if err != nil {
		return fmt.Errorf("unable to determine job type: %v", err)
	}
	w.jobType = jobType
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
	junits = append(junits, testRequiredInstallerResourcesMissing(finalIntervals)...)
	junits = append(junits, testEtcdShouldNotLogSlowFdataSyncs(finalIntervals)...)
	junits = append(junits, testEtcdShouldNotLogDroppedRaftMessages(finalIntervals)...)
	junits = append(junits, testOperatorStatusChanged(finalIntervals)...)

	// see TRT-1688 - for now, for vsphere, count this test failure as a flake
	isVsphere := w.jobType.Platform == "vsphere"
	junits = append(junits, testEtcdDoesNotLogExcessiveTookTooLongMessages(finalIntervals, isVsphere)...)

	return junits, nil
}

func (*legacyMonitorTests) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*legacyMonitorTests) Cleanup(ctx context.Context) error {
	return nil
}
