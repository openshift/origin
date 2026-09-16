package legacynodemonitortests

import (
	"context"
	"errors"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

type legacyMonitorTests struct {
	adminRESTConfig *rest.Config
	topology        string
	reducedTopology bool
}

func NewLegacyTests() monitortestframework.MonitorTest {
	return &legacyMonitorTests{}
}

func (w *legacyMonitorTests) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

// StartCollection resolves the control plane topology up front, while the
// cluster is still healthy — evaluation runs after whatever disruption the suite
// caused, which is the worst moment to ask the apiserver a question.
func (w *legacyMonitorTests) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig

	reducedTopology, topology, err := platformidentification.ResolveReducedTopology(ctx, adminRESTConfig)
	if err != nil {
		logrus.Warningf("legacy-node-monitor-tests: couldn't determine control plane topology: %s", utility.ErrorSummary(err))
	}
	w.reducedTopology = reducedTopology
	w.topology = topology

	return nil
}

func (w *legacyMonitorTests) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*legacyMonitorTests) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

// EvaluateTestsFromConstructedIntervals produces the node junits, flaking rather
// than failing the tests in reducedTopologyFlakedTests when the cluster cannot
// keep a quorum through a node reboot.
func (w *legacyMonitorTests) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {

	clusterData, clusterDataErrs := platformidentification.BuildClusterData(ctx, w.adminRESTConfig)
	if clusterDataErrs != nil && len(*clusterDataErrs) > 0 {
		// Partial cluster data still drives useful tests, so this is a warning
		// rather than a failure.
		logrus.Warningf("legacy-node-monitor-tests: cluster data is incomplete: %s", utility.ErrorSummary(errors.Join(*clusterDataErrs...)))
	}
	if clusterData.Topology == "" {
		// BuildClusterData reports no topology at all when any of its unrelated
		// lookups fail, so prefer the value StartCollection already resolved.
		clusterData.Topology = w.topology
	}

	var junits []*junitapi.JUnitTestCase
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

	if w.reducedTopology {
		junits = ensureFlakeOnReducedTopology(junits, reducedTopologyFlakedTests)
	}

	return junits, nil
}

// reducedTopologyFlakedTests names the tests whose failures are expected on a
// control plane that loses quorum when a single node reboots.
var reducedTopologyFlakedTests = map[string]bool{
	"[sig-api-machinery] kube-apiserver terminates within graceful termination period": true,
	"[sig-node] overlapping apiserver process detected during kube-apiserver rollout":  true,
}

// ensureFlakeOnReducedTopology converts hard failures to flakes for tests expected
// to fail during disruptive recovery on DualReplica/SingleReplica topologies.
func ensureFlakeOnReducedTopology(junits []*junitapi.JUnitTestCase, flakedTests map[string]bool) []*junitapi.JUnitTestCase {
	failed := map[string]bool{}
	passed := map[string]bool{}
	for _, j := range junits {
		if j.FailureOutput != nil {
			failed[j.Name] = true
		} else {
			passed[j.Name] = true
		}
	}
	for name := range flakedTests {
		if failed[name] && !passed[name] {
			junits = append(junits, &junitapi.JUnitTestCase{Name: name})
		}
	}
	return junits
}

func (*legacyMonitorTests) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*legacyMonitorTests) Cleanup(ctx context.Context) error {
	return nil
}
