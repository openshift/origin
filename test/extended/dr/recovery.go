package dr

import (
	"context"
	"fmt"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery][Timeout:30m]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("recovery")
	f.SkipNamespaceCreation = true
	oc := exutil.NewCLIWithoutNamespace("recovery")

	g.AfterEach(func() {
		g.GinkgoT().Log("turning the quorum guard back on")
		data := fmt.Sprintf(`{"spec": {"unsupportedConfigOverrides": {"useUnsupportedUnsafeNonHANonProductionUnstableEtcd": false}}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(context.Background(), "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting to delete post backup resources....")
		err = removePostBackupResources(oc)
		err = errors.Wrap(err, "post backup resources cleanup timed out")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("[Feature:EtcdRecovery][Disruptive] Restore snapshot from node on another single unhealthy node", g.Label("Size:L"), func() {
		// ensure the CEO can still act without quorum, doing it first so the CEO can cycle while we install ssh keys
		data := fmt.Sprintf(`{"spec": {"unsupportedConfigOverrides": {"useUnsupportedUnsafeNonHANonProductionUnstableEtcd": true}}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(context.Background(), "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		err = InstallSSHKeyOnControlPlaneNodes(oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		masters := masterNodes(oc)
		// Need one node to back up from and another to restore to
		o.Expect(len(masters)).To(o.BeNumerically(">=", 3))

		// Pick one node to back up on
		backupNode := masters[0]
		framework.Logf("Selecting node %q as the backup host", backupNode.Name)

		// Pick a different node to recover on
		recoveryNode := masters[1]
		framework.Logf("Selecting node %q as the recovery host", recoveryNode.Name)

		err = runClusterBackupScript(oc, backupNode)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = createPostBackupResources(oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		// remove the etcd recovery member before we're starting to restore
		err = removeMemberOfNode(oc, recoveryNode)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = runSnapshotRestoreScript(oc, recoveryNode, backupNode)
		o.Expect(err).ToNot(o.HaveOccurred())

		forceOperandRedeployment(oc.AdminOperatorClient().OperatorV1())

		waitForReadyEtcdStaticPods(oc.AdminKubeClient(), len(masters))
		waitForOperatorsToSettle()

		framework.Logf("asserting post backup resources are still found...")
		assertPostBackupResourcesAreStillFound(oc)
		framework.Logf("asserting post backup resources are still functional...")
		assertPostBackupResourcesAreStillFunctional(oc)
	})
})

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery][Timeout:2h]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("recovery")
	f.SkipNamespaceCreation = true
	oc := exutil.NewCLIWithoutNamespace("recovery")

	g.AfterEach(func() {
		g.GinkgoT().Log("turning the quorum guard back on")
		data := fmt.Sprintf(`{"spec": {"unsupportedConfigOverrides": {"useUnsupportedUnsafeNonHANonProductionUnstableEtcd": false}}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(context.Background(), "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// we need to ensure this test also ends with a stable revision for api and etcd
		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = waitForApiServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for APIServer pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("[Feature:EtcdRecovery][Disruptive] Recover with snapshot with two unhealthy nodes and lost quorum", g.Label("Size:L"), func() {
		// ensure the CEO can still act without quorum, doing it first so the CEO can cycle while we install ssh keys
		data := fmt.Sprintf(`{"spec": {"unsupportedConfigOverrides": {"useUnsupportedUnsafeNonHANonProductionUnstableEtcd": true}}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(context.Background(), "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// we need to ensure each test starts with a stable revision for api and etcd
		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = waitForApiServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for APIServer pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		err = InstallSSHKeyOnControlPlaneNodes(oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		masters := masterNodes(oc)
		o.Expect(len(masters)).To(o.BeNumerically(">=", 3))
		backupNode := masters[0]
		framework.Logf("Selecting node %q as the backup host", backupNode.Name)
		recoveryNode := masters[1]
		framework.Logf("Selecting node %q as the recovery host", recoveryNode.Name)
		nonRecoveryNodes := []*corev1.Node{backupNode, masters[2]}

		err = runClusterBackupScript(oc, backupNode)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = createPostBackupResources(oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		// From here on out we're going to leave the restore orchestration to a pod running on the recovery node.
		// While it makes it more difficult to gather debug information during that time, it's much easier
		// to test on different platforms due to ssh constraints. Previous approaches with bastions were not scalable.

		// The pod will attempt to run the following steps:
		// - stop/move etcd static pods on all control plane nodes
		// - copying the snapshot tarball from the backup node to the recovery node
		// - run the restore script, which results in running a single node etcd cluster from backup
		// During the whole time, the API won't be responsive, and we're just waiting for the apiserver to come back up.
		err = runClusterRestoreScript(oc, recoveryNode, backupNode, nonRecoveryNodes)
		o.Expect(err).ToNot(o.HaveOccurred())

		// we should come back with a single etcd static pod
		waitForReadyEtcdStaticPods(oc.AdminKubeClient(), 1)
		forceOperandRedeployment(oc.AdminOperatorClient().OperatorV1())
		// CEO will bring back the other etcd static pods again
		waitForReadyEtcdStaticPods(oc.AdminKubeClient(), len(masters))
		waitForOperatorsToSettle()

		framework.Logf("asserting post backup resources are not found anymore...")
		assertPostBackupResourcesAreNotFound(oc)
	})
})

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery][Timeout:1h]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("recovery")
	f.SkipNamespaceCreation = true
	oc := exutil.NewCLIWithoutNamespace("recovery")

	g.AfterEach(func() {
		g.GinkgoT().Log("turning the quorum guard back on")
		data := fmt.Sprintf(`{"spec": {"unsupportedConfigOverrides": {"useUnsupportedUnsafeNonHANonProductionUnstableEtcd": false}}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(context.Background(), "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// we need to ensure this test also ends with a stable revision for api and etcd
		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = waitForApiServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for APIServer pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("[Feature:EtcdRecovery][Disruptive] Recover with quorum restore", g.Label("Size:L"), func() {
		// ensure the CEO can still act without quorum, doing it first so the CEO can cycle while we install ssh keys
		data := fmt.Sprintf(`{"spec": {"unsupportedConfigOverrides": {"useUnsupportedUnsafeNonHANonProductionUnstableEtcd": true}}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(context.Background(), "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// we need to ensure each test starts with a stable revision for api and etcd
		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = waitForApiServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for APIServer pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		err = InstallSSHKeyOnControlPlaneNodes(oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		masters := masterNodes(oc)
		o.Expect(len(masters)).To(o.BeNumerically(">=", 3))
		recoveryNode := masters[2]

		err = runQuorumRestoreScript(oc, recoveryNode)
		o.Expect(err).ToNot(o.HaveOccurred())

		forceOperandRedeployment(oc.AdminOperatorClient().OperatorV1())
		// CEO will bring back the other etcd static pods again
		waitForReadyEtcdStaticPods(oc.AdminKubeClient(), len(masters))
		waitForOperatorsToSettle()
	})
})
