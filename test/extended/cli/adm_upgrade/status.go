package adm_upgrade

import (
	"context"

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

	g.It("reports correctly when the cluster is not updating", g.Label("Size:S"), func() {

		// CLI-side oc adm upgrade status does not support HyperShift (assumes MCPs, ignores NodePools, has no knowledge of
		// management / hosted cluster separation)
		isHyperShift, err := exutil.IsHypershift(context.TODO(), oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isHyperShift {
			g.Skip("oc adm upgrade status is not supported on HyperShift")
		}

		cmd := oc.Run("adm", "upgrade", "status")
		out, err := cmd.Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("The cluster is not updating."))
	})
})
