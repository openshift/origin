package legacytestframeworkinvariants

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/invariantlibrary/pathologicaleventlibrary"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/alerts"
	"github.com/openshift/origin/pkg/invariantlibrary/platformidentification"
	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type legacyInvariantTests struct {
	adminRESTConfig   *rest.Config
	duration          time.Duration
	recordedResources monitorapi.ResourcesMap
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

func (w *legacyInvariantTests) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	w.recordedResources = recordedResources
	return nil, nil
}

func (w *legacyInvariantTests) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	jobType, err := platformidentification.GetJobType(context.TODO(), w.adminRESTConfig)
	if err != nil {
		// JobType will be nil here, but we want test cases to all fail if this is the case, so we rely on them to nil check
		logrus.WithError(err).Warn("ERROR: unable to determine job type for alert testing, jobType will be nil")
	}

	junits := []*junitapi.JUnitTestCase{}

	isUpgrade := platformidentification.DidUpgradeHappenDuringCollection(finalIntervals, time.Time{}, time.Time{})
	if isUpgrade {
		junits = append(junits, pathologicaleventlibrary.TestDuplicatedEventForUpgrade(finalIntervals, w.adminRESTConfig)...)
		junits = append(junits, testAlerts(finalIntervals, alerts.AllowedAlertsDuringUpgrade, jobType,
			w.adminRESTConfig, w.duration, w.recordedResources)...)
	} else {
		junits = append(junits, pathologicaleventlibrary.TestDuplicatedEventForStableSystem(finalIntervals, w.adminRESTConfig)...)
		junits = append(junits, testAlerts(finalIntervals, alerts.AllowedAlertsDuringConformance, jobType,
			w.adminRESTConfig, w.duration, w.recordedResources)...)
	}

	return junits, nil
}

func (*legacyInvariantTests) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*legacyInvariantTests) Cleanup(ctx context.Context) error {
	return nil
}
