package helpers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machinev1beta1client "github.com/openshift/client-go/machine/clientset/versioned/typed/machine/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

const masterMachineLabelSelector = "machine.openshift.io/cluster-api-machine-role" + "=" + "master"
const machineDeletionHookName = "EtcdQuorumOperator"
const machineDeletionHookOwner = "clusteroperator/etcd"

type TestingT interface {
	Logf(format string, args ...interface{})
}

// CreateNewMasterMachine creates a new master node by cloning an existing Machine resource
func CreateNewMasterMachine(ctx context.Context, t TestingT, machineClient machinev1beta1client.MachineInterface) (string, error) {
	machineList, err := machineClient.List(ctx, metav1.ListOptions{LabelSelector: masterMachineLabelSelector})
	if err != nil {
		return "", err
	}
	var machineToClone *machinev1beta1.Machine
	for _, machine := range machineList.Items {
		machinePhase := pointer.StringDeref(machine.Status.Phase, "Unknown")
		if machinePhase == "Running" {
			machineToClone = &machine
			break
		}
		t.Logf("%q machine is in unexpected %q state", machine.Name, machinePhase)
	}

	if machineToClone == nil {
		return "", fmt.Errorf("unable to find a running master machine to clone")
	}
	// assigning a new Name and clearing ProviderID is enough
	// for MAO to pick it up and provision a new master machine/node
	machineToClone.Name = fmt.Sprintf("%s-clone", machineToClone.Name)
	machineToClone.Spec.ProviderID = nil
	machineToClone.ResourceVersion = ""

	clonedMachine, err := machineClient.Create(context.TODO(), machineToClone, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	t.Logf("Created a new master machine/node %q", clonedMachine.Name)
	return clonedMachine.Name, nil
}

func EnsureMasterMachine(ctx context.Context, t TestingT, machineName string, machineClient machinev1beta1client.MachineInterface) error {
	waitPollInterval := 15 * time.Second
	waitPollTimeout := 5 * time.Minute
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
func EnsureInitialClusterState(ctx context.Context, t TestingT, etcdClientFactory EtcdClientCreator, machineClient machinev1beta1client.MachineInterface) error {
	if err := recoverClusterToInitialStateIfNeeded(ctx, t, machineClient); err != nil {
		return err
	}
	if err := EnsureVotingMembersCount(t, etcdClientFactory, 3); err != nil {
		return err
	}
	return EnsureMasterMachinesAndCount(ctx, t, machineClient)
}

// EnsureMasterMachinesAndCount checks if there are only 3 running master machines otherwise it returns an error
func EnsureMasterMachinesAndCount(ctx context.Context, t TestingT, machineClient machinev1beta1client.MachineInterface) error {
	waitPollInterval := 15 * time.Second
	waitPollTimeout := 10 * time.Minute
	t.Logf("Waiting up to %s for the cluster to reach the expected machines count of 3", waitPollTimeout.String())

	return wait.Poll(waitPollInterval, waitPollTimeout, func() (bool, error) {
		machineList, err := machineClient.List(ctx, metav1.ListOptions{LabelSelector: masterMachineLabelSelector})
		if err != nil {
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

		if len(machineList.Items) != 3 {
			var machineNames []string
			for _, machine := range machineList.Items {
				machineNames = append(machineNames, machine.Name)
			}
			t.Logf("expected exactly 3 master machines, got %d, machines are: %v", len(machineList.Items), machineNames)
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
	machineList, err := machineClient.List(ctx, metav1.ListOptions{LabelSelector: masterMachineLabelSelector})
	if err != nil {
		return err
	}

	var machineNames []string
	for _, machine := range machineList.Items {
		machineNames = append(machineNames, machine.Name)
	}

	t.Logf("checking if there are any excessive machines in the cluster (created by a previous test), expected cluster size is 3, found %v machines: %v", len(machineList.Items), machineNames)
	for _, machine := range machineList.Items {
		if strings.HasSuffix(machine.Name, "-clone") {
			err := machineClient.Delete(ctx, machine.Name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("failed removing the machine: %q, err: %v", machine.Name, err)
			}
			t.Logf("successfully deleted an excessive machine %q from the API (perhaps, created by a previous test)", machine.Name)
		}
	}

	return nil
}

// EnsureVotingMembersCount counts the number of voting etcd members, it doesn't evaluate health conditions or any other attributes (i.e. name) of individual members
// this method won't fail immediately on errors, this is useful during scaling down operation until the feature can ensure this operation to be graceful
func EnsureVotingMembersCount(t TestingT, etcdClientFactory EtcdClientCreator, expectedMembersCount int) error {
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

		ctx, cancel := context.WithTimeout(context.TODO(), 15*time.Second)
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
		return true, nil
	})
}

func EnsureMemberRemoved(etcdClientFactory EtcdClientCreator, memberName string) error {
	etcdClient, closeFn, err := etcdClientFactory.NewEtcdClient()
	if err != nil {
		return err
	}
	defer closeFn()

	ctx, cancel := context.WithTimeout(context.TODO(), 15*time.Second)
	defer cancel()
	rsp, err := etcdClient.MemberList(ctx)
	if err != nil {
		return err
	}

	for _, member := range rsp.Members {
		if member.Name == memberName {
			return fmt.Errorf("member %v hasn't been removed", spew.Sdump(member))
		}
	}
	return nil
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
// next, it returns the node name as it is used to name an etcd member
//
// note:
// it will exit and report an error in case the node was not found
func MachineNameToEtcdMemberName(ctx context.Context, kubeClient kubernetes.Interface, machineClient machinev1beta1client.MachineInterface, machineName string) (string, error) {
	machine, err := machineClient.Get(ctx, machineName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	machineProviderID := pointer.StringDeref(machine.Spec.ProviderID, "")
	if len(machineProviderID) == 0 {
		return "", fmt.Errorf("failed to get the providerID for %q machine", machineName)
	}

	// find corresponding node, match on providerID
	masterNodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
	if err != nil {
		return "", err
	}

	var nodeNames []string
	for _, masterNode := range masterNodes.Items {
		if masterNode.Spec.ProviderID == machineProviderID {
			return masterNode.Name, nil
		}
		nodeNames = append(nodeNames, masterNode.Name)
	}

	return "", fmt.Errorf("unable to find a node for the corresponding %q machine on ProviderID: %v, checked: %v", machineName, machineProviderID, nodeNames)
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

func isClientConnectionLost(err error) bool {
	return strings.Contains(err.Error(), "client connection lost")
}
