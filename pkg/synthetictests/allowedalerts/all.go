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

		newAlert("Cloud Credential Operator", "CloudCredentialOperatorProvisioningFailed").pending().neverFail(),
		newAlert("Cloud Credential Operator", "CloudCredentialOperatorProvisioningFailed").firing(),

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

		// controle plane owns most aggregated apiservers.  Not perfect, but closer than random.
		newAlert("kube-apiserver", "AggregatedAPIDown").pending().neverFail(),
		newAlert("kube-apiserver", "AggregatedAPIDown").firing(),
		// controle plane owns most aggregated apiservers.  Not perfect, but closer than random.
		newAlert("kube-apiserver", "KubeAggregatedAPIDown").pending().neverFail(),
		newAlert("kube-apiserver", "KubeAggregatedAPIDown").firing(),
		// controle plane owns most aggregated apiservers.  Not perfect, but closer than random.
		newAlert("kube-apiserver", "KubeAggregatedAPIErrors").pending().neverFail(),
		newAlert("kube-apiserver", "KubeAggregatedAPIErrors").firing(),
		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").pending().neverFail(),
		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").firing(),
		newAlert("kube-apiserver", "KubeClientErrors").pending().neverFail(),
		newAlert("kube-apiserver", "KubeClientErrors").firing(),
		newAlert("kube-apiserver", "KubeAPITerminatedRequests").pending().neverFail(),
		newAlert("kube-apiserver", "KubeAPITerminatedRequests").firing(),
		// control plane owns most of the kube pods.  This isn't perfect, but it's slightly closer than random.
		newAlert("kube-apiserver", "KubePodCrashLooping").pending().neverFail(),
		newAlert("kube-apiserver", "KubePodCrashLooping").firing(),
		newAlert("kube-apiserver", "KubeAPIDown").pending().neverFail(),
		newAlert("kube-apiserver", "KubeAPIDown").firing(),

		newAlert("kube-scheduler", "KubeSchedulerDown").pending().neverFail(),
		newAlert("kube-scheduler", "KubeSchedulerDown").firing(),

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

		newAlert("Networking", "SDNPodNotReady").pending().neverFail(),
		newAlert("Networking", "SDNPodNotReady").firing(),
		newAlert("Networking", "NoRunningOvnMaster").pending().neverFail(),
		newAlert("Networking", "NoRunningOvnMaster").firing(),

		newAlert("Node", "KubeletDown").pending().neverFail(),
		newAlert("Node", "KubeletDown").firing(),
		newAlert("Node", "KubeNodeNotReady").pending().neverFail(),
		newAlert("Node", "KubeNodeNotReady").firing(),
		newAlert("Node", "NodeProxyApplySlow").pending().neverFail(),
		newAlert("Node", "NodeProxyApplySlow").firing(),
		newAlert("Node", "NTOPodsNotReady").pending().neverFail(),
		newAlert("Node", "NTOPodsNotReady").firing(),

		newAlert("OLM", "CommunityOperatorsCatalogError").pending().neverFail(),
		newAlert("OLM", "CommunityOperatorsCatalogError").firing(),
		newAlert("OLM", "RedhatMarketplaceCatalogError").pending().neverFail(),
		newAlert("OLM", "RedhatMarketplaceCatalogError").firing(),
		newAlert("OLM", "CertifiedOperatorsCatalogError").pending().neverFail(),
		newAlert("OLM", "CertifiedOperatorsCatalogError").firing(),
		newAlert("OLM", "RedhatOperatorsCatalogError").pending().neverFail(),
		newAlert("OLM", "RedhatOperatorsCatalogError").firing(),

		newAlert("Routing", "IngressControllerDegraded").pending().neverFail(),
		newAlert("Routing", "IngressControllerDegraded").firing(),

		newAlert("storage", "KubePersistentVolumeErrors").pending().neverFail(),
		newAlert("storage", "KubePersistentVolumeErrors").firing(),
		newAlert("storage", "VSphereOpenshiftNodeHealthFail").pending().neverFail(),
		newAlert("storage", "VSphereOpenshiftNodeHealthFail").firing().neverFail(), // https://bugzilla.redhat.com/show_bug.cgi?id=2055729

		// need to split by name
		newAlert("Unknown", "ClusterOperatorDown").pending().neverFail(),
		newAlert("Unknown", "ClusterOperatorDown").firing(),
		newAlert("Unknown", "KubeContainerWaiting").pending().neverFail(),
		newAlert("Unknown", "KubeContainerWaiting").firing(),
		// This isn't perfect, but it's slightly closer than random.  To change ownership, provide evidence of the top
		// three ways this test fails.
		newAlert("Unknown", "KubeDeploymentReplicasMismatch").pending().neverFail(),
		newAlert("Unknown", "KubeDeploymentReplicasMismatch").firing(),
		// this is usually down to a scheduler problem on node restart.  This isn't perfect, but it's slightly closer than random.
		newAlert("Unknown", "KubePodNotReady").pending().neverFail(),
		newAlert("Unknown", "KubePodNotReady").firing(),
		newAlert("Unknown", "KubeStatefulSetReplicasMismatch").pending().neverFail(),
		newAlert("Unknown", "KubeStatefulSetReplicasMismatch").firing(),
		newAlert("Unknown", "ClusterOperatorDegraded").pending().neverFail(),
		newAlert("Unknown", "ClusterOperatorDegraded").firing(),
		newAlert("Unknown", "KubeJobFailed").pending().neverFail(),
		newAlert("Unknown", "KubeJobFailed").firing(),
	}
}
