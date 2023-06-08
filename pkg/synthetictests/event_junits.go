package synthetictests

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/alerts"
	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/client-go/rest"
)

// StableSystemEventInvariants are invariants that should hold true when a cluster is in
// steady state (not being changed externally). Use these with suites that assume the
// cluster is under no adversarial change (config changes, induced disruption to nodes,
// etcd, or apis).
func StableSystemEventInvariants(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string, recordedResource *monitorapi.ResourcesMap) (tests []*junitapi.JUnitTestCase) {

	jobType, err := platformidentification.GetJobType(context.TODO(), kubeClientConfig)
	if err != nil {
		// JobType will be nil here, but we want test cases to all fail if this is the case, so we rely on them to nil check
		logrus.WithError(err).Warn("ERROR: unable to determine job type for alert testing, jobType will be nil")
	}

	tests = SystemEventInvariants(events, duration, kubeClientConfig, testSuite, recordedResource)
	tests = append(tests, testContainerFailures(events)...)
	tests = append(tests, testDeleteGracePeriodZero(events)...)
	tests = append(tests, testKubeApiserverProcessOverlap(events)...)
	tests = append(tests, testKubeAPIServerGracefulTermination(events)...)
	tests = append(tests, testKubeletToAPIServerGracefulTermination(events)...)
	tests = append(tests, testPodTransitions(events)...)
	tests = append(tests, testPodSandboxCreation(events, kubeClientConfig)...)
	tests = append(tests, testOvnNodeReadinessProbe(events, kubeClientConfig)...)

	tests = append(tests, TestAllAPIBackendsForDisruption(events, duration, jobType)...)
	tests = append(tests, TestAllIngressBackendsForDisruption(events, duration, jobType)...)
	tests = append(tests, TestExternalBackendsForDisruption(events, duration, jobType)...)
	tests = append(tests, TestAPIServerIPTablesAccessDisruption(events)...)

	tests = append(tests, testMultipleSingleSecondDisruptions(events)...)
	tests = append(tests, testStableSystemOperatorStateTransitions(events)...)
	tests = append(tests, testDuplicatedEventForStableSystem(events, kubeClientConfig, testSuite)...)
	tests = append(tests, testStaticPodLifecycleFailure(events, kubeClientConfig, testSuite)...)
	tests = append(tests, testErrImagePullConnTimeoutOpenShiftNamespaces(events)...)
	tests = append(tests, testErrImagePullConnTimeout(events)...)
	tests = append(tests, testErrImagePullQPSExceededOpenShiftNamespaces(events)...)
	tests = append(tests, testErrImagePullQPSExceeded(events)...)
	tests = append(tests, testErrImagePullManifestUnknownOpenShiftNamespaces(events)...)
	tests = append(tests, testErrImagePullManifestUnknown(events)...)
	tests = append(tests, testErrImagePullGenericOpenShiftNamespaces(events)...)
	tests = append(tests, testErrImagePullGeneric(events)...)
	tests = append(tests, testAlerts(events, alerts.AllowedAlertsDuringConformance, jobType,
		kubeClientConfig, duration, recordedResource)...)
	tests = append(tests, testOperatorOSUpdateStaged(events, kubeClientConfig)...)
	tests = append(tests, testOperatorOSUpdateStartedEventRecorded(events, kubeClientConfig)...)
	tests = append(tests, testPodNodeNameIsImmutable(events)...)
	tests = append(tests, testBackoffPullingRegistryRedhatImage(events)...)
	tests = append(tests, testRequiredInstallerResourcesMissing(events)...)
	tests = append(tests, testBackoffStartingFailedContainer(events)...)
	tests = append(tests, testBackoffStartingFailedContainerForE2ENamespaces(events)...)
	tests = append(tests, testAPIQuotaEvents(events)...)
	tests = append(tests, testErrorUpdatingEndpointSlices(events)...)
	tests = append(tests, testConfigOperatorReadinessProbe(events)...)
	tests = append(tests, testConfigOperatorProbeErrorReadinessProbe(events)...)
	tests = append(tests, testConfigOperatorProbeErrorLivenessProbe(events)...)
	tests = append(tests, testOauthApiserverProbeErrorReadiness(events)...)
	tests = append(tests, testNoOVSVswitchdUnreasonablyLongPollIntervals(events)...)
	tests = append(tests, testOauthApiserverProbeErrorLiveness(events)...)
	tests = append(tests, testOauthApiserverProbeErrorConnectionRefused(events)...)
	tests = append(tests, testNodeHasNoDiskPressure(events)...)
	tests = append(tests, testNodeHasSufficientMemory(events)...)
	tests = append(tests, testNodeHasSufficientPID(events)...)

	tests = append(tests, testFailedToDeleteCGroupsPath(events)...)
	tests = append(tests, testAnonymousCertConnectionFailure(events)...)
	tests = append(tests, testHttpConnectionLost(events)...)
	tests = append(tests, testMarketplaceStartupProbeFailure(events)...)
	tests = append(tests, testErrImagePullUnrecognizedSignatureFormat(events)...)
	tests = append(tests, testEtcdShouldNotLogSlowFdataSyncs(events)...)
	tests = append(tests, testEtcdShouldNotLogDroppedRaftMessages(events)...)
	tests = append(tests, testDNSOverlapDisruption(events)...)
	return tests
}

