package etcd

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/pkg/errors"

	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	machinev1 "github.com/openshift/client-go/machine/clientset/versioned/typed/machine/v1"
	machinev1beta1client "github.com/openshift/client-go/machine/clientset/versioned/typed/machine/v1beta1"
	testlibraryapi "github.com/openshift/library-go/test/library/apiserver"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	scalingtestinglibrary "github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-etcd][Feature:EtcdVerticalScaling][Suite:openshift/etcd/scaling][Serial] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-scaling").AsAdmin()

	var (
		etcdClientFactory *scalingtestinglibrary.EtcdClientFactoryImpl
		machineClientSet  *machineclient.Clientset
		machineClient     machinev1beta1client.MachineInterface
		nodeClient        v1.NodeInterface
		cpmsClient        machinev1.ControlPlaneMachineSetInterface
		kubeClient        kubernetes.Interface
		cpmsActive        bool
		ctx               context.Context
		err               error
	)

	cleanupPlatformSpecificConfiguration := func() { /*noop*/ }

	g.BeforeEach(func() {
		cleanupPlatformSpecificConfiguration = scalingtestinglibrary.InitPlatformSpecificConfiguration(oc)

		//setup
		ctx = context.TODO()
		etcdClientFactory = scalingtestinglibrary.NewEtcdClientFactory(oc.KubeClient())
		machineClientSet, err = machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).ToNot(o.HaveOccurred())
		machineClient = machineClientSet.MachineV1beta1().Machines("openshift-machine-api")
		nodeClient = oc.KubeClient().CoreV1().Nodes()
		cpmsClient = machineClientSet.MachineV1().ControlPlaneMachineSets("openshift-machine-api")
		kubeClient = oc.KubeClient()

		// assert the cluster state before we run the test
		err = scalingtestinglibrary.EnsureInitialClusterState(context.Background(), g.GinkgoT(), etcdClientFactory, machineClient, kubeClient)
		err = errors.Wrap(err, "pre-test: timed out waiting for initial cluster state to have 3 running machines and 3 voting members")
		o.Expect(err).ToNot(o.HaveOccurred())

		// checks if the current platform has an active CPMS
		cpmsActive, err = scalingtestinglibrary.IsCPMSActive(context.Background(), g.GinkgoT(), cpmsClient)
		err = errors.Wrap(err, "pre-test: failed to determine if ControlPlaneMachineSet is present and active")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.AfterEach(func() {
		cleanupPlatformSpecificConfiguration()
	})

	// The following test covers a basic vertical scaling scenario.
	// 1) Delete a machine
	// 2) That should prompt the ControlPlaneMachineSetOperator(CPMSO) to create a replacement
	//		machine and node for that machine index
	// 3) The operator will first scale-up the new machine's member
	// 4) Then scale-down the machine that is pending deletion by removing its member and deletion hook
	// The test will validate the size of the etcd cluster and make sure the cluster membership
	// changes with the new member added and the old one removed.
	g.It("is able to vertically scale up and down with a single node [Timeout:60m][apigroup:machine.openshift.io]", g.Label("Size:L"), func() {
		if cpmsActive {
			// TODO: Add cleanup step to recover back to 3 running machines and members if the test fails

			framework.Logf("CPMS is active. Relying on CPMSO to replace the machine during vertical scaling")

			// step 1: delete a running machine to trigger the CPMSO to create a new one to replace it
			deletedMachineName, err := scalingtestinglibrary.DeleteSingleMachine(ctx, g.GinkgoT(), machineClient)
			o.Expect(err).ToNot(o.HaveOccurred())
			framework.Logf("Deleted machine %q", deletedMachineName)

			// step 2: wait until we have 4 voting members which indicates a scale-up event
			// We can't check for 4 members here as the clustermemberremoval controller will race to
			// remove the old member (from the machine pending deletion) as soon as the new machine's member
			// is promoted to a voting member.
			// In practice there is a period during the revision rollout due to the membership change
			// where we will see 4 voting members, however our polling may miss that and cause the test to flake.
			//
			// We previously waited until the CPMS status.readyReplicas showed 4 however that doesn't happen in practice
			// so we can't use that as a signal for scale-up

			// step 3: wait for the machine pending deletion to have its member removed to indicate scale-down
			framework.Logf("Waiting for etcd member %q to be removed", deletedMachineName)
			deletedMemberName, err := scalingtestinglibrary.MachineNameToEtcdMemberName(ctx, oc.KubeClient(), machineClient, deletedMachineName)
			err = errors.Wrapf(err, "failed to get etcd member name for deleted machine: %v", deletedMachineName)
			o.Expect(err).ToNot(o.HaveOccurred())

			err = scalingtestinglibrary.EnsureMemberRemoved(g.GinkgoT(), etcdClientFactory, deletedMemberName)
			err = errors.Wrapf(err, "scale-down: timed out waiting for member (%v) to be removed", deletedMemberName)
			o.Expect(err).ToNot(o.HaveOccurred())

			// step 4: Wait for apiserver revision rollout to stabilize
			g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
			err = testlibraryapi.WaitForAPIServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc.KubeClient().CoreV1().Pods("openshift-kube-apiserver"))
			err = errors.Wrap(err, "scale-up: timed out waiting for APIServer pods to stabilize on the same revision")
			o.Expect(err).ToNot(o.HaveOccurred())

			// step 5: verify member and machine counts go back down to 3
			framework.Logf("Waiting for etcd membership to show 3 voting members")
			err = scalingtestinglibrary.EnsureVotingMembersCount(ctx, g.GinkgoT(), etcdClientFactory, kubeClient, 3)
			err = errors.Wrap(err, "scale-down: timed out waiting for 3 voting members in the etcd cluster and etcd-endpoints configmap")
			o.Expect(err).ToNot(o.HaveOccurred())

			framework.Logf("Waiting for 3 ready replicas on CPMS")
			err = scalingtestinglibrary.EnsureReadyReplicasOnCPMS(ctx, g.GinkgoT(), 3, cpmsClient, nodeClient)
			err = errors.Wrap(err, "scale-down: timed out waiting for CPMS to show 3 ready replicas")
			o.Expect(err).ToNot(o.HaveOccurred())

			framework.Logf("Waiting for 3 Running master machines")
			err = scalingtestinglibrary.EnsureMasterMachinesAndCount(ctx, g.GinkgoT(), machineClient)
			err = errors.Wrap(err, "scale-down: timed out waiting for only 3 Running master machines")
			o.Expect(err).ToNot(o.HaveOccurred())

			framework.Logf("Waiting for CPMS replicas to converge")
			err = scalingtestinglibrary.EnsureCPMSReplicasConverged(ctx, cpmsClient)
			o.Expect(err).ToNot(o.HaveOccurred())

			return
		}

		// For a non-CPMS supported platform the test resorts to manually creating and deleting a machine
		framework.Logf("CPMS is inactive. The test will manually add and remove a machine for vertical scaling")

		// step 0: ensure clean state after the test
		defer func() {
			// since the deletion triggers a new rollout
			// we need to make sure that the API is stable after the test
			// so that other e2e test won't hit an API that undergoes a termination (write request might fail)
			g.GinkgoT().Log("cleaning routine: ensuring initial cluster state and waiting for api servers to stabilize on the same revision")
			err = scalingtestinglibrary.EnsureInitialClusterState(ctx, g.GinkgoT(), etcdClientFactory, machineClient, kubeClient)
			err = errors.Wrap(err, "cleaning routine: timed out while ensuring cluster state back to 3 running machines and 3 voting members")
			o.Expect(err).ToNot(o.HaveOccurred())

			err = testlibraryapi.WaitForAPIServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc.KubeClient().CoreV1().Pods("openshift-kube-apiserver"))
			err = errors.Wrap(err, "cleaning routine: timed out waiting for APIServer pods to stabilize on the same revision")
			o.Expect(err).ToNot(o.HaveOccurred())
		}()

		// step 1: add a new master node and wait until it is in Running state
		machineName, err := scalingtestinglibrary.CreateNewMasterMachine(ctx, g.GinkgoT(), machineClient, nil)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = scalingtestinglibrary.EnsureMasterMachine(ctx, g.GinkgoT(), machineName, machineClient)
		err = errors.Wrapf(err, "scale-up: timed out waiting for machine (%s) to become Running", machineName)
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 2: wait until a new member shows up and check if it is healthy
		//         and until all kube-api servers have reached the same revision
		//         this additional step is the best-effort of ensuring they
		//         have observed the new member before disruption
		err = scalingtestinglibrary.EnsureVotingMembersCount(ctx, g.GinkgoT(), etcdClientFactory, kubeClient, 4)
		err = errors.Wrap(err, "scale-up: timed out waiting for 4 voting members in the etcd cluster and etcd-endpoints configmap")
		o.Expect(err).ToNot(o.HaveOccurred())

		memberName, err := scalingtestinglibrary.MachineNameToEtcdMemberName(ctx, oc.KubeClient(), machineClient, machineName)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = scalingtestinglibrary.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, memberName)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = testlibraryapi.WaitForAPIServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc.KubeClient().CoreV1().Pods("openshift-kube-apiserver"))
		err = errors.Wrap(err, "scale-up: timed out waiting for APIServer pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 3: clean-up: delete the machine and wait until etcd member is removed from the etcd cluster
		err = machineClient.Delete(ctx, machineName, metav1.DeleteOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		framework.Logf("successfully deleted the machine %q from the API", machineName)

		err = scalingtestinglibrary.EnsureVotingMembersCount(ctx, g.GinkgoT(), etcdClientFactory, kubeClient, 3)
		err = errors.Wrap(err, "scale-down: timed out waiting for 3 voting members in the etcd cluster and etcd-endpoints configmap")
		o.Expect(err).ToNot(o.HaveOccurred())

		err = scalingtestinglibrary.EnsureMemberRemoved(g.GinkgoT(), etcdClientFactory, memberName)
		err = errors.Wrapf(err, "scale-down: timed out waiting for member (%v) to be removed", memberName)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = scalingtestinglibrary.EnsureMasterMachinesAndCount(ctx, g.GinkgoT(), machineClient)
		err = errors.Wrap(err, "scale-down: timed out waiting for only 3 Running master machines")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	// The following test covers a basic vertical scaling scenario when CPMS is disabled
	// and validates that the scale-down does not happen before the scale-up event.
	// When CPMS is active, we can't ideally confirm that scale-down doesn't happen before scale-up because,
	// the clustermemberremoval controller will race to remove the old member (from the machine pending deletion)
	// as soon as the new machine's member is promoted to a voting member.
	//
	// 1) If the CPMS is active, first disable it by deleting the CPMS custom resource
	// 2) Delete a machine
	// 3) Ensure the voting member count remains at 3 after the deletion of a machine and before a new machine is added,
	// 	  to verify that scale-down hasn't occurred before scale up when cluster membership is healthy
	// 4) Create a new master machine and ensure it is running (scale-up)
	// 5) Scale-down is validated by confirming the member removal and changes in the cluster membership
	g.It("is able to vertically scale up and down when CPMS is disabled [apigroup:machine.openshift.io]", g.Label("Size:L"), func() {
		if cpmsActive {
			// step 0: disable the CPMS
			framework.Logf("Disable the CPMS")
			err := scalingtestinglibrary.DisableCPMS(ctx, g.GinkgoT(), cpmsClient)
			err = errors.Wrap(err, "pre-test: failed to disable the CPMS")
			o.Expect(err).ToNot(o.HaveOccurred())

			// re-enable CPMS after the test
			defer func() {
				framework.Logf("Re-enable the CPMS")
				err := scalingtestinglibrary.EnableCPMS(ctx, g.GinkgoT(), cpmsClient)
				err = errors.Wrap(err, "post-test: failed to re-enable the CPMS")
				o.Expect(err).ToNot(o.HaveOccurred())
			}()
		}

		framework.Logf("CPMS is disabled. The test will delete an existing machine and manually create a new machine to validate scale-down doesn't happen before scale-up event")

		// step 1: delete a running machine
		//
		// A copy of the machine to be deleted is made initially, so it can be used to clone a new machine, thereby ensuring
		// the newly created machine belongs to the same index and is placed in the same availability zone as the deleted machine.
		machineToDelete, err := scalingtestinglibrary.AnyRunningMasterMachine(ctx, g.GinkgoT(), machineClient)
		err = errors.Wrap(err, "initial-action: failed to retrieve a running master machine for deletion")
		o.Expect(err).ToNot(o.HaveOccurred())

		err = scalingtestinglibrary.DeleteMachine(ctx, g.GinkgoT(), machineClient, machineToDelete.Name)
		o.Expect(err).ToNot(o.HaveOccurred())
		framework.Logf("Deleted machine %q", machineToDelete.Name)

		// step 2: Verify the voting member count remain at 3 after the deletion of a machine and
		// before a new machine is added to ensure scale-down hasn't occurred before scale up when cluster membership is healthy.
		framework.Logf("Ensuring the etcd membership remains at 3 voting members to confirm that scale-down hasn't occurred before scale up when cluster membership is healthy")
		err = scalingtestinglibrary.WaitOnExpectedVotingMembersCount(ctx, g.GinkgoT(), etcdClientFactory, kubeClient, 3)
		err = errors.Wrap(err, "scale-down should not have happened before scale up when cluster membership is healthy")
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 3: add a new master node and wait until it is in Running state.
		newMachineName, err := scalingtestinglibrary.CreateNewMasterMachine(ctx, g.GinkgoT(), machineClient, machineToDelete)
		o.Expect(err).ToNot(o.HaveOccurred())
		framework.Logf("Created machine %q", newMachineName)

		err = scalingtestinglibrary.EnsureMasterMachine(ctx, g.GinkgoT(), newMachineName, machineClient)
		err = errors.Wrapf(err, "scale-up: timed out waiting for machine (%s) to become Running", newMachineName)
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 4: wait for the machine pending deletion to have its member removed to indicate scale-down
		framework.Logf("Waiting for etcd member %q to be removed", machineToDelete.Name)
		deletedMemberName, err := scalingtestinglibrary.MachineNameToEtcdMemberName(ctx, oc.KubeClient(), machineClient, machineToDelete.Name)
		err = errors.Wrapf(err, "failed to get etcd member name for deleted machine: %v", machineToDelete.Name)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = scalingtestinglibrary.EnsureMemberRemoved(g.GinkgoT(), etcdClientFactory, deletedMemberName)
		err = errors.Wrapf(err, "scale-down: timed out waiting for member (%v) to be removed", deletedMemberName)
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 5: Wait for apiserver revision rollout to stabilize
		framework.Logf("waiting for api servers to stabilize on the same revision")
		err = testlibraryapi.WaitForAPIServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc.KubeClient().CoreV1().Pods("openshift-kube-apiserver"))
		err = errors.Wrap(err, "scale-down: timed out waiting for APIServer pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 6: verify member and machine counts go back down to 3
		framework.Logf("Waiting for etcd membership to show 3 voting members")
		err = scalingtestinglibrary.EnsureVotingMembersCount(ctx, g.GinkgoT(), etcdClientFactory, kubeClient, 3)
		err = errors.Wrap(err, "scale-down: timed out waiting for 3 voting members in the etcd cluster and etcd-endpoints configmap")
		o.Expect(err).ToNot(o.HaveOccurred())

		framework.Logf("Waiting for 3 ready replicas on CPMS")
		err = scalingtestinglibrary.EnsureReadyReplicasOnCPMS(ctx, g.GinkgoT(), 3, cpmsClient, nodeClient)
		err = errors.Wrap(err, "scale-down: timed out waiting for CPMS to show 3 ready replicas")
		o.Expect(err).ToNot(o.HaveOccurred())

		framework.Logf("Waiting for 3 Running master machines")
		err = scalingtestinglibrary.EnsureMasterMachinesAndCount(ctx, g.GinkgoT(), machineClient)
		err = errors.Wrap(err, "scale-down: timed out waiting for only 3 Running master machines")
		o.Expect(err).ToNot(o.HaveOccurred())

		framework.Logf("Waiting for CPMS replicas to converge")
		err = scalingtestinglibrary.EnsureCPMSReplicasConverged(ctx, cpmsClient)
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})
