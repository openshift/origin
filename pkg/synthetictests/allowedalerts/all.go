package allowedalerts

import (
	"context"

	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"k8s.io/client-go/rest"
)

// AllAlertTests returns the list of AlertTests with independent tests instead of a general check.
// clientConfig may be nil, but the quality of the allowances will be better if it is set.
// You may choose to send nil to get a list of names for instance.
func AllAlertTests(ctx context.Context, clientConfig *rest.Config) []AlertTest {
	var etcdAllowance AlertTestAllowanceCalculator
	etcdAllowance = defaultAllowances

	// if we have a clientConfig,  use it.
	if clientConfig != nil {
		operatorClient, err := operatorv1client.NewForConfig(clientConfig)
		if err != nil {
			panic(err)
		}
		etcdAllowance, err = NewAllowedWhenEtcdRevisionChange(ctx, operatorClient)
		if err != nil {
			panic(err)
		}
	}

	return []AlertTest{
		newWatchdogAlert(),

		newAlert("etcd", "etcdMembersDown").pending().neverFail(),
		newAlert("etcd", "etcdMembersDown").firing(),
		newAlert("etcd", "etcdGRPCRequestsSlow").pending().neverFail(),
		newAlert("etcd", "etcdGRPCRequestsSlow").firing(),
		newAlert("etcd", "etcdHighNumberOfFailedGRPCRequests").pending().neverFail(),
		newAlert("etcd", "etcdHighNumberOfFailedGRPCRequests").firing(),
		newAlert("etcd", "etcdMemberCommunicationSlow").pending().neverFail(),
		newAlert("etcd", "etcdMemberCommunicationSlow").firing(),
		newAlert("etcd", "etcdNoLeader").pending().neverFail(),
		newAlert("etcd", "etcdNoLeader").firing(),
		newAlert("etcd", "etcdHighFsyncDurations").pending().neverFail(),
		newAlert("etcd", "etcdHighFsyncDurations").firing(),
		newAlert("etcd", "etcdHighCommitDurations").pending().neverFail(),
		newAlert("etcd", "etcdHighCommitDurations").firing(),
		newAlert("etcd", "etcdInsufficientMembers").pending().neverFail(),
		newAlert("etcd", "etcdInsufficientMembers").firing(),
		newAlert("etcd", "etcdHighNumberOfLeaderChanges").pending().neverFail(),
		newAlert("etcd", "etcdHighNumberOfLeaderChanges").withAllowance(etcdAllowance).firing(),

		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").pending().neverFail(),
		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").firing(),
		newAlert("kube-apiserver", "KubeClientErrors").pending().neverFail(),
		newAlert("kube-apiserver", "KubeClientErrors").firing(),

		newAlert("storage", "KubePersistentVolumeErrors").pending().neverFail(),
		newAlert("storage", "KubePersistentVolumeErrors").firing(),

		newAlert("machine config operator", "MCDDrainError").pending().neverFail(),
		newAlert("machine config operator", "MCDDrainError").firing(),

		newAlert("monitoring", "PrometheusOperatorWatchErrors").pending().neverFail(),
		newAlert("monitoring", "PrometheusOperatorWatchErrors").firing(),

		newAlert("storage", "VSphereOpenshiftNodeHealthFail").pending().neverFail(),
		newAlert("storage", "VSphereOpenshiftNodeHealthFail").firing().neverFail(), // https://bugzilla.redhat.com/show_bug.cgi?id=2055729
	}
}
