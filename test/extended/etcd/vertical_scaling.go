package etcd

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/pkg/errors"

	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	testlibraryapi "github.com/openshift/library-go/test/library/apiserver"

	scalingtestinglibrary "github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-etcd][Feature:EtcdVerticalScaling][Suite:openshift/etcd/scaling] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-scaling").AsAdmin()

	cleanupPlatformSpecificConfiguration := func() { /*noop*/ }

	g.BeforeEach(func() {
		cleanupPlatformSpecificConfiguration = scalingtestinglibrary.InitPlatformSpecificConfiguration(oc)
	})

	g.AfterEach(func() {
		cleanupPlatformSpecificConfiguration()
	})

	// The following test covers a basic vertical scaling scenario.
	// It starts by adding a new master machine to the cluster
	// next it validates the size of etcd cluster and makes sure the new member is healthy.
	// The test ends by removing the newly added machine and validating the size of the cluster
	// and asserting the member was removed from the etcd cluster by contacting MemberList API.
	g.It("is able to vertically scale up and down with a single node [Timeout:60m][apigroup:machine.openshift.io]", func() {
		// set up
		ctx := context.TODO()
		etcdClientFactory := scalingtestinglibrary.NewEtcdClientFactory(oc.KubeClient())
		machineClientSet, err := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).ToNot(o.HaveOccurred())
		machineClient := machineClientSet.MachineV1beta1().Machines("openshift-machine-api")
		nodeClient := oc.KubeClient().CoreV1().Nodes()
		cpmsClient := machineClientSet.MachineV1().ControlPlaneMachineSets("openshift-machine-api")
		kubeClient := oc.KubeClient()

		// make sure it can be run on the current platform
		scalingtestinglibrary.SkipIfUnsupportedPlatform(ctx, oc)

		// assert the cluster state before we run the test
		err = scalingtestinglibrary.EnsureInitialClusterState(ctx, g.GinkgoT(), etcdClientFactory, machineClient, kubeClient)
		err = errors.Wrap(err, "pre-test: timed out waiting for initial cluster state to have 3 running machines and 3 voting members")
		o.Expect(err).ToNot(o.HaveOccurred())

		cpmsActive, err := scalingtestinglibrary.IsCPMSActive(ctx, g.GinkgoT(), cpmsClient)
		err = errors.Wrap(err, "pre-test: failed to determine if ControlPlaneMachineSet is present and active")
		o.Expect(err).ToNot(o.HaveOccurred())

		if cpmsActive {
			// TODO: Add cleanup step to recover back to 3 running machines and members if the test fails

			framework.Logf("CPMS is active. Relying on CPMSO to replace the machine during vertical scaling")

			// step 1: delete a running machine to trigger the CPMSO to create a new one to replace it
			machineName, err := scalingtestinglibrary.DeleteSingleMachine(ctx, g.GinkgoT(), machineClient)
			o.Expect(err).ToNot(o.HaveOccurred())
			framework.Logf("Waiting for machine %q pending deletion to be replaced", machineName)

			memberName, err := scalingtestinglibrary.MachineNameToEtcdMemberName(ctx, oc.KubeClient(), machineClient, machineName)
			err = errors.Wrapf(err, "failed to get etcd member name for deleted machine: %v", machineName)
			o.Expect(err).ToNot(o.HaveOccurred())

			// step 2: wait until the CPMSO scales-up by creating a new machine
			// We need to check the cpms' status.readyReplicas because the phase of one machine will always be Deleting
			// so we can't use EnsureMasterMachinesAndCount() since that counts for machines that aren't pending deletion
			err = scalingtestinglibrary.EnsureReadyReplicasOnCPMS(ctx, g.GinkgoT(), 4, cpmsClient, nodeClient)
			err = errors.Wrap(err, "scale-up: timed out waiting for CPMS to show 4 ready replicas")
			o.Expect(err).ToNot(o.HaveOccurred())

			// We can't check for 4 members here as the clustermemberremoval controller will race to
			// remove the old member (from the machine pending deletion) as soon as the new machine's member
			// is promoted to a voting member.
			// Instead we just wait until the CPMS shows 3 replicas again which indicates that the new member was added
			// successfully

			// step 3: wait for automatic scale-down as the replica count goes back down to 3
			err = scalingtestinglibrary.EnsureReadyReplicasOnCPMS(ctx, g.GinkgoT(), 3, cpmsClient, nodeClient)
			err = errors.Wrap(err, "scale-down: timed out waiting for CPMS to show 3 ready replicas")
			o.Expect(err).ToNot(o.HaveOccurred())

			err = scalingtestinglibrary.EnsureVotingMembersCount(ctx, g.GinkgoT(), etcdClientFactory, kubeClient, 3)
			err = errors.Wrap(err, "scale-down: timed out waiting for 3 voting members in the etcd cluster and etcd-endpoints configmap")
			o.Expect(err).ToNot(o.HaveOccurred())

			err = scalingtestinglibrary.EnsureMemberRemoved(g.GinkgoT(), etcdClientFactory, memberName)
			err = errors.Wrapf(err, "scale-down: timed out waiting for member (%v) to be removed", memberName)
			o.Expect(err).ToNot(o.HaveOccurred())

			err = scalingtestinglibrary.EnsureMasterMachinesAndCount(ctx, g.GinkgoT(), machineClient)
			err = errors.Wrap(err, "scale-down: timed out waiting for only 3 Running master machines")
			o.Expect(err).ToNot(o.HaveOccurred())

			// step 4: Wait for apiserver revision rollout to stabilize
			g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
			err = testlibraryapi.WaitForAPIServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc.KubeClient().CoreV1().Pods("openshift-kube-apiserver"))
			err = errors.Wrap(err, "scale-up: timed out waiting for APIServer pods to stabilize on the same revision")
			o.Expect(err).ToNot(o.HaveOccurred())

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
		machineName, err := scalingtestinglibrary.CreateNewMasterMachine(ctx, g.GinkgoT(), machineClient)
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
})
