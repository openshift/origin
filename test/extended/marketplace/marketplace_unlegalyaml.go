package marketplace

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Marketplace] [Serial] Marketplace unlegal yaml file test", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace", exutil.KubeConfigPath())
		marketplaceNs = "openshift-marketplace"

		opsrcYamltem = exutil.FixturePath("testdata", "marketplace", "opsrc", "03-opsrc.yaml")
		cscYamltem   = exutil.FixturePath("testdata", "marketplace", "csc", "03-csc.yaml")
	)

	g.AfterEach(func() {
	})

	// Create opsrc&csc failed by unlegal yaml file
	g.It("[OCP-21406] [OCP-21421] create opsrc&csc failed by unlegal yaml file", func() {

		// Create one opsrc with unlegal yaml file
		opsrcYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=wrongopsrc", "LABEL=wrongopsrc", "DISPLAYNAME=wrongopsrc", "PUBLISHER=wrongopsrc", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		opsrcmeg, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", opsrcYaml).Output()
		o.Expect(opsrcmeg).Should(o.ContainSubstring("is invalid"))
		o.Expect(opsrcmeg).Should(o.ContainSubstring("spec.endpoint"))
		o.Expect(opsrcmeg).Should(o.ContainSubstring("spec.registryNamespace"))
		o.Expect(opsrcmeg).Should(o.ContainSubstring("spec.type"))

		// Create one ocsc with unlegal yaml file
		cscYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cscYamltem, "-p", "NAME=wrongocsc", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs), "DISPLAYNAME=wrongocsc", "PUBLISHER=wrongocsc").OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		cscmeg, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", cscYaml).Output()
		o.Expect(cscmeg).Should(o.ContainSubstring("is invalid"))
		o.Expect(cscmeg).Should(o.ContainSubstring("spec.packages"))
		o.Expect(cscmeg).Should(o.ContainSubstring("spec.targetNamespace"))
	})
})
