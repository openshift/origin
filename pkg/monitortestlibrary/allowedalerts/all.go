package allowedalerts

import (
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

// AllAlertTests returns the list of AlertTests with independent tests instead of relying on a backstop test.
// etcdAllowance can be the DefaultAllowances, but the quality of testing will be better if it is set.
// Some callers do not intend to run these tests (rather only to list alerts which have a test),
// in which case JobType can be an empty struct.
func AllAlertTests(jobType *platformidentification.JobType, clusterStability *monitortestframework.ClusterStabilityDuringTest, etcdAllowance AlertTestAllowanceCalculator) []AlertTest {

	ret := []AlertTest{}
	ret = append(ret, newWatchdogAlert(jobType, clusterStability))
	ret = append(ret, newAlertTestPerNamespace("KubePodNotReady", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTestPerNamespace("KubePodNotReady", jobType).firing().toTests()...)

	ret = append(ret, newAlertTest("bz-etcd", "etcdMembersDown", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdMembersDown", jobType).firing().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdGRPCRequestsSlow", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdGRPCRequestsSlow", jobType).firing().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdHighNumberOfFailedGRPCRequests", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdHighNumberOfFailedGRPCRequests", jobType).firing().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdMemberCommunicationSlow", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdMemberCommunicationSlow", jobType).firing().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdNoLeader", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdNoLeader", jobType).firing().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdHighFsyncDurations", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdHighFsyncDurations", jobType).firing().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdHighCommitDurations", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdHighCommitDurations", jobType).firing().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdInsufficientMembers", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-etcd", "etcdInsufficientMembers", jobType).firing().toTests()...)

	// A rare and pretty serious failure, should always be accompanied by other failures but we want to see a specific test failure for this.
	// It likely means a kubelet is down.
	ret = append(ret, newAlertTest(
		"sig-node", "TargetDown", jobType).inNamespace("kube-system").
		firing().alwaysFail().toTests()...)

	ret = append(ret, newAlertTest("bz-etcd", "etcdHighNumberOfLeaderChanges", jobType).pending().neverFail().toTests()...)

	// This test gets a little special treatment, if we're moving through etcd updates, we expect leader changes, so if this scenario is detected
	// this test is given fixed leeway for the alert to fire, otherwise it too falls back to historical data.
	ret = append(ret, newAlertTest("bz-etcd", "etcdHighNumberOfLeaderChanges", jobType).withAllowance(etcdAllowance).firing().toTests()...)

	ret = append(ret, newAlertTest("bz-kube-apiserver", "KubeAPIErrorBudgetBurn", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-kube-apiserver", "KubeAPIErrorBudgetBurn", jobType).firing().toTests()...)
	ret = append(ret, newAlertTest("bz-kube-apiserver", "KubeClientErrors", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-kube-apiserver", "KubeClientErrors", jobType).firing().toTests()...)

	ret = append(ret, newAlertTest("bz-storage", "KubePersistentVolumeErrors", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-storage", "KubePersistentVolumeErrors", jobType).firing().toTests()...)

	ret = append(ret, newAlertTest("bz-machine config operator", "MCDDrainError", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-machine config operator", "MCDDrainError", jobType).firing().toTests()...)

	ret = append(ret, newAlertTest("bz-single-node", "KubeMemoryOvercommit", jobType).pending().neverFail().toTests()...)
	// this appears to have no direct impact on the cluster in CI.  It's important in general, but for CI we're willing to run pretty hot.
	ret = append(ret, newAlertTest("bz-single-node", "KubeMemoryOvercommit", jobType).firing().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-machine config operator", "MCDPivotError", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-machine config operator", "MCDPivotError", jobType).firing().toTests()...)

	ret = append(ret, newAlertTest("bz-monitoring", "PrometheusOperatorWatchErrors", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-monitoring", "PrometheusOperatorWatchErrors", jobType).firing().toTests()...)

	ret = append(ret, newAlertTest("bz-networking", "OVNKubernetesResourceRetryFailure", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-networking", "OVNKubernetesResourceRetryFailure", jobType).firing().toTests()...)

	ret = append(ret, newAlertTest("bz-OLM", "RedhatOperatorsCatalogError", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-OLM", "RedhatOperatorsCatalogError", jobType).firing().toTests()...)

	ret = append(ret, newAlertTest("bz-storage", "VSphereOpenshiftNodeHealthFail", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-storage", "VSphereOpenshiftNodeHealthFail", jobType).firing().neverFail().toTests()...) // https://bugzilla.redhat.com/show_bug.cgi?id=2055729

	ret = append(ret, newAlertTest("bz-samples", "SamplesImagestreamImportFailing", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlertTest("bz-samples", "SamplesImagestreamImportFailing", jobType).firing().toTests()...)

	ret = append(ret, newAlertTest("bz-apiserver-auth", "PodSecurityViolation", jobType).firing().toTests()...)

	ret = append(ret, newAlertTest("bz-Cluster Version Operator", "ClusterOperatorDegraded", jobType).firing().alwaysFail().toTests()...)

	return ret
}
