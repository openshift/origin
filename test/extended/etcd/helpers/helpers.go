package helpers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"github.com/pkg/errors"

	configv1 "github.com/openshift/api/config/v1"
	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	machinev1client "github.com/openshift/client-go/machine/clientset/versioned/typed/machine/v1"
	machinev1beta1client "github.com/openshift/client-go/machine/clientset/versioned/typed/machine/v1beta1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"

	bmhelper "github.com/openshift/origin/test/extended/baremetal"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/utils/pointer"
)

const masterMachineLabelSelector = "machine.openshift.io/cluster-api-machine-role" + "=" + "master"
const machineDeletionHookName = "EtcdQuorumOperator"
const machineDeletionHookOwner = "clusteroperator/etcd"
const masterNodeRoleLabel = "node-role.kubernetes.io/master"

type TestingT interface {
	Logf(format string, args ...interface{})
}

// AnyRunningMasterMachine finds and returns a running master machine from the cluster.
func AnyRunningMasterMachine(ctx context.Context, t TestingT, machineClient machinev1beta1client.MachineInterface) (*machinev1beta1.Machine, error) {
	machineList, err := machineClient.List(ctx, metav1.ListOptions{LabelSelector: masterMachineLabelSelector})
	if err != nil {
		return nil, fmt.Errorf("unable to list master machines: %w", err)
	}

	for _, machine := range machineList.Items {
		machinePhase := pointer.StringDeref(machine.Status.Phase, "Unknown")
		if machinePhase == "Running" {
			return &machine, nil
		}
		t.Logf("%q machine is in unexpected %q state", machine.Name, machinePhase)
	}
	return nil, fmt.Errorf("no running master machines found")
}

