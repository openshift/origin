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

	return []AlertTest{
		newWatchdogAlert(),

		newAlert("Cluster Version Operator", "ClusterVersionOperatorDown").pending().neverFail(),
		newAlert("Cluster Version Operator", "ClusterVersionOperatorDown").firing(),

		newAlert("DNS", "CoreDNSErrorsHigh").pending().neverFail(),
		newAlert("DNS", "CoreDNSErrorsHigh").firing(),

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

		// controle plane owns most aggregated APIsserver.  Not perfect, but closer than random.
		newAlert("kube-apiserver", "AggregatedAPIDown").pending().neverFail(),
		newAlert("kube-apiserver", "AggregatedAPIDown").firing(),
		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").pending().neverFail(),
		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").firing(),
		newAlert("kube-apiserver", "KubeClientErrors").pending().neverFail(),
		newAlert("kube-apiserver", "KubeClientErrors").firing(),
		// control plane owns most of the kube pods.  This isn't perfect, but it's slightly closer than random.
		newAlert("kube-apiserver", "KubePodCrashLooping").pending().neverFail(),
		newAlert("kube-apiserver", "KubePodCrashLooping").firing(),

		newAlert("machine config operator", "MCDDrainError").pending().neverFail(),
		newAlert("machine config operator", "MCDDrainError").firing(),

		newAlert("monitoring", "AlertmanagerClusterDown").pending().neverFail(),
		newAlert("monitoring", "AlertmanagerClusterDown").firing(),
		newAlert("monitoring", "PrometheusOperatorWatchErrors").pending().neverFail(),
		newAlert("monitoring", "PrometheusOperatorWatchErrors").firing(),
		newAlert("monitoring", "PrometheusErrorSendingAlertsToSomeAlertmanagers").pending().neverFail(),
		newAlert("monitoring", "PrometheusErrorSendingAlertsToSomeAlertmanagers").firing(),
		newAlert("monitoring", "PrometheusOperatorListErrors").pending().neverFail(),
		newAlert("monitoring", "PrometheusOperatorListErrors").firing(),
		// this isn't perfect, but if the monitoring team can localize the bz components from the firing alerts, it should be possible to do better.
		newAlert("monitoring", "TargetDown").pending().neverFail(),
		newAlert("monitoring", "TargetDown").firing(),

		newAlert("Node", "KubeletDown").pending().neverFail(),
		newAlert("Node", "KubeletDown").firing(),

		newAlert("Routing", "IngressControllerDegraded").pending().neverFail(),
		newAlert("Routing", "IngressControllerDegraded").firing(),

		newAlert("storage", "KubePersistentVolumeErrors").pending().neverFail(),
		newAlert("storage", "KubePersistentVolumeErrors").firing(),
		newAlert("storage", "VSphereOpenshiftNodeHealthFail").pending().neverFail(),
		newAlert("storage", "VSphereOpenshiftNodeHealthFail").firing().neverFail(), // https://bugzilla.redhat.com/show_bug.cgi?id=2055729
	}
}
