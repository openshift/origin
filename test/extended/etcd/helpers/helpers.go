package helpers

import (
	"context"
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	machinev1beta1client "github.com/openshift/client-go/machine/clientset/versioned/typed/machine/v1beta1"

	bmhelper "github.com/openshift/origin/test/extended/baremetal"
	exutil "github.com/openshift/origin/test/extended/util"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
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
	machineToClone.Annotations = map[string]string{}
	machineToClone.Spec.LifecycleHooks = machinev1beta1.LifecycleHooks{}

	clonedMachine, err := machineClient.Create(context.TODO(), machineToClone, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	t.Logf("Created a new master machine/node %q", clonedMachine.Name)
	return clonedMachine.Name, nil
}

func EnsureMasterMachine(ctx context.Context, t TestingT, machineName string, machineClient machinev1beta1client.MachineInterface) error {
	waitPollInterval := 15 * time.Second
	waitPollTimeout := 10 * time.Minute
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
func EnsureInitialClusterState(ctx context.Context, t TestingT, machineClient machinev1beta1client.MachineInterface, kubeClient kubernetes.Interface) error {
	if err := recoverClusterToInitialStateIfNeeded(ctx, t, machineClient); err != nil {
		return err
	}
	if err := EnsureVotingMembersCount(ctx, t, kubeClient, 3); err != nil {
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
			return isTransientAPIError(t, err)
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

// EnsureVotingMembersCount counts the number of voting etcd members by looking at the endpoints configmap, it doesn't evaluate health conditions or any other attributes (i.e. name) of individual members
// this method won't fail immediately on errors, this is useful during scaling down operation until the feature can ensure this operation to be graceful
func EnsureVotingMembersCount(ctx context.Context, t TestingT, kubeClient kubernetes.Interface, expectedMembersCount int) error {
	waitPollInterval := 15 * time.Second
	waitPollTimeout := 10 * time.Minute
	t.Logf("Waiting up to %s for the cluster to reach the expected member count of %v", waitPollTimeout.String(), expectedMembersCount)

	return wait.Poll(waitPollInterval, waitPollTimeout, func() (bool, error) {
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
	// no need to scale a single node cluster
	skipIfSingleNode(oc)
	// bare metal, albeit having MCO, is having trouble provisioning machines quickly enough
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
