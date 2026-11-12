package dr

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	g.It("[Feature:EtcdRecovery][Disruptive] Restore snapshot from node on another single unhealthy node", func() {
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

	g.It("[Feature:EtcdRecovery][Disruptive] Recover with snapshot with two unhealthy nodes and lost quorum", func() {
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

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery][Timeout:3h]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("recovery")
	f.SkipNamespaceCreation = true
	oc := exutil.NewCLIWithoutNamespace("recovery")

	var recoveryCleanupNeeded bool
	g.AfterEach(func() {
		if !recoveryCleanupNeeded {
			return
		}
		defer func() { recoveryCleanupNeeded = false }()
		defer func() {
			if err := removePostBackupResources(oc); err != nil && !apierrors.IsNotFound(err) {
				g.GinkgoT().Errorf("could not clean up post-backup resources: %v", err)
			}
		}()

		g.GinkgoT().Log("turning the quorum guard back on")
		data := fmt.Sprintf(`{"spec": {"unsupportedConfigOverrides": {"useUnsupportedUnsafeNonHANonProductionUnstableEtcd": false}}}`)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		cancel()
		o.Expect(err).ToNot(o.HaveOccurred(), "disable recovery quorum guard override")

		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = waitForApiServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for APIServer pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred(), "wait for API server stability during recovery cleanup")

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred(), "wait for etcd stability during recovery cleanup")
	})

	g.It("[Feature:EtcdRecovery][OCPFeatureGate:AutomatedEtcdBackup][apigroup:operator.openshift.io][Skipped:SingleReplicaTopology][Disruptive] Recover from EtcdBackup with two unhealthy nodes and lost quorum", func() {
		supported, err := supportsEtcdBackupRecoveryAPI(oc)
		o.Expect(err).ToNot(o.HaveOccurred(), "inspect EtcdBackup API")
		if !supported {
			g.Skip("the EtcdBackup CRD is absent or uses the legacy PVC API without local recovery backups")
		}

		recoveryCleanupNeeded = true
		data := fmt.Sprintf(`{"spec": {"unsupportedConfigOverrides": {"useUnsupportedUnsafeNonHANonProductionUnstableEtcd": true}}}`)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, err = oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		cancel()
		o.Expect(err).ToNot(o.HaveOccurred(), "enable recovery quorum guard override")

		err = waitForApiServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		o.Expect(err).ToNot(o.HaveOccurred(), "wait for API server stability")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		o.Expect(err).ToNot(o.HaveOccurred(), "wait for etcd stability")

		g.DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := oc.AdminKubeClient().CoreV1().Pods(openshiftEtcdNamespace).Delete(ctx, "dr-ssh", metav1.DeleteOptions{})
			cancel()
			if err != nil && !apierrors.IsNotFound(err) {
				g.GinkgoT().Errorf("could not clean up SSH test pod: %v", err)
			}

			ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
			err = oc.AdminKubeClient().AppsV1().DaemonSets(openshiftEtcdNamespace).Delete(ctx, "dr-ssh", metav1.DeleteOptions{})
			cancel()
			if err != nil && !apierrors.IsNotFound(err) {
				g.GinkgoT().Errorf("could not clean up SSH installer DaemonSet: %v", err)
			}

			ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
			err = oc.AdminKubeClient().CoreV1().Secrets(openshiftEtcdNamespace).Delete(ctx, "dr-ssh", metav1.DeleteOptions{})
			cancel()
			if err != nil && !apierrors.IsNotFound(err) {
				g.GinkgoT().Errorf("could not clean up SSH key Secret: %v", err)
			}
		})
		err = InstallSSHKeyOnControlPlaneNodes(oc)
		o.Expect(err).ToNot(o.HaveOccurred(), "prepare SSH access to control plane nodes")

		masters := masterNodes(oc)
		o.Expect(len(masters)).To(o.BeNumerically(">=", 3), "recovery requires at least three control plane nodes")
		backupNode := masters[0]
		recoveryNode := masters[1]
		nonRecoveryNodes := make([]*corev1.Node, 0, len(masters)-1)
		for _, node := range masters {
			if node.Name != recoveryNode.Name {
				nonRecoveryNodes = append(nonRecoveryNodes, node)
			}
		}

		backupName, err := createEtcdBackup(oc, backupNode)
		g.DeferCleanup(func() {
			if err := deleteEtcdBackup(oc, backupName); err != nil {
				framework.Logf("could not clean up EtcdBackup %q: %v", backupName, err)
			}
		})
		o.Expect(err).ToNot(o.HaveOccurred(), "create EtcdBackup")

		files, err := waitForEtcdBackup(oc, backupName, backupNode)
		o.Expect(err).ToNot(o.HaveOccurred(), "wait for EtcdBackup files")
		g.DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := oc.AdminKubeClient().CoreV1().Pods(openshiftEtcdNamespace).Delete(ctx, etcdBackupStagePodName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				g.GinkgoT().Errorf("could not clean up backup staging pod: %v", err)
			}
		})
		err = stageEtcdBackupFiles(oc, backupNode, files)
		o.Expect(err).ToNot(o.HaveOccurred(), "stage EtcdBackup files")

		err = createPostBackupResources(oc)
		o.Expect(err).ToNot(o.HaveOccurred(), "create post-backup resources")

		g.DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := oc.AdminKubeClient().CoreV1().Pods(openshiftEtcdNamespace).Delete(ctx, "bumping-etcd-restore-pod", metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				g.GinkgoT().Errorf("could not clean up recovery pod: %v", err)
			}
		})
		err = runClusterRestoreScript(oc, recoveryNode, backupNode, nonRecoveryNodes)
		o.Expect(err).ToNot(o.HaveOccurred(), "start cluster restore")

		waitForReadyEtcdStaticPods(oc.AdminKubeClient(), 1)
		forceOperandRedeployment(oc.AdminOperatorClient().OperatorV1())
		waitForReadyEtcdStaticPods(oc.AdminKubeClient(), len(masters))
		waitForOperatorsToSettle()
		assertPostBackupResourcesAreNotFound(oc)

		err = deleteEtcdBackup(oc, backupName)
		o.Expect(err).ToNot(o.HaveOccurred(), "delete EtcdBackup")
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

	g.It("[Feature:EtcdRecovery][Disruptive] Recover with quorum restore", func() {
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