// CreateNewMasterMachine creates a new master node by cloning an existing Machine resource.
// If templateMachine is provided, it will be cloned. Otherwise a running machine will be picked for cloning.
func CreateNewMasterMachine(ctx context.Context, t TestingT, machineClient machinev1beta1client.MachineInterface, templateMachine *machinev1beta1.Machine) (string, error) {
	var machineToClone *machinev1beta1.Machine
	var err error

	//if templateMachine is not provided, get a running master machine to clone
	if templateMachine == nil {
		machineToClone, err = AnyRunningMasterMachine(ctx, t, machineClient)
		if err != nil {
			return "", err
		}

		// assigning a new Name and clearing ProviderID is enough
		// for MAO to pick it up and provision a new master machine/node
		machineToClone.Name = fmt.Sprintf("%s-clone", machineToClone.Name)
	} else {
		//if templateMachine is provided, form the new machine name using the same clusterIdRole and index
		machineToClone = templateMachine.DeepCopy()
		machineIndex := machineToClone.Name[strings.LastIndex(machineToClone.Name, "-")+1:]
		clusterId := machineToClone.ObjectMeta.Labels[machinev1beta1.MachineClusterIDLabel]
		machineRole := machineToClone.ObjectMeta.Labels["machine.openshift.io/cluster-api-machine-role"]
		machineToClone.Name = fmt.Sprintf("%s-%s-%s-%s", clusterId, machineRole, rand.String(5), machineIndex)
	}

	machineToClone.Spec.ProviderID = nil
	machineToClone.ResourceVersion = ""
	machineToClone.Annotations = map[string]string{}
	machineToClone.Spec.LifecycleHooks = machinev1beta1.LifecycleHooks{}

	clonedMachine, err := machineClient.Create(ctx, machineToClone, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	t.Logf("Created a new master machine/node %q", clonedMachine.Name)
	return clonedMachine.Name, nil
}

func EnsureMasterMachine(ctx context.Context, t TestingT, machineName string, machineClient machinev1beta1client.MachineInterface) error {
	waitPollInterval := 15 * time.Second
	// This timeout should be tuned for the platform that takes the longest to provision a node and result
	// in a Running machine phase.
	waitPollTimeout := 25 * time.Minute
	t.Logf("Waiting up to %s for %q machine to be in the Running state", waitPollTimeout.String(), machineName)

	return wait.Poll(waitPollInterval, waitPollTimeout, func() (bool, error) {
		machine, err := machineClient.Get(ctx, machineName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		machinePhase := pointer.StringDeref(machine.Status.Phase, "Unknown")
		t.Logf("%q machine is in %q state", machineName, machinePhase)
		if machinePhase != "Running" {
			return false, nil
		}
		if !hasMachineDeletionHook(machine) {
			// it takes some time to add the hook
			t.Logf("%q machine doesn't have required deletion hooks", machine.Name)
			return false, nil
		}
		return true, nil
	})
}

// EnsureInitialClusterState makes sure the cluster state is expected, that is, has only 3 running machines and exactly 3 voting members
// otherwise it attempts to recover the cluster by removing any excessive machines
func EnsureInitialClusterState(ctx context.Context, t TestingT, etcdClientFactory EtcdClientCreator, machineClient machinev1beta1client.MachineInterface, kubeClient kubernetes.Interface) error {
	if err := recoverClusterToInitialStateIfNeeded(ctx, t, machineClient); err != nil {
		return err
	}
	if err := EnsureVotingMembersCount(ctx, t, etcdClientFactory, kubeClient, 3); err != nil {
		return err
	}
	return EnsureMasterMachinesAndCount(ctx, t, 3, machineClient)
}

// EnsureMasterMachinesAndCount checks if there are only given number of running master machines otherwise it returns an error
func EnsureMasterMachinesAndCount(ctx context.Context, t TestingT, expectedCount int, machineClient machinev1beta1client.MachineInterface) error {
	waitPollInterval := 15 * time.Second
	waitPollTimeout := 10 * time.Minute
	t.Logf("Waiting up to %s for the cluster to reach the expected machines count of %d", waitPollTimeout.String(), expectedCount)

	return wait.Poll(waitPollInterval, waitPollTimeout, func() (bool, error) {
		machineList, err := machineClient.List(ctx, metav1.ListOptions{LabelSelector: masterMachineLabelSelector})
		if err != nil {
			return isTransientAPIError(t, err)
		}

		if len(machineList.Items) != expectedCount {
			var machineNames []string
			for _, machine := range machineList.Items {
				machineNames = append(machineNames, machine.Name)
			}
			t.Logf("expected exactly %d master machines, got %d, machines are: %v", expectedCount, len(machineList.Items), machineNames)
			return false, nil
		}

		for _, machine := range machineList.Items {
			machinePhase := pointer.StringDeref(machine.Status.Phase, "")
			if machinePhase != "Running" {
				return false, fmt.Errorf("%q machine is in unexpected %q state, expected Running", machine.Name, machinePhase)
			}
			if !hasMachineDeletionHook(&machine) {
				return false, fmt.Errorf("%q machine doesn't have required deletion hooks", machine.Name)
			}
		}
		return true, nil
	})
}

func recoverClusterToInitialStateIfNeeded(ctx context.Context, t TestingT, machineClient machinev1beta1client.MachineInterface) error {
	waitPollInterval := 15 * time.Second
	waitPollTimeout := 5 * time.Minute
	t.Logf("Trying up to %s to recover the cluster to its initial state", waitPollTimeout.String())

	return wait.Poll(waitPollInterval, waitPollTimeout, func() (bool, error) {
		machineList, err := machineClient.List(ctx, metav1.ListOptions{LabelSelector: masterMachineLabelSelector})
		if err != nil {
			return isTransientAPIError(t, err)
		}

		var machineNames []string
		for _, machine := range machineList.Items {
			machineNames = append(machineNames, machine.Name)
		}

		t.Logf("checking if there are any excessive machines in the cluster (created by a previous test), expected cluster size is 3, found %v machines: %v", len(machineList.Items), machineNames)
		for _, machine := range machineList.Items {
			if strings.HasSuffix(machine.Name, "-clone") {
				// first forcefully remove the hooks
				machine.Spec.LifecycleHooks = machinev1beta1.LifecycleHooks{}
				if _, err := machineClient.Update(ctx, &machine, metav1.UpdateOptions{}); err != nil {
					return isTransientAPIError(t, err)
				}
				// then the machine
				if err := machineClient.Delete(ctx, machine.Name, metav1.DeleteOptions{}); err != nil {
					return isTransientAPIError(t, err)
				}
				t.Logf("successfully deleted an excessive machine %q from the API (perhaps, created by a previous test)", machine.Name)
			}
		}
		return true, nil
	})
}

// DeleteMachine deletes the given machine and returns error if any issues occur during deletion
func DeleteMachine(ctx context.Context, t TestingT, machineClient machinev1beta1client.MachineInterface, machineToDelete string) error {
	t.Logf("attempting to delete machine '%q'", machineToDelete)
	if err := machineClient.Delete(ctx, machineToDelete, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			t.Logf("machine '%q' was listed but not found or already deleted", machineToDelete)
			return nil
		}
		return err
	}
	t.Logf("successfully deleted machine '%q'", machineToDelete)
	return nil
}

// DeleteSingleMachine deletes the master machine with lowest index. Returns the deleted machine name and error if any issues occur during deletion
func DeleteSingleMachine(ctx context.Context, t TestingT, machineClient machinev1beta1client.MachineInterface) (string, error) {
	machineToDelete := ""
	// list master machines
	machineList, err := machineClient.List(ctx, metav1.ListOptions{LabelSelector: masterMachineLabelSelector})
	if err != nil {
		return "", fmt.Errorf("error listing master machines: '%w'", err)
	}
	// Machine names are suffixed with an index number (e.g "ci-op-xlbdrkvl-6a467-qcbkh-master-0")
	// so we sort to pick the lowest index, e.g master-0 in this example
	machineNames := []string{}
	for _, m := range machineList.Items {
		machineNames = append(machineNames, m.Name)
	}
	sort.Strings(machineNames)
	machineToDelete = machineNames[0]

	if err = DeleteMachine(ctx, t, machineClient, machineToDelete); err != nil {
		return "", err
	}
	return machineToDelete, nil
}

// IsCPMSActive returns true if the current platform's has an active CPMS
// Not all platforms are supported (as of 4.12 only AWS and Azure)
// See https://github.com/openshift/cluster-control-plane-machine-set-operator/tree/main/docs/user#supported-platforms
func IsCPMSActive(ctx context.Context, t TestingT, cpmsClient machinev1client.ControlPlaneMachineSetInterface) (bool, error) {
	// The CPMS singleton in the "openshift-machine-api" namespace is named "cluster"
	// https://github.com/openshift/cluster-control-plane-machine-set-operator/blob/bba395abab62fc12de4a9b9b030700546f4b822e/pkg/controllers/controlplanemachineset/controller.go#L50-L53
	cpms, err := cpmsClient.Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// The CPMS state must be active in order for the platform to be supported
	// See https://github.com/openshift/cluster-control-plane-machine-set-operator/blob/7961d1457c6aef26d3b1dafae962da2a2aba18ef/docs/user/installation.md#anatomy-of-a-controlplanemachineset
	if cpms.Spec.State != machinev1.ControlPlaneMachineSetStateActive {
		return false, nil
	}

	return true, nil
}

// DisableCPMS disables the CPMS by deleting the custom resource and verifies it.
// Returns error if there was one while disabling or verifying
func DisableCPMS(ctx context.Context, t TestingT, cpmsClient machinev1client.ControlPlaneMachineSetInterface) error {
	waitPollInterval := 5 * time.Second
	waitPollTimeout := 1 * time.Minute
	if err := cpmsClient.Delete(ctx, "cluster", metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	t.Logf("Waiting up to %s for the CPMS to be disabled", waitPollTimeout.String())
	return wait.PollUntilContextTimeout(ctx, waitPollInterval, waitPollTimeout, true, func(ctx context.Context) (done bool, err error) {
		isActive, err := IsCPMSActive(ctx, t, cpmsClient)
		if err != nil {
			return true, err
		}
		if isActive {
			return false, nil
		}
		return true, nil
	})
}

// EnableCPMS activates the CPMS setting the .spec.state field to Active and verifies it.
// Returns error if there was one while activating or verifying
func EnableCPMS(ctx context.Context, t TestingT, cpmsClient machinev1client.ControlPlaneMachineSetInterface) error {
	waitPollInterval := 5 * time.Second
	waitPollTimeout := 1 * time.Minute
	cpms, err := cpmsClient.Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}

	cpms.Spec.State = machinev1.ControlPlaneMachineSetStateActive
	_, err = cpmsClient.Update(ctx, cpms, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	t.Logf("Waiting up to %s for the CPMS to be activated", waitPollTimeout.String())
	return wait.PollUntilContextTimeout(ctx, waitPollInterval, waitPollTimeout, true, func(ctx context.Context) (done bool, err error) {
		isActive, err := IsCPMSActive(ctx, t, cpmsClient)
		if err != nil {
			return true, err
		}
		if !isActive {
			return false, nil
		}
		return true, nil
	})
}

// EnsureReadyReplicasOnCPMS checks if status.readyReplicas on the cluster CPMS is n
// this effectively counts the number of control-plane machines with the provider state as running
func EnsureReadyReplicasOnCPMS(ctx context.Context, t TestingT, expectedReplicaCount int, cpmsClient machinev1client.ControlPlaneMachineSetInterface, nodeClient v1.NodeInterface) error {
	waitPollInterval := 5 * time.Second
	waitPollTimeout := 30 * time.Minute
	t.Logf("Waiting up to %s for the CPMS to have status.readyReplicas = %v", waitPollTimeout.String(), expectedReplicaCount)

	return wait.Poll(waitPollInterval, waitPollTimeout, func() (bool, error) {
		cpms, err := cpmsClient.Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			return isTransientAPIError(t, err)
		}

		if cpms.Status.ReadyReplicas != int32(expectedReplicaCount) {
			t.Logf("expected %d ready replicas on CPMS, got: %v,", expectedReplicaCount, cpms.Status.ReadyReplicas)
			return false, nil
		}
		t.Logf("CPMS has reached the desired number of ready replicas: %v,", cpms.Status.ReadyReplicas)

		err = EnsureReadyMasterNodes(ctx, expectedReplicaCount, nodeClient)
		if err != nil {
			t.Logf("expected number of master nodes is not ready yet: '%w'", err)
			return false, nil
		}

		return true, nil
	})
}

// EnsureReadyMasterNodes checks if the current master nodes matches the expected number of master nodes,
// and that all master nodes' are Ready
func EnsureReadyMasterNodes(ctx context.Context, expectedReplicaCount int, nodeClient v1.NodeInterface) error {
	masterNodes, err := nodeClient.List(ctx, metav1.ListOptions{LabelSelector: masterNodeRoleLabel})
	if err != nil {
		return fmt.Errorf("failed to list master nodes:'%w'", err)
	}

	if len(masterNodes.Items) != expectedReplicaCount {
		return fmt.Errorf("expected number of master nodes is '%d', but got '%d' instead", expectedReplicaCount, len(masterNodes.Items))
	}

	for _, node := range masterNodes.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status != corev1.ConditionTrue {
				return fmt.Errorf("master node '%v' is not ready", node)
			}
		}
	}

	return nil
}

