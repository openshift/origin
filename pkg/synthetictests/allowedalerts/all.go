package allowedalerts

func AllAlertTests() []AlertTest {
	return []AlertTest{
		newWatchdogAlert(),

		newAlert("etcd", "etcdMembersDown").pending().neverFail(),
		newAlert("etcd", "etcdMembersDown").info(),
		newAlert("etcd", "etcdGRPCRequestsSlow").pending().neverFail(),
		newAlert("etcd", "etcdGRPCRequestsSlow").info(),
		newAlert("etcd", "etcdHighNumberOfFailedGRPCRequests").pending().neverFail(),
		newAlert("etcd", "etcdHighNumberOfFailedGRPCRequests").info(),
		newAlert("etcd", "etcdMemberCommunicationSlow").pending().neverFail(),
		newAlert("etcd", "etcdMemberCommunicationSlow").info(),
		newAlert("etcd", "etcdNoLeader").pending().neverFail(),
		newAlert("etcd", "etcdNoLeader").info(),
		newAlert("etcd", "etcdHighFsyncDurations").pending().neverFail(),
		newAlert("etcd", "etcdHighFsyncDurations").info(),
		newAlert("etcd", "etcdHighCommitDurations").pending().neverFail(),
		newAlert("etcd", "etcdHighCommitDurations").info(),
		newAlert("etcd", "etcdInsufficientMembers").pending().neverFail(),
		newAlert("etcd", "etcdInsufficientMembers").info(),
		newAlert("etcd", "etcdHighNumberOfLeaderChanges").pending().neverFail(),
		newAlert("etcd", "etcdHighNumberOfLeaderChanges").info(),

		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").pending().neverFail(),
		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").info().neverFail(), // https://bugzilla.redhat.com/show_bug.cgi?id=2039539
		newAlert("kube-apiserver", "KubeClientErrors").pending().neverFail(),
		newAlert("kube-apiserver", "KubeClientErrors").info(),

		newAlert("storage", "KubePersistentVolumeErrors").pending().neverFail(),
		newAlert("storage", "KubePersistentVolumeErrors").info(),

		newAlert("machine config operator", "MCDDrainError").pending().neverFail(),
		newAlert("machine config operator", "MCDDrainError").info(),

		newAlert("monitoring", "PrometheusOperatorWatchErrors").pending().neverFail(),
		newAlert("monitoring", "PrometheusOperatorWatchErrors").info(),
	}
}