// SystemUpgradeEventInvariants are invariants tested against events that should hold true in a cluster
// that is being upgraded without induced disruption
func SystemUpgradeEventInvariants(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string, recordedResource *monitorapi.ResourcesMap) (tests []*junitapi.JUnitTestCase) {

	// JobType will be nil here, but we want test cases to all fail if this is the case, so we rely on them to nil check
	jobType, err := platformidentification.GetJobType(context.TODO(), kubeClientConfig)
	if err != nil {
		logrus.WithError(err).Warn("ERROR: unable to determine job type for alert testing, jobType will be nil")
	}

	tests = SystemEventInvariants(events, duration, kubeClientConfig, testSuite, recordedResource)
	tests = append(tests, testContainerFailures(events)...)
	tests = append(tests, testDeleteGracePeriodZero(events)...)
	tests = append(tests, testKubeApiserverProcessOverlap(events)...)
	tests = append(tests, testKubeAPIServerGracefulTermination(events)...)
	tests = append(tests, testKubeletToAPIServerGracefulTermination(events)...)
	tests = append(tests, testPodTransitions(events)...)
	tests = append(tests, testPodSandboxCreation(events, kubeClientConfig)...)
	tests = append(tests, testOvnNodeReadinessProbe(events, kubeClientConfig)...)
	tests = append(tests, testNodeUpgradeTransitions(events, kubeClientConfig)...)
	tests = append(tests, testUpgradeOperatorStateTransitions(events)...)
	tests = append(tests, testDuplicatedEventForUpgrade(events, kubeClientConfig, testSuite)...)
	tests = append(tests, testStaticPodLifecycleFailure(events, kubeClientConfig, testSuite)...)
	tests = append(tests, testErrImagePullConnTimeoutOpenShiftNamespaces(events)...)
	tests = append(tests, testErrImagePullConnTimeout(events)...)
	tests = append(tests, testErrImagePullQPSExceededOpenShiftNamespaces(events)...)
	tests = append(tests, testErrImagePullQPSExceeded(events)...)
	tests = append(tests, testErrImagePullManifestUnknownOpenShiftNamespaces(events)...)
	tests = append(tests, testErrImagePullManifestUnknown(events)...)
	tests = append(tests, testErrImagePullGenericOpenShiftNamespaces(events)...)
	tests = append(tests, testErrImagePullGeneric(events)...)
	tests = append(tests, testAlerts(events, alerts.AllowedAlertsDuringUpgrade, jobType,
		kubeClientConfig, duration, recordedResource)...)
	tests = append(tests, testOperatorOSUpdateStaged(events, kubeClientConfig)...)
	tests = append(tests, testOperatorOSUpdateStartedEventRecorded(events, kubeClientConfig)...)
	tests = append(tests, testPodNodeNameIsImmutable(events)...)
	tests = append(tests, testBackoffPullingRegistryRedhatImage(events)...)
	tests = append(tests, testRequiredInstallerResourcesMissing(events)...)
	tests = append(tests, testBackoffStartingFailedContainer(events)...)
	tests = append(tests, testBackoffStartingFailedContainerForE2ENamespaces(events)...)
	tests = append(tests, testAPIQuotaEvents(events)...)
	tests = append(tests, testErrorUpdatingEndpointSlices(events)...)

	tests = append(tests, TestAllAPIBackendsForDisruption(events, duration, jobType)...)
	tests = append(tests, TestAllIngressBackendsForDisruption(events, duration, jobType)...)
	tests = append(tests, TestExternalBackendsForDisruption(events, duration, jobType)...)
	tests = append(tests, testMultipleSingleSecondDisruptions(events)...)
	tests = append(tests, testNoDNSLookupErrorsInDisruptionSamplers(events)...)
	tests = append(tests, testNoOVSVswitchdUnreasonablyLongPollIntervals(events)...)
	tests = append(tests, TestAPIServerIPTablesAccessDisruption(events)...)

	tests = append(tests, testNoExcessiveSecretGrowthDuringUpgrade()...)
	tests = append(tests, testNoExcessiveConfigMapGrowthDuringUpgrade()...)
	tests = append(tests, testConfigOperatorReadinessProbe(events)...)
	tests = append(tests, testConfigOperatorProbeErrorReadinessProbe(events)...)
	tests = append(tests, testConfigOperatorProbeErrorLivenessProbe(events)...)
	tests = append(tests, testOauthApiserverProbeErrorReadiness(events)...)
	tests = append(tests, testOauthApiserverProbeErrorLiveness(events)...)
	tests = append(tests, testOauthApiserverProbeErrorConnectionRefused(events)...)
	tests = append(tests, testNodeHasNoDiskPressure(events)...)
	tests = append(tests, testNodeHasSufficientMemory(events)...)
	tests = append(tests, testNodeHasSufficientPID(events)...)
	tests = append(tests, testErrorReconcilingNode(events)...)
	tests = append(tests, testFailedScheduling(events)...)
	tests = append(tests, testOperatorStatusChanged(events)...)

	tests = append(tests, testAnonymousCertConnectionFailure(events)...)
	tests = append(tests, testFailedToDeleteCGroupsPath(events)...)
	tests = append(tests, testHttpConnectionLost(events)...)
	tests = append(tests, testMarketplaceStartupProbeFailure(events)...)
	tests = append(tests, testErrImagePullUnrecognizedSignatureFormat(events)...)
	tests = append(tests, testEtcdShouldNotLogSlowFdataSyncs(events)...)
	tests = append(tests, testEtcdShouldNotLogDroppedRaftMessages(events)...)
	tests = append(tests, testMasterNodesUpdated(events)...)
	tests = append(tests, testDNSOverlapDisruption(events)...)
	return tests
}

// SystemEventInvariants are invariants tested against events that should hold true in any cluster,
// even one undergoing disruption. These are usually focused on things that must be true on a single
// machine, even if the machine crashes.
func SystemEventInvariants(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string, _ *monitorapi.ResourcesMap) (tests []*junitapi.JUnitTestCase) {
	tests = append(tests, testSystemDTimeout(events)...)
	tests = append(tests, testPodIPReuse(events)...)
	return tests
}