// EnsureCPMSReplicasConverged returns error if the number of expected master machines not equals the number of actual master machines
// otherwise it returns nil
func EnsureCPMSReplicasConverged(ctx context.Context, cpmsClient machinev1client.ControlPlaneMachineSetInterface) error {
	cpms, err := cpmsClient.Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get controlPlaneMachineSet object: '%w'", err)
	}

	if *cpms.Spec.Replicas != cpms.Status.ReadyReplicas {
		return fmt.Errorf("CPMS replicas failed to converge, expected status.readyReplicas '%d' to be equal to spec.replicas '%v'", cpms.Status.ReadyReplicas, cpms.Spec.Replicas)
	}
	return nil
}

// EnsureVotingMembersCount counts the number of voting etcd members, it doesn't evaluate health conditions or any other attributes (i.e. name) of individual members
// this method won't fail immediately on errors, this is useful during scaling down operation until the feature can ensure this operation to be graceful
func EnsureVotingMembersCount(ctx context.Context, t TestingT, etcdClientFactory EtcdClientCreator, kubeClient kubernetes.Interface, expectedMembersCount int) error {
	waitPollInterval := 15 * time.Second
	waitPollTimeout := 10 * time.Minute
	t.Logf("Waiting up to %s for the cluster to reach the expected member count of %v", waitPollTimeout.String(), expectedMembersCount)

	return wait.Poll(waitPollInterval, waitPollTimeout, func() (bool, error) {
		etcdClient, closeFn, err := etcdClientFactory.NewEtcdClient()
		if err != nil {
			t.Logf("failed to get etcd client, will retry, err: %v", err)
			return false, nil
		}
		defer closeFn()

		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		memberList, err := etcdClient.MemberList(ctx)
		if err != nil {
			t.Logf("failed to get the member list, will retry, err: %v", err)
			return false, nil
		}

		var votingMemberNames []string
		for _, member := range memberList.Members {
			if !member.IsLearner {
				votingMemberNames = append(votingMemberNames, member.Name)
			}
		}
		if len(votingMemberNames) != expectedMembersCount {
			t.Logf("unexpected number of voting etcd members, expected exactly %d, got: %v, current members are: %v", expectedMembersCount, len(votingMemberNames), votingMemberNames)
			return false, nil
		}
		t.Logf("cluster has reached the expected number of %v voting members, the members are: %v", expectedMembersCount, votingMemberNames)

		t.Logf("ensuring that the openshift-etcd/etcd-endpoints cm has the expected number of %v voting members", expectedMembersCount)
		etcdEndpointsConfigMap, err := kubeClient.CoreV1().ConfigMaps("openshift-etcd").Get(ctx, "etcd-endpoints", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		currentVotingMemberIPListSet := sets.NewString()
		for _, votingMemberIP := range etcdEndpointsConfigMap.Data {
			currentVotingMemberIPListSet.Insert(votingMemberIP)
		}
		if currentVotingMemberIPListSet.Len() != expectedMembersCount {
			t.Logf("unexpected number of voting members in the openshift-etcd/etcd-endpoints cm, expected exactly %d, got: %v, current members are: %v", expectedMembersCount, currentVotingMemberIPListSet.Len(), currentVotingMemberIPListSet.List())
			return false, nil
		}
		return true, nil
	})
}

// WaitOnExpectedVotingMembersCount waits for 2 minutes and ensures the etcd membership remains at the expected member count. Returns an error if there is a change
// in the expected voting members count
func WaitOnExpectedVotingMembersCount(ctx context.Context, t TestingT, etcdClientFactory EtcdClientCreator, kubeClient kubernetes.Interface, expectedMembersCount int) error {
	waitPollInterval := 15 * time.Second
	waitPollTimeout := 2 * time.Minute
	var votingMemberNames []string

	err := wait.PollUntilContextTimeout(ctx, waitPollInterval, waitPollTimeout, false, func(ctx context.Context) (done bool, err error) {
		etcdClient, closeFn, err := etcdClientFactory.NewEtcdClient()
		if err != nil {
			t.Logf("failed to get etcd client, will retry, err: %v", err)
			return false, nil
		}
		defer closeFn()

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		memberList, err := etcdClient.MemberList(ctx)
		if err != nil {
			t.Logf("failed to get the member list, will retry, err: %v", err)
			return false, nil
		}

		votingMemberNames = []string{}
		for _, member := range memberList.Members {
			if !member.IsLearner {
				votingMemberNames = append(votingMemberNames, member.Name)
			}
		}
		if len(votingMemberNames) != expectedMembersCount {
			t.Logf("unexpected change in number of voting etcd members from %d to: %v, current members are: %v", expectedMembersCount, len(votingMemberNames), votingMemberNames)
			return true, nil
		}
		return false, nil
	})

	// the poll will timeout and context deadline exceeded error would be returned if there isn't any change in the membership and
	// the poll will return early with no error if there is an unexpected change from the expected voting member count.
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil
		}
		return fmt.Errorf("polling encountered an error: %v", err)
	}
	return fmt.Errorf("failed to confirm that voting membership remained constant, expected %d, got %d, current members are: %v", expectedMembersCount, len(votingMemberNames), votingMemberNames)
}

