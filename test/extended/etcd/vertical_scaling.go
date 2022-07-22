package etcd

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	machineclient "github.com/openshift/client-go/machine/clientset/versioned"

	scalingtestinglibrary "github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-etcd][Serial] etcd", func() {
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
	// The test ends by removing the newly added machine and validating the size of the cluster.
	g.It("is able to vertically scale up and down with a single node [Timeout:60m]", func() {
		// set up
		ctx := context.TODO()
		machineClientSet, err := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).ToNot(o.HaveOccurred())
		machineClient := machineClientSet.MachineV1beta1().Machines("openshift-machine-api")
		kubeClient := oc.KubeClient()

		// make sure it can be run on the current platform
		scalingtestinglibrary.SkipIfUnsupportedPlatform(ctx, oc)

		// assert the cluster state before we run the test
		err = scalingtestinglibrary.EnsureInitialClusterState(ctx, g.GinkgoT(), machineClient, kubeClient)
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 0: ensure clean state after the test
		defer func() {
			g.GinkgoT().Log("cleaning routine: ensuring initial cluster state")
			err = scalingtestinglibrary.EnsureInitialClusterState(ctx, g.GinkgoT(), machineClient, kubeClient)
			o.Expect(err).ToNot(o.HaveOccurred())
		}()

		// step 1: add a new master node and wait until it is in Running state
		machineName, err := scalingtestinglibrary.CreateNewMasterMachine(ctx, g.GinkgoT(), machineClient)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = scalingtestinglibrary.EnsureMasterMachine(ctx, g.GinkgoT(), machineName, machineClient)
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 2: wait until a new member shows up and check if it is healthy
		//         and until all kube-api servers have reached the same revision
		//         this additional step is the best-effort of ensuring they
		//         have observed the new member before disruption
		err = scalingtestinglibrary.EnsureVotingMembersCount(ctx, g.GinkgoT(), kubeClient, 4)
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 3: clean-up: delete the machine and wait until etcd member is removed from the etcd cluster
		err = machineClient.Delete(ctx, machineName, metav1.DeleteOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		framework.Logf("successfully deleted the machine %q from the API", machineName)
		err = scalingtestinglibrary.EnsureVotingMembersCount(ctx, g.GinkgoT(), kubeClient, 3)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = scalingtestinglibrary.EnsureMasterMachinesAndCount(ctx, g.GinkgoT(), machineClient)
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})
