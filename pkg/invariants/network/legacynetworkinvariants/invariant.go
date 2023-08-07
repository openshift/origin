package legacynetworkinvariants

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/invariantlibrary/platformidentification"

	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type legacyInvariantTests struct {
	adminRESTConfig *rest.Config
	duration        time.Duration
}

func NewLegacyTests() invariants.InvariantTest {
	return &legacyInvariantTests{}
}

func (w *legacyInvariantTests) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *legacyInvariantTests) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	w.duration = end.Sub(beginning)
	return nil, nil, nil
}

func (*legacyInvariantTests) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *legacyInvariantTests) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	jobType, err := platformidentification.GetJobType(context.TODO(), w.adminRESTConfig)
	if err != nil {
		// JobType will be nil here, but we want test cases to all fail if this is the case, so we rely on them to nil check
		logrus.WithError(err).Warn("ERROR: unable to determine job type for alert testing, jobType will be nil")
	}

	junits := []*junitapi.JUnitTestCase{}
	junits = append(junits, testPodSandboxCreation(finalIntervals, w.adminRESTConfig)...)
	junits = append(junits, testOvnNodeReadinessProbe(finalIntervals, w.adminRESTConfig)...)
	junits = append(junits, testNoDNSLookupErrorsInDisruptionSamplers(finalIntervals)...)
	junits = append(junits, testNoOVSVswitchdUnreasonablyLongPollIntervals(finalIntervals)...)
	junits = append(junits, testPodIPReuse(finalIntervals)...)
	junits = append(junits, testPodSandboxCreation(finalIntervals, w.adminRESTConfig)...)
	junits = append(junits, testOvnNodeReadinessProbe(finalIntervals, w.adminRESTConfig)...)
	junits = append(junits, testErrorUpdatingEndpointSlices(finalIntervals)...)
	junits = append(junits, testMultipleSingleSecondDisruptions(finalIntervals)...)
	junits = append(junits, testDNSOverlapDisruption(finalIntervals)...)
	junits = append(junits, TestAllIngressBackendsForDisruption(finalIntervals, w.duration, jobType)...)

	return junits, nil
}

func (*legacyInvariantTests) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*legacyInvariantTests) Cleanup(ctx context.Context) error {
	return nil
}