func EnsureMemberRemoved(t TestingT, etcdClientFactory EtcdClientCreator, memberName string) error {
	waitPollInterval := 15 * time.Second
	// Waiting 30 mins since the test needs to wait for scale-up and scale-down at this point
	waitPollTimeout := 30 * time.Minute
	t.Logf("Waiting up to %s for %v member to be removed from the cluster", waitPollTimeout.String(), memberName)

	return wait.Poll(waitPollInterval, waitPollTimeout, func() (bool, error) {
		etcdClient, closeFn, err := etcdClientFactory.NewEtcdClient()
		if err != nil {
			t.Logf("failed to get etcd client, will retry, err: %v", err)
			return false, nil
		}
		defer closeFn()

		ctx, cancel := context.WithTimeout(context.TODO(), 15*time.Second)
		defer cancel()
		memberList, err := etcdClient.MemberList(ctx)
		if err != nil {
			t.Logf("failed to get member list, will retry, err: %v", err)
			return false, nil
		}

		currentVotingMemberIPListSet := sets.NewString()
		for _, member := range memberList.Members {
			if !member.IsLearner {
				currentVotingMemberIPListSet.Insert(member.PeerURLs[0])
			}
		}

		for _, member := range memberList.Members {
			if member.Name == memberName {
				framework.Logf("member %v (%v) has not been removed, current voting members are: %v", member.Name, member.PeerURLs[0], currentVotingMemberIPListSet.List())
				return false, nil
			}
		}
		return true, nil
	})
}

