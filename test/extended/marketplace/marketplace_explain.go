package marketplace

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Marketplace] Marketplace oc explain the opsrc & csc", func() {

	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("marketplace", exutil.KubeConfigPath())
	)

	//OCP-23670 description info for opsrc&csc CRDs
	g.It("ensure the opsrc & csc can be explain by oc command", func() {

		explainOpsrc, _ := oc.AsAdmin().WithoutNamespace().Run("explain").Args("operatorsource").Output()
		o.Expect(explainOpsrc).ShouldNot(o.ContainSubstring("<empty>"))
		explainCsc, _ := oc.AsAdmin().WithoutNamespace().Run("explain").Args("catalogsourceconfig").Output()
		o.Expect(explainCsc).ShouldNot(o.ContainSubstring("<empty>"))
	})
})
