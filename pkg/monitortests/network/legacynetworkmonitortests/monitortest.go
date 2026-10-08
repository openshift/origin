package legacynetworkmonitortests

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

type legacyMonitorTests struct {
	adminRESTConfig *rest.Config
	duration        time.Duration
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
	w.duration = end.Sub(beginning)
	return nil, nil, nil
}

func (*legacyMonitorTests) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *legacyMonitorTests) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	skipROSA, err := isROSACluster(w.adminRESTConfig)
	if err != nil {
		logrus.WithError(err).Warn("unable to determine whether legacy network monitor tests are running on ROSA")
	}

	junits := []*junitapi.JUnitTestCase{}
	junits = append(junits, testPodSandboxCreation(finalIntervals, w.adminRESTConfig, skipROSA)...)
	junits = append(junits, testOvnNodeReadinessProbe(finalIntervals, w.adminRESTConfig)...)
	junits = append(junits, testNoDNSLookupErrorsInDisruptionSamplers(finalIntervals)...)
	junits = append(junits, testNoExcessiveDNSDisruption(finalIntervals)...)
	junits = append(junits, testNoOVSVswitchdUnreasonablyLongPollIntervals(finalIntervals, skipROSA)...)
	junits = append(junits, testPodIPReuse(finalIntervals)...)
	junits = append(junits, testErrorUpdatingEndpointSlices(finalIntervals)...)
	junits = append(junits, TestMultipleSingleSecondDisruptions(finalIntervals, w.adminRESTConfig)...)
	junits = append(junits, testDNSOverlapDisruption(finalIntervals)...)
	junits = append(junits, testNoTooManyNetlinkEventLogs(finalIntervals)...)

	return junits, nil
}

func (*legacyMonitorTests) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*legacyMonitorTests) Cleanup(ctx context.Context) error {
	return nil
}
