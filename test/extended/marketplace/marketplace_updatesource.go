package marketplace

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-operator][Feature:Marketplace] Marketplace update the source", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace", exutil.KubeConfigPath())
		allNs         = "openshift-operators"
		marketplaceNs = "openshift-marketplace"
		updateNs      = "default"
		timeWait      = 90

		opsrcYamltem = exutil.FixturePath("testdata", "marketplace", "opsrc", "02-opsrc.yaml")
		cscYamltem   = exutil.FixturePath("testdata", "marketplace", "csc", "02-csc.yaml")
	)

	g.AfterEach(func() {
		// Clear the resource
		allresourcelist := [][]string{
			{"operatorsource", "updateopsrc", marketplaceNs},
			{"catalogsourceconfig", "updatecsc", marketplaceNs},
		}

		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	// Update one exist opsrc and csc
	g.It("update the exist opsrc and csc [Serial]", func() {

		// Create one opsrc updateopsrc
		opsrcYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=updateopsrc", "NAMESPACE=marketplace_e2e", "LABEL=updateopsrc", "DISPLAYNAME=updateopsrc", "PUBLISHER=updateopsrc", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for the opsrc is created finished
		err = waitForSource(oc, "operatorsource", "updateopsrc", timeWait, marketplaceNs)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Create the csc updatecsc
		cscYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cscYamltem, "-p", "NAME=updatecsc", fmt.Sprintf("NAMESPACE=%s", allNs), fmt.Sprintf("MARKETPLACE=%s", marketplaceNs), "PACKAGES=camel-k-marketplace-e2e-tests", "DISPLAYNAME=updatecsc", "PUBLISHER=updatecsc").OutputToFile("config.json")
		err = createResources(oc, cscYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		// Wait for the csc is created finished
		err = waitForSource(oc, "catalogsourceconfig", "updatecsc", timeWait, marketplaceNs)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Update the csc packagelist and the target namespaces
		_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("catalogsourceconfig/updatecsc", "-p", `{"spec":{"packages": "cockroachdb-marketplace-e2e-tests"}}`, "--type=merge", "-n", marketplaceNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("catalogsourceconfig/updatecsc", "-p", `{"spec":{"targetNamespace": "default"}}`, "--type=merge", "-n", marketplaceNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Check the csc status
		err = waitForSource(oc, "catalogsourceconfig", "updatecsc", timeWait, marketplaceNs)
		o.Expect(err).NotTo(o.HaveOccurred())
		// Get the result with the updated csc
		cataOut, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "-lcsc-owner-name=updatecsc", "-o=name", "-n", updateNs).Output()
		o.Expect(cataOut).Should(o.ContainSubstring("updatecsc"))
		packagelist, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsourceconfig", "updatecsc", "-o=jsonpath={.status.packageRepositioryVersions}", "-n", marketplaceNs).Output()
		o.Expect(packagelist).Should(o.ContainSubstring("cockroachdb-marketplace"))

		// Update the endpoint of opsrc to an invalid one
		_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("operatorsource/updateopsrc", "-p", `{"spec":{"registryNamespace": "noExistRegistoryforTest"}}`, "--type=merge", "-n", marketplaceNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		outStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("operatorsource", "updateopsrc", "-o=jsonpath={.status.currentPhase.phase.name}", "-n", marketplaceNs).Output()
		o.Expect(outStatus).Should(o.ContainSubstring("Failed"))

		// Update the endpoint to marketplace_e2e
		_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("operatorsource/updateopsrc", "-p", `{"spec":{"registryNamespace": "marketplace_e2e"}}`, "--type=merge", "-n", marketplaceNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForSource(oc, "operatorsource", "updateopsrc", timeWait, marketplaceNs)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

})
