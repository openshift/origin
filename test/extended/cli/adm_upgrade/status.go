package adm_upgrade

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("oc-adm-upgrade-status")
	f.SkipNamespaceCreation = true

	oc := exutil.NewCLIWithoutNamespace("oc-adm-upgrade-status").AsAdmin()

	g.It("reports correctly when the cluster is not updating", func() {
		cmd := oc.Run("adm", "upgrade", "status").EnvVar("OC_ENABLE_CMD_UPGRADE_STATUS", "true")
		out, err := cmd.Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("The cluster is not updating."))
	})
})
