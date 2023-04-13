package dr

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
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
	})

	g.It("[Feature:EtcdRecovery][Disruptive] Cluster should recover from backup of another node", func() {
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

		err = runClusterRestoreScript(oc, recoveryNode, backupNode)
		o.Expect(err).ToNot(o.HaveOccurred())

		// TODO(thomas): that's not all the validation from the old test
		waitForReadyEtcdPods(oc.AdminKubeClient(), len(masters))
		waitForOperatorsToSettle()

		// TODO(thomas): wrap in disruption testing
	})
})