func EnsureHealthyMember(t TestingT, etcdClientFactory EtcdClientCreator, memberName string) error {
	etcdClient, closeFn, err := etcdClientFactory.NewEtcdClientForMember(memberName)
	if err != nil {
		return err
	}
	defer closeFn()

	ctx, cancel := context.WithTimeout(context.TODO(), 15*time.Second)
	defer cancel()

	// We know it's a voting member so lineared read is fine
	_, err = etcdClient.Get(ctx, "health")
	if err != nil {
		return fmt.Errorf("failed to check healthiness condition of the %q member, err: %v", memberName, err)
	}
	t.Logf("successfully evaluated health condition of %q member", memberName)
	return nil
}

// MachineNameToEtcdMemberName finds an etcd member name that corresponds to the given machine name
// first it looks up a node that corresponds to the machine by comparing the ProviderID field
// next, it returns the node name as it is used to name an etcd member.
//
// # In cases the ProviderID is empty it will try to find a node that matches an internal IP address
//
// note:
// it will exit and report an error in case the node was not found
func MachineNameToEtcdMemberName(ctx context.Context, kubeClient kubernetes.Interface, machineClient machinev1beta1client.MachineInterface, machineName string) (string, error) {
	machine, err := machineClient.Get(ctx, machineName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	masterNodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: masterNodeRoleLabel})
	if err != nil {
		return "", err
	}

	machineProviderID := pointer.StringDeref(machine.Spec.ProviderID, "")
	if len(machineProviderID) != 0 {
		// case 1: find corresponding node, match on providerID
		var nodeNames []string
		for _, masterNode := range masterNodes.Items {
			if masterNode.Spec.ProviderID == machineProviderID {
				return masterNode.Name, nil
			}
			nodeNames = append(nodeNames, masterNode.Name)
		}

		return "", fmt.Errorf("unable to find a node for the corresponding %q machine on ProviderID: %v, checked: %v", machineName, machineProviderID, nodeNames)
	}

	// case 2: match on an internal ip address
	machineIPListSet := sets.NewString()
	for _, addr := range machine.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			machineIPListSet.Insert(addr.Address)
		}
	}

	var nodeNames []string
	for _, masterNode := range masterNodes.Items {
		for _, addr := range masterNode.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				if machineIPListSet.Has(addr.Address) {
					return masterNode.Name, nil
				}
			}
			nodeNames = append(nodeNames, masterNode.Name)
		}
	}
	return "", fmt.Errorf("unable to find a node for the corresponding %q machine on the following machine's IPs: %v, checked: %v", machineName, machineIPListSet.List(), nodeNames)
}

