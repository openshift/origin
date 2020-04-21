package marketplace

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-operator][Feature:Marketplace] Marketplace watch the sources", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace")
		allNs         = "openshift-operators"
		marketplaceNs = "openshift-marketplace"
		timeWait      = 300

		opsrcYamltem = exutil.FixturePath("testdata", "marketplace", "opsrc", "02-opsrc.yaml")
		cscYamltem   = exutil.FixturePath("testdata", "marketplace", "csc", "02-csc.yaml")
	)

	g.AfterEach(func() {
		// Clear the resource
		allresourcelist := [][]string{
			{"operatorsource", "watchopsrc", marketplaceNs},
			{"catalogsourceconfig", "watchcsc", marketplaceNs},
		}

		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	// Watch the sources of opsrc and csc
	g.It("check the child source of opsrc and csc [Serial]", func() {

		// Create one opsrc watchopsrc
		opsrcYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=watchopsrc", "NAMESPACE=marketplace_e2e", "LABEL=watchopsrc", "DISPLAYNAME=watchopsrc", "PUBLISHER=watchopsrc", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for the opsrc is created finished
		err = waitForSource(oc, "operatorsource", "watchopsrc", timeWait, marketplaceNs)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Create the csc watchcsc
		cscYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cscYamltem, "-p", "NAME=watchcsc", fmt.Sprintf("NAMESPACE=%s", allNs), fmt.Sprintf("MARKETPLACE=%s", marketplaceNs), "PACKAGES=camel-k-marketplace-e2e-tests", "DISPLAYNAME=watchcsc", "PUBLISHER=watchcsc").OutputToFile("config.json")
		err = createResources(oc, cscYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		// Wait for the csc is created finished
		err = waitForSource(oc, "catalogsourceconfig", "watchcsc", timeWait, marketplaceNs)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Delete the catalogsource service deployment of watchopsrc and watchcsc
		resourceList := [][]string{
			{"catalogsource", "watchopsrc", marketplaceNs},
			{"service", "watchopsrc", marketplaceNs},
			{"deployment", "watchopsrc", marketplaceNs},
			{"catalogsource", "watchcsc", allNs},
			{"service", "watchcsc", marketplaceNs},
			{"deployment", "watchcsc", marketplaceNs},
		}
		for _, source := range resourceList {
			err = clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		// Check the sources recovered
		for _, source := range resourceList {
			msg, _ := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeTrue())
		}
	})
})
