package allowedalerts

import (
	"context"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// AllAlertTests returns the list of AlertTests with independent tests instead of a general check.
// clientConfig may be nil, but the quality of the allowances will be better if it is set.
// You may choose to send nil to get a list of names for instance.
func AllAlertTests(ctx context.Context, clientConfig *rest.Config, duration time.Duration) []AlertTest {
	var etcdAllowance AlertTestAllowanceCalculator
	etcdAllowance = defaultAllowances

	// if we have a clientConfig,  use it.
	if clientConfig != nil {
		kubeClient, err := kubernetes.NewForConfig(clientConfig)
		if err != nil {
			panic(err)
		}
		etcdAllowance, err = NewAllowedWhenEtcdRevisionChange(ctx, kubeClient, duration)
		if err != nil {
			panic(err)
		}
	}

	ret := []AlertTest{}
	ret = append(ret, newWatchdogAlert())
	ret = append(ret, newNamespacedAlert("KubePodNotReady").pending().neverFail().toTests()...)
	ret = append(ret, newNamespacedAlert("KubePodNotReady").firing().toTests()...)

	ret = append(ret, newAlert("etcd", "etcdMembersDown").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdMembersDown").firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdGRPCRequestsSlow").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdGRPCRequestsSlow").firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighNumberOfFailedGRPCRequests").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighNumberOfFailedGRPCRequests").firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdMemberCommunicationSlow").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdMemberCommunicationSlow").firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdNoLeader").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdNoLeader").firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighFsyncDurations").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighFsyncDurations").firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighCommitDurations").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighCommitDurations").firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdInsufficientMembers").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdInsufficientMembers").firing().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighNumberOfLeaderChanges").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("etcd", "etcdHighNumberOfLeaderChanges").withAllowance(etcdAllowance).firing().toTests()...)

	ret = append(ret, newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").firing().toTests()...)
	ret = append(ret, newAlert("kube-apiserver", "KubeClientErrors").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("kube-apiserver", "KubeClientErrors").firing().toTests()...)

	ret = append(ret, newAlert("storage", "KubePersistentVolumeErrors").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("storage", "KubePersistentVolumeErrors").firing().toTests()...)

	ret = append(ret, newAlert("machine config operator", "MCDDrainError").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("machine config operator", "MCDDrainError").firing().toTests()...)

	ret = append(ret, newAlert("machine config operator", "MCDPivotError").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("machine config operator", "MCDPivotError").firing().toTests()...)

	ret = append(ret, newAlert("monitoring", "PrometheusOperatorWatchErrors").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("monitoring", "PrometheusOperatorWatchErrors").firing().toTests()...)

	ret = append(ret, newAlert("OLM", "RedhatOperatorsCatalogError").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("OLM", "RedhatOperatorsCatalogError").firing().toTests()...)

	ret = append(ret, newAlert("storage", "VSphereOpenshiftNodeHealthFail").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("storage", "VSphereOpenshiftNodeHealthFail").firing().neverFail().toTests()...) // https://bugzilla.redhat.com/show_bug.cgi?id=2055729

	ret = append(ret, newAlert("samples", "SamplesImagestreamImportFailing").pending().neverFail().toTests()...)
	ret = append(ret, newAlert("samples", "SamplesImagestreamImportFailing").firing().toTests()...)

	return ret
}