// StopKubelet stops the kubelet on the given node by spwaning a pod on the node and running the stop kubelet command
func StopKubelet(ctx context.Context, adminKubeClient kubernetes.Interface, node *corev1.Node) error {
	if node == nil {
		return fmt.Errorf("cannot stop kubelet: node reference is nil; a valid node is required")
	}

	podSpec := applycorev1.PodSpec().WithRestartPolicy(corev1.RestartPolicyNever).WithHostNetwork(true).WithHostPID(true)
	podSpec.Containers = []applycorev1.ContainerApplyConfiguration{
		*applycorev1.Container().
			WithName("kubelet-stopper").
			WithSecurityContext(applycorev1.SecurityContext().WithPrivileged(true).WithRunAsUser(0)).
			WithImage(image.ShellImage()).
			WithVolumeMounts(applycorev1.VolumeMount().WithName("host").WithMountPath("/host")).
			WithCommand("/bin/sh").
			WithArgs("-c", "chroot /host /bin/sh -c 'sleep 1 && systemctl stop kubelet'"),
	}
	podSpec.NodeSelector = map[string]string{"kubernetes.io/hostname": node.Labels["kubernetes.io/hostname"]}
	podSpec.Tolerations = []applycorev1.TolerationApplyConfiguration{*applycorev1.Toleration().WithOperator(corev1.TolerationOpExists)}
	podSpec.Volumes = []applycorev1.VolumeApplyConfiguration{
		*applycorev1.Volume().WithName("host").WithHostPath(applycorev1.HostPathVolumeSource().WithPath("/").WithType("Directory")),
	}

	pod := applycorev1.Pod("kubelet-stopper", "openshift-etcd").WithSpec(podSpec)
	_, err := adminKubeClient.CoreV1().Pods(*pod.Namespace).Apply(context.Background(), pod, metav1.ApplyOptions{FieldManager: *pod.Name})
	if err != nil {
		return fmt.Errorf("error applying pod %w", err)
	}

	isNodeNotReady := e2enode.WaitForNodeToBeNotReady(ctx, adminKubeClient, node.Name, 5*time.Minute)
	if !isNodeNotReady {
		return fmt.Errorf("timed out waiting for the node %s to be NotReady", node.Name)
	}

	return nil
}

