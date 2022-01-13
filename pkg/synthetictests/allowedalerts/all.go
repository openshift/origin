package allowedalerts

func AllAlertTests() []AlertTest {
	return []AlertTest{
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
		newAlert("etcd", "etcdHighNumberOfLeaderChanges").firing(),

		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").pending().neverFail(),
		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").firing(),
		newAlert("kube-apiserver", "KubeClientErrors").pending().neverFail(),
		newAlert("kube-apiserver", "KubeClientErrors").firing(),

		newAlert("storage", "KubePersistentVolumeErrors").pending().neverFail(),
		newAlert("storage", "KubePersistentVolumeErrors").firing(),

		newAlert("machine config operator", "MCDDrainError").pending().neverFail(),
		newAlert("machine config operator", "MCDDrainError").firing(),
	}
}
