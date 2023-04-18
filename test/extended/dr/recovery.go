package dr

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("recovery")
	f.SkipNamespaceCreation = true

	oc := exutil.NewCLIWithoutNamespace("recovery")

	g.BeforeEach(func() {
		err := InstallSSHKeyOnControlPlaneNodes(oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		// ensure the CEO can still act with 2 nodes
		data := fmt.Sprintf(`{"spec": {"unsupportedConfigOverrides": {"useUnsupportedUnsafeNonHANonProductionUnstableEtcd": true}}}`)
		_, err = oc.AdminOperatorClient().OperatorV1().Etcds().Patch(context.Background(), "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.AfterEach(func() {
		// enable the quorum check again for any other tests that come after
		data := fmt.Sprintf(`{"spec": {}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(context.Background(), "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("[Feature:EtcdRecovery][Disruptive] Restore snapshot from node on another single unhealthy node", func() {
		masters := masterNodes(oc)
		// Need one node to back up from and another to restore to
		o.Expect(len(masters)).To(o.BeNumerically(">=", 2))

		// Pick one node to back up on
		backupNode := masters[0]
		framework.Logf("Selecting node %q as the backup host", backupNode.Name)

		// Pick a different node to recover on
		recoveryNode := masters[1]
		framework.Logf("Selecting node %q as the recovery host", recoveryNode.Name)

		err := runClusterBackupScript(oc, backupNode)
		o.Expect(err).ToNot(o.HaveOccurred())

		// remove the etcd recovery member before we're starting to restore
		err = removeMemberOfNode(oc, recoveryNode)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = runClusterRestoreScript(oc, recoveryNode, backupNode)
		o.Expect(err).ToNot(o.HaveOccurred())

		forceOperandRedeployment(oc.AdminOperatorClient().OperatorV1())

		// TODO(thomas): that's not all the validation from the old test
		waitForReadyEtcdStaticPods(oc.AdminKubeClient(), len(masters))
		waitForOperatorsToSettle()

		// TODO(thomas): wrap in disruption testing
	})
})

func waitForReadyEtcdStaticPods(client kubernetes.Interface, masterCount int) {
	g.By("Waiting for all etcd static pods to become ready")
	waitForPodsTolerateClientTimeout(
		client.CoreV1().Pods("openshift-etcd"),
		exutil.ParseLabelsOrDie("app=etcd"),
		exutil.CheckPodIsRunning,
		masterCount,
		40*time.Minute,
	)
}