func AssertVotingMemberAndMasterMachineCount(ctx context.Context, t TestingT, expectedCount int, actionDescription string, kubeClient kubernetes.Interface, machineClient machinev1beta1client.MachineInterface, etcdClientFactory EtcdClientCreator) {
	framework.Logf("Waiting for etcd membership to show %d voting members", expectedCount)
	err := EnsureVotingMembersCount(ctx, t, etcdClientFactory, kubeClient, expectedCount)
	err = errors.Wrapf(err, "%s: timed out waiting for %d voting members in the etcd cluster and etcd-endpoints configmap", actionDescription, expectedCount)
	o.Expect(err).ToNot(o.HaveOccurred())

	framework.Logf("Waiting for %d Running master machines", expectedCount)
	err = EnsureMasterMachinesAndCount(ctx, t, expectedCount, machineClient)
	err = errors.Wrapf(err, "%s: timed out waiting for only %d Running master machines", actionDescription, expectedCount)
	o.Expect(err).ToNot(o.HaveOccurred())

}

func AssertCPMSReplicasAndConvergence(ctx context.Context, t TestingT, expectedCount int, actionDescription string, cpmsClient machinev1client.ControlPlaneMachineSetInterface, nodeClient v1.NodeInterface) {
	framework.Logf("Waiting for %d ready replicas on CPMS", expectedCount)
	err := EnsureReadyReplicasOnCPMS(ctx, t, expectedCount, cpmsClient, nodeClient)
	err = errors.Wrapf(err, "%s: timed out waiting for CPMS to show %d ready replicas", actionDescription, expectedCount)
	o.Expect(err).ToNot(o.HaveOccurred())

	framework.Logf("Waiting for CPMS replicas to converge")
	err = EnsureCPMSReplicasConverged(ctx, cpmsClient)
	o.Expect(err).ToNot(o.HaveOccurred())
}

