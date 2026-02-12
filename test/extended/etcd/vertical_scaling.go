package etcd

import (
	"context"
	"fmt"
	"time"

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
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-etcd][Feature:EtcdVerticalScaling]", func() {
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

	g.Describe("[Suite:openshift/etcd/scaling][Serial] etcd", func() {
		// The following test covers a basic vertical scaling scenario.
		// 1) Delete a machine
		// 2) That should prompt the ControlPlaneMachineSetOperator(CPMSO) to create a replacement
		//		machine and node for that machine index
		// 3) The operator will first scale-up the new machine's member
		// 4) Then scale-down the machine that is pending deletion by removing its member and deletion hook
		// The test will validate the size of the etcd cluster and make sure the cluster membership
		// changes with the new member added and the old one removed.
		g.It("is able to vertically scale up and down with a single node [Timeout:60m][apigroup:machine.openshift.io]", func() {
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
				scalingtestinglibrary.AssertVotingMemberAndMasterMachineCount(ctx, g.GinkgoT(), 3, "scale-down", kubeClient, machineClient, etcdClientFactory)

				scalingtestinglibrary.AssertCPMSReplicasAndConvergence(ctx, g.GinkgoT(), 3, "scale-down", cpmsClient, nodeClient)

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

			err = scalingtestinglibrary.EnsureMemberRemoved(g.GinkgoT(), etcdClientFactory, memberName)
			err = errors.Wrapf(err, "scale-down: timed out waiting for member (%v) to be removed", memberName)
			o.Expect(err).ToNot(o.HaveOccurred())

			scalingtestinglibrary.AssertVotingMemberAndMasterMachineCount(ctx, g.GinkgoT(), 3, "scale-down", kubeClient, machineClient, etcdClientFactory)
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
		g.It("is able to vertically scale up and down when CPMS is disabled [apigroup:machine.openshift.io]", func() {
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
			scalingtestinglibrary.AssertVotingMemberAndMasterMachineCount(ctx, g.GinkgoT(), 3, "scale-down", kubeClient, machineClient, etcdClientFactory)

			scalingtestinglibrary.AssertCPMSReplicasAndConvergence(ctx, g.GinkgoT(), 3, "scale-down", cpmsClient, nodeClient)
		})
	})

	g.Describe("[Suite:openshift/etcd/disruptive-scaling][Serial][Disruptive] etcd", func() {
		// The following test covers a vertical scaling scenario when a member is unhealthy.
		// This test validates that scale down happens before scale up if the deleted member is unhealthy.
		// CPMS is disabled to observe that scale-down happens first in this case.
		//
		// 1) If the CPMS is active, first disable it by deleting the CPMS custom resource.
		// 2) Remove the static pod manifest from a node and stop the kubelet on the node. This makes the member unhealthy.
		// 3) Delete the machine hosting the node in step 2.
		// 4) Verify the member removal and the total voting member count of 2 to ensure scale-down happens first when a member is unhealthy.
		// 5) Restore the initial cluster state by creating a new machine(scale-up) and re-enabling CPMS
		g.It("is able to vertically scale down when a member is unhealthy [apigroup:machine.openshift.io]", func() {
			framework.Logf("This test validates that scale-down happens before scale up if the deleted member is unhealthy")

			etcdNamespace := "openshift-etcd"

			if cpmsActive {
				//step 0: disable the CPMS
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

			// test-setup: select a master machine to delete, then find its associated node and the etcd pod running on it
			// pick a running master machine to be deleted
			machineToDelete, err := scalingtestinglibrary.AnyRunningMasterMachine(ctx, g.GinkgoT(), machineClient)
			err = errors.Wrap(err, "test-setup: failed to retrieve a running master machine for deletion")
			o.Expect(err).ToNot(o.HaveOccurred())

			// find the node hosted on the machine
			etcdTargetNodeName := machineToDelete.Status.NodeRef.Name
			o.Expect(etcdTargetNodeName).ToNot(o.BeNil(), fmt.Sprintf("test-setup: expected to find a NodeRef.Name for the machine %s, but it is nil", machineToDelete.Name))

			etcdTargetNode, err := nodeClient.Get(ctx, etcdTargetNodeName, metav1.GetOptions{})
			err = errors.Wrapf(err, "test-setup: failed to retrieve the node %s", etcdTargetNodeName)
			o.Expect(err).ToNot(o.HaveOccurred())

			// retrieve the pod on the etcdTargetNode that will be removed when the static pod manifest is moved from the node
			etcdPods, err := kubeClient.CoreV1().Pods(etcdNamespace).List(ctx, metav1.ListOptions{LabelSelector: "app=etcd", FieldSelector: "spec.nodeName=" + etcdTargetNodeName})
			err = errors.Wrapf(err, "test-setup: failed to retrieve the etcd pod on the node %s", etcdTargetNodeName)
			o.Expect(err).ToNot(o.HaveOccurred())
			etcdTargetPod := etcdPods.Items[0]

			// step 1: make a member unhealthy by removing the etcd static pod manifest from the node, then stopping the kubelet on that node
			framework.Logf("Removing the etcd static pod manifest from the node %s", etcdTargetNodeName)
			err = oc.AsAdmin().Run("debug").Args("-n", etcdNamespace, "node/"+etcdTargetNodeName, "--", "chroot", "/host", "/bin/bash", "-c", "rm /etc/kubernetes/manifests/etcd-pod.yaml").Execute()
			err = errors.Wrapf(err, "unhealthy member setup: failed to remove etcd static pod manifest from the node %s", etcdTargetNodeName)
			o.Expect(err).ToNot(o.HaveOccurred())

			err = e2epod.WaitForPodNotFoundInNamespace(ctx, kubeClient, etcdTargetPod.Name, etcdNamespace, 5*time.Minute)
			err = errors.Wrapf(err, "unhealthy member setup: timed-out waiting for etcd static pod %s on the node %s, to fully terminate", etcdTargetPod.Name, etcdTargetNodeName)
			o.Expect(err).ToNot(o.HaveOccurred())

			framework.Logf("Stopping the kubelet on the node %s", etcdTargetNodeName)
			err = scalingtestinglibrary.StopKubelet(ctx, oc.AdminKubeClient(), etcdTargetNode)
			err = errors.Wrapf(err, "unhealthy member setup: failed to stop the kubelet on the node %s", etcdTargetNodeName)
			o.Expect(err).ToNot(o.HaveOccurred())

			// step 2: delete the machine hosting the node that has unhealthy member
			err = scalingtestinglibrary.DeleteMachine(ctx, g.GinkgoT(), machineClient, machineToDelete.Name)
			o.Expect(err).ToNot(o.HaveOccurred())
			framework.Logf("Deleted machine %q", machineToDelete.Name)

			// step 3: wait for the machine pending deletion to have its member removed to indicate scale-down happens first when a member is unhealthy.
			framework.Logf("Waiting for etcd member %q to be removed", etcdTargetNodeName)
			err = scalingtestinglibrary.EnsureMemberRemoved(g.GinkgoT(), etcdClientFactory, etcdTargetNodeName)
			err = errors.Wrapf(err, "scale-down: timed out waiting for member (%v) to be removed", etcdTargetNodeName)
			o.Expect(err).ToNot(o.HaveOccurred())

			// wait for apiserver revision rollout to stabilize
			framework.Logf("Waiting for api servers to stabilize on the same revision")
			err = testlibraryapi.WaitForAPIServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc.KubeClient().CoreV1().Pods("openshift-kube-apiserver"))
			err = errors.Wrap(err, "scale-down: timed out waiting for APIServer pods to stabilize on the same revision")
			o.Expect(err).ToNot(o.HaveOccurred())

			// verify voting member count and master machine count also shows 2 to confirm scale-down happens first when the deleted member is unhealthy
			scalingtestinglibrary.AssertVotingMemberAndMasterMachineCount(ctx, g.GinkgoT(), 2, "scale-down", kubeClient, machineClient, etcdClientFactory)

			// step 4: restore to original state and observe scale-up, by creating a new machine that is a copy of the deleted machine
			newMachineName, err := scalingtestinglibrary.CreateNewMasterMachine(ctx, g.GinkgoT(), machineClient, machineToDelete)
			o.Expect(err).ToNot(o.HaveOccurred())
			framework.Logf("Created machine %q", newMachineName)

			err = scalingtestinglibrary.EnsureMasterMachine(ctx, g.GinkgoT(), newMachineName, machineClient)
			err = errors.Wrapf(err, "scale-up: timed out waiting for machine (%s) to become Running", newMachineName)
			o.Expect(err).ToNot(o.HaveOccurred())

			// wait for apiserver revision rollout to stabilize
			framework.Logf("Waiting for api servers to stabilize on the same revision")
			err = testlibraryapi.WaitForAPIServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc.KubeClient().CoreV1().Pods("openshift-kube-apiserver"))
			err = errors.Wrap(err, "scale-up: timed out waiting for APIServer pods to stabilize on the same revision")
			o.Expect(err).ToNot(o.HaveOccurred())

			// verify member and machine counts go back up to 3
			scalingtestinglibrary.AssertVotingMemberAndMasterMachineCount(ctx, g.GinkgoT(), 3, "scale-up", kubeClient, machineClient, etcdClientFactory)

			scalingtestinglibrary.AssertCPMSReplicasAndConvergence(ctx, g.GinkgoT(), 3, "scale-up", cpmsClient, nodeClient)
		})

		// The following test covers a vertical scaling scenario when kubelet is not working on a node.
		// This test validates that deleting the machine hosting the node where the kubelet is stopped doesn't get stuck when CPMS is enabled.
		//
		// CPMS should be active for this test scenario
		// 1) Stop the kubelet on a node
		// 2) Delete the machine hosting the node in step 2.
		// 3) That should prompt the ControlPlaneMachineSetOperator(CPMSO) to create a replacement
		//		machine and node for that machine index
		// 4) The operator will first scale-up the new machine's member
		// 5) Then scale-down happens, the machine that is pending deletion is removed
		g.It("is able to vertically scale up and down when kubelet is not running on a node[apigroup:machine.openshift.io]", func() {
			framework.Logf("This test validates that deleting the machine hosting the node where the kubelet is stopped doesn't get stuck when CPMS is enabled.")

			// pick a running master machine to be deleted
			machineToDelete, err := scalingtestinglibrary.AnyRunningMasterMachine(ctx, g.GinkgoT(), machineClient)
			err = errors.Wrap(err, "pre-test: failed to retrieve a running master machine for deletion")
			o.Expect(err).ToNot(o.HaveOccurred())

			// find the node hosted on the machine
			etcdTargetNodeName := machineToDelete.Status.NodeRef.Name
			o.Expect(etcdTargetNodeName).ToNot(o.BeNil(), fmt.Sprintf("pre-test: expected to find a NodeRef.Name for the machine %s, but it is nil", machineToDelete.Name))

			etcdTargetNode, err := nodeClient.Get(ctx, etcdTargetNodeName, metav1.GetOptions{})
			err = errors.Wrapf(err, "pre-test: failed to retrieve the node %s", etcdTargetNodeName)
			o.Expect(err).ToNot(o.HaveOccurred())

			// step 1: stop the kubelet on the node
			framework.Logf("Stopping the kubelet on the node %s", etcdTargetNodeName)
			err = scalingtestinglibrary.StopKubelet(ctx, oc.AdminKubeClient(), etcdTargetNode)
			err = errors.Wrapf(err, "failed to stop the kubelet on the node %s", etcdTargetNodeName)
			o.Expect(err).ToNot(o.HaveOccurred())

			// step 2: delete the machine on which kubelet is stopped to trigger the CPMSO to create a new one to replace it
			err = scalingtestinglibrary.DeleteMachine(ctx, g.GinkgoT(), machineClient, machineToDelete.Name)
			o.Expect(err).ToNot(o.HaveOccurred())
			framework.Logf("Deleted machine %q", machineToDelete.Name)

			// step 3: wait for the machine pending deletion to have its member removed to indicate scale-down
			framework.Logf("Waiting for etcd member %q to be removed", etcdTargetNodeName)
			err = scalingtestinglibrary.EnsureMemberRemoved(g.GinkgoT(), etcdClientFactory, etcdTargetNodeName)
			err = errors.Wrapf(err, "scale-down: timed out waiting for member (%v) to be removed", etcdTargetNodeName)
			o.Expect(err).ToNot(o.HaveOccurred())

			// step 4: wait for apiserver revision rollout to stabilize
			g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
			err = testlibraryapi.WaitForAPIServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc.KubeClient().CoreV1().Pods("openshift-kube-apiserver"))
			err = errors.Wrap(err, "scale-up: timed out waiting for APIServer pods to stabilize on the same revision")
			o.Expect(err).ToNot(o.HaveOccurred())

			// step 5: verify member and machine counts go back down to 3
			scalingtestinglibrary.AssertVotingMemberAndMasterMachineCount(ctx, g.GinkgoT(), 3, "scale-down", kubeClient, machineClient, etcdClientFactory)

			scalingtestinglibrary.AssertCPMSReplicasAndConvergence(ctx, g.GinkgoT(), 3, "scale-down", cpmsClient, nodeClient)
		})
	})
})
