package legacytestframeworkmonitortests

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

const (
	PathologicalMonitorName = "legacy-test-framework-invariants-pathological"
)

type legacyPathologicalMonitorTests struct {
	adminRESTConfig *rest.Config
	duration        time.Duration
}

func NewLegacyPathologicalMonitorTests(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &legacyPathologicalMonitorTests{}
}

func (w *legacyPathologicalMonitorTests) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *legacyPathologicalMonitorTests) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *legacyPathologicalMonitorTests) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	w.duration = end.Sub(beginning)
	return nil, nil, nil
}

func (w *legacyPathologicalMonitorTests) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *legacyPathologicalMonitorTests) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	isUpgrade := platformidentification.DidUpgradeHappenDuringCollection(finalIntervals, time.Time{}, time.Time{})
	if isUpgrade {
		junits = append(junits, pathologicaleventlibrary.TestDuplicatedEventForUpgrade(finalIntervals, w.adminRESTConfig)...)
	} else {
		junits = append(junits, pathologicaleventlibrary.TestDuplicatedEventForStableSystem(finalIntervals, w.adminRESTConfig)...)
	}

	// SHADOW: bounded oauth-openshift pathological scaling accounting. This is informational
	// only. It never forgives anything and never alters the verdicts produced above (nor the
	// existing 55/70 allowances). Budgets are nil until measured finite budgets are
	// established in a future, separate change.
	_, shadowJunits := pathologicaleventlibrary.EvaluateOAuthBoundedAccountingShadow(finalIntervals, nil)
	junits = append(junits, shadowJunits...)

	return junits, nil
}

func (*legacyPathologicalMonitorTests) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*legacyPathologicalMonitorTests) Cleanup(ctx context.Context) error {
	return nil
}