func InitPlatformSpecificConfiguration(oc *exutil.CLI) func() {
	SkipIfUnsupportedPlatform(context.TODO(), oc)

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// For baremetal platforms, an extra worker must be previously deployed to allow subsequent scaling operations
	if infra.Status.PlatformStatus.Type == configv1.BareMetalPlatformType {
		dc, err := dynamic.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		helper := bmhelper.NewBaremetalTestHelper(dc)
		if helper.CanDeployExtraWorkers() {
			helper.Setup()
			helper.DeployExtraWorker(0)
		}
		return helper.DeleteAllExtraWorkers
	}
	return func() { /*noop*/ }
}

func SkipIfUnsupportedPlatform(ctx context.Context, oc *exutil.CLI) {
	machineClientSet, err := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).ToNot(o.HaveOccurred())
	machineClient := machineClientSet.MachineV1beta1().Machines("openshift-machine-api")
	skipUnlessFunctionalMachineAPI(ctx, machineClient)
	skipIfSingleNode(oc)
	skipIfBareMetal(oc)
}

func skipUnlessFunctionalMachineAPI(ctx context.Context, machineClient machinev1beta1client.MachineInterface) {
	machines, err := machineClient.List(ctx, metav1.ListOptions{LabelSelector: masterMachineLabelSelector})
	// the machine API can be unavailable resulting in a 404 or an empty list
	if err != nil {
		if !apierrors.IsNotFound(err) {
			o.Expect(err).ToNot(o.HaveOccurred())
		}
		e2eskipper.Skipf("haven't found machines resources on the cluster, this test can be run on a platform that supports functional MachineAPI")
		return
	}
	if len(machines.Items) == 0 {
		e2eskipper.Skipf("got an empty list of machines resources from the cluster, this test can be run on a platform that supports functional MachineAPI")
		return
	}

	// we expect just a single machine to be in the Running state
	for _, machine := range machines.Items {
		phase := pointer.StringDeref(machine.Status.Phase, "")
		if phase == "Running" {
			return
		}
	}
	e2eskipper.Skipf("haven't found a machine in running state, this test can be run on a platform that supports functional MachineAPI")
	return
}

func skipIfAzure(oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if infra.Status.PlatformStatus.Type == configv1.AzurePlatformType {
		e2eskipper.Skipf("this test is currently flaky on the azure platform")
	}
}

func skipIfSingleNode(oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if infra.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode {
		e2eskipper.Skipf("this test can be run only against an HA cluster, skipping it on an SNO env")
	}
}

func skipIfBareMetal(oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if infra.Status.PlatformStatus.Type == configv1.BareMetalPlatformType {
		e2eskipper.Skipf("this test is currently broken on the metal platform and needs to be fixed")
	}
}

func skipIfVsphere(oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if infra.Status.PlatformStatus.Type == configv1.VSpherePlatformType {
		e2eskipper.Skipf("this test is currently broken on the vsphere platform and needs to be fixed (BZ2094919)")
	}
}

func hasMachineDeletionHook(machine *machinev1beta1.Machine) bool {
	for _, hook := range machine.Spec.LifecycleHooks.PreDrain {
		if hook.Name == machineDeletionHookName && hook.Owner == machineDeletionHookOwner {
			return true
		}
	}
	return false
}

// transientAPIError returns true if the provided error indicates that a retry against an HA server has a good chance to succeed.
func transientAPIError(err error) bool {
	switch {
	case err == nil:
		return false
	case net.IsProbableEOF(err), net.IsConnectionReset(err), net.IsNoRoutesError(err), isClientConnectionLost(err):
		return true
	default:
		return false
	}
}

func isTransientAPIError(t TestingT, err error) (bool, error) {
	// we tolerate some disruption until https://bugzilla.redhat.com/show_bug.cgi?id=2082778
	// is fixed and rely on the monitor for reporting (p99).
	// this is okay since we observe disruption during the upgrade jobs too,
	// the only difference is that during the upgrade job we don’t access the API except from the monitor.
	if transientAPIError(err) {
		t.Logf("ignoring %v for now, the error is considered a transient error (will retry)", err)
		return false, nil
	}
	return false, err
}

func isClientConnectionLost(err error) bool {
	return strings.Contains(err.Error(), "client connection lost")
}
