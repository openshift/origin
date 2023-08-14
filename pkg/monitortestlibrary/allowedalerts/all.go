package allowedalerts

import (
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

// AllAlertTests returns the list of AlertTests with independent tests instead of relying on a backstop test.
// etcdAllowance can be the DefaultAllowances, but the quality of testing will be better if it is set.
// Some callers do not intend to run these tests (rather only to list alerts which have a test),
// in which case JobType can be an empty struct.
func AllAlertTests(jobType *platformidentification.JobType, etcdAllowance AlertTestAllowanceCalculator) []AlertTest {

	ret := []AlertTest{}
	ret = append(ret, newWatchdogAlert(jobType))
	ret = append(ret, newNamespacedAlert("KubePodNotReady", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newNamespacedAlert("KubePodNotReady", jobType).firing().toTests()...)

	ret = append(ret, newAlert("etcd", "etcdMembersDown", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdMembersDown", jobType).firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdGRPCRequestsSlow", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdGRPCRequestsSlow", jobType).firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighNumberOfFailedGRPCRequests", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighNumberOfFailedGRPCRequests", jobType).firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdMemberCommunicationSlow", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdMemberCommunicationSlow", jobType).firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdNoLeader", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdNoLeader", jobType).firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighFsyncDurations", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighFsyncDurations", jobType).firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighCommitDurations", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighCommitDurations", jobType).firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdInsufficientMembers", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdInsufficientMembers", jobType).firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighNumberOfLeaderChanges", jobType).pending().neverFail().toTests()...)

	// This test gets a little special treatment, if we're moving through etcd updates, we expect leader changes, so if this scenario is detected
	// this test is given fixed leeway for the alert to fire, otherwise it too falls back to historical data.
	ret = append(ret, newAlert("etcd", "etcdHighNumberOfLeaderChanges", jobType).withAllowance(etcdAllowance).firing().toTests()...)

	ret = append(ret, newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn", jobType).firing().toTests()...)
	ret = append(ret, newAlert("kube-apiserver", "KubeClientErrors", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("kube-apiserver", "KubeClientErrors", jobType).firing().toTests()...)

	ret = append(ret, newAlert("storage", "KubePersistentVolumeErrors", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("storage", "KubePersistentVolumeErrors", jobType).firing().toTests()...)

	ret = append(ret, newAlert("machine config operator", "MCDDrainError", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("machine config operator", "MCDDrainError", jobType).firing().toTests()...)

	ret = append(ret, newAlert("single-node", "KubeMemoryOvercommit", jobType).pending().neverFail().toTests()...)
	// this appears to have no direct impact on the cluster in CI.  It's important in general, but for CI we're willing to run pretty hot.
	ret = append(ret, newAlert("single-node", "KubeMemoryOvercommit", jobType).firing().neverFail().toTests()...)
	ret = append(ret, newAlert("machine config operator", "MCDPivotError", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("machine config operator", "MCDPivotError", jobType).firing().toTests()...)

	ret = append(ret, newAlert("monitoring", "PrometheusOperatorWatchErrors", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("monitoring", "PrometheusOperatorWatchErrors", jobType).firing().toTests()...)

	ret = append(ret, newAlert("networking", "OVNKubernetesResourceRetryFailure", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("networking", "OVNKubernetesResourceRetryFailure", jobType).firing().toTests()...)

	ret = append(ret, newAlert("OLM", "RedhatOperatorsCatalogError", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("OLM", "RedhatOperatorsCatalogError", jobType).firing().toTests()...)

	ret = append(ret, newAlert("storage", "VSphereOpenshiftNodeHealthFail", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("storage", "VSphereOpenshiftNodeHealthFail", jobType).firing().neverFail().toTests()...) // https://bugzilla.redhat.com/show_bug.cgi?id=2055729

	ret = append(ret, newAlert("samples", "SamplesImagestreamImportFailing", jobType).pending().neverFail().toTests()...)
	ret = append(ret, newAlert("samples", "SamplesImagestreamImportFailing", jobType).firing().toTests()...)

	return ret
}
