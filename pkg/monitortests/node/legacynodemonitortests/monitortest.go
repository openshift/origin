package legacynodemonitortests

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

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

	clusterData, _ := platformidentification.BuildClusterData(context.Background(), w.adminRESTConfig)

	containerFailures, err := testContainerFailures(w.adminRESTConfig, finalIntervals)
	if err != nil {
		return nil, err
	}
	junits := []*junitapi.JUnitTestCase{}
	junits = append(junits, containerFailures...)
	junits = append(junits, testDeleteGracePeriodZero(finalIntervals)...)
	junits = append(junits, testKubeApiserverProcessOverlap(finalIntervals)...)
	junits = append(junits, testKubeAPIServerGracefulTermination(finalIntervals)...)
	junits = append(junits, testKubeletToAPIServerGracefulTermination(finalIntervals)...)
	junits = append(junits, testPodTransitions(finalIntervals)...)
	junits = append(junits, testErrImagePullConnTimeoutOpenShiftNamespaces(finalIntervals)...)
	junits = append(junits, testErrImagePullConnTimeout(finalIntervals)...)
	junits = append(junits, testErrImagePullQPSExceededOpenShiftNamespaces(finalIntervals)...)
	junits = append(junits, testErrImagePullQPSExceeded(finalIntervals)...)
	junits = append(junits, testErrImagePullManifestUnknownOpenShiftNamespaces(finalIntervals)...)
	junits = append(junits, testErrImagePullManifestUnknown(finalIntervals)...)
	junits = append(junits, testErrImagePullGenericOpenShiftNamespaces(finalIntervals)...)
	junits = append(junits, testErrImagePullGeneric(finalIntervals)...)
	junits = append(junits, testFailedToDeleteCGroupsPath(finalIntervals)...)
	junits = append(junits, testAnonymousCertConnectionFailure(finalIntervals)...)
	junits = append(junits, testHttpConnectionLost(finalIntervals)...)
	junits = append(junits, testErrImagePullUnrecognizedSignatureFormat(finalIntervals)...)
	junits = append(junits, testLeaseUpdateError(finalIntervals)...)
	junits = append(junits, testSystemDTimeout(finalIntervals)...)
	junits = append(junits, testNodeHasNoDiskPressure(finalIntervals)...)
	junits = append(junits, testNodeHasSufficientMemory(finalIntervals)...)
	junits = append(junits, testNodeHasSufficientPID(finalIntervals)...)
	junits = append(junits, testBackoffPullingRegistryRedhatImage(finalIntervals)...)
	junits = append(junits, testBackoffStartingFailedContainer(clusterData, finalIntervals)...)
	junits = append(junits, testConfigOperatorReadinessProbe(finalIntervals)...)
	junits = append(junits, testConfigOperatorProbeErrorReadinessProbe(finalIntervals)...)
	junits = append(junits, testConfigOperatorProbeErrorLivenessProbe(finalIntervals)...)
	junits = append(junits, testMasterNodesUpdated(finalIntervals)...)
	junits = append(junits, testMarketplaceStartupProbeFailure(finalIntervals)...)
	junits = append(junits, testFailedScheduling(finalIntervals)...)
	junits = append(junits, testBackoffStartingFailedContainerForE2ENamespaces(finalIntervals)...)

	isUpgrade := platformidentification.DidUpgradeHappenDuringCollection(finalIntervals, time.Time{}, time.Time{})
	if isUpgrade {
		junits = append(junits, testNodeUpgradeTransitions(finalIntervals)...)
	}

	return junits, nil
}

func (*legacyMonitorTests) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*legacyMonitorTests) Cleanup(ctx context.Context) error {
	return nil
}
