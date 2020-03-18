package marketplace

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operator][Feature:Marketplace] [Serial] Marketplace retry the opsrc and csc logical test", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace", exutil.KubeConfigPath())
		allNs         = "openshift-operators"
		marketplaceNs = "openshift-marketplace"
		resourceWait  = 300 * time.Second

		opsrcYamltem = exutil.FixturePath("testdata", "marketplace", "opsrc", "02-opsrc.yaml")
		cscYamltem   = exutil.FixturePath("testdata", "marketplace", "csc", "02-csc.yaml")
	)

	g.AfterEach(func() {
		// Clear the resource
		allresourcelist := [][]string{
			{"operatorsource", "emptyopsrc", marketplaceNs},
			{"operatorsource", "opsrce2e", marketplaceNs},
			{"catalogsourceconfig", "retrycsc", marketplaceNs},
		}

		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	//
	g.It("check the retry logical of opsrc and csc", func() {
		// Create csc "retrycsc" and the package "camel-k-marketplace-e2e-tests" that doesn't exist
		cscYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cscYamltem, "-p", "NAME=retrycsc", fmt.Sprintf("NAMESPACE=%s", allNs), fmt.Sprintf("MARKETPLACE=%s", marketplaceNs), "PACKAGES=camel-k-marketplace-e2e-tests", "DISPLAYNAME=retrycsc", "PUBLISHER=retrycsc").OutputToFile("config.json")
		err = createResources(oc, cscYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		// Check the csc status
		outMesg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsourceconfig", "retrycsc", "-o=jsonpath={.status.currentPhase.phase.name}", "-n", marketplaceNs).Output()
		o.Expect(outMesg).Should(o.ContainSubstring("Configuring"))

		// Create opsrc contains the package "camel-k-marketplace-e2e-tests" 
		opsrcYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=opsrce2e", "NAMESPACE=marketplace_e2e", "LABEL=opsrce2e", "DISPLAYNAME=opsrce2e", "PUBLISHER=opsrce2e", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for the opsrc is created finished
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("operatorsource", "opsrce2e", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create opsrce2e, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for the csc is created finished
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("catalogsourceconfig", "retrycsc", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create csctestlabel, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Create opsrc "emptyopsrc" that the return packages list is empty
		opsrcYaml1, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=emptyopsrc", "NAMESPACE=noexistappregistory", "LABEL=emptyopsrc", "DISPLAYNAME=emptyopsrc", "PUBLISHER=emptyopsrc", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = createResources(oc, opsrcYaml1)
		o.Expect(err).NotTo(o.HaveOccurred())

		outStatus, err := oc.AsAdmin().Run("get").Args("operatorsource", "emptyopsrc", "-o=jsonpath={.status.currentPhase.phase.name}", "-n", marketplaceNs).Output()
		o.Expect(outStatus).Should(o.ContainSubstring("Failed"))
	})
})
