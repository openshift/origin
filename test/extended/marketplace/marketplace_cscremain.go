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

var _ = g.Describe("[sig-operator] [Serial] [Feature:Marketplace] Marketplace cscremain", func() {

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
		// Clear the created resource
		allresourcelist := [][]string{
			{"operatorsource", "opsrcremaintest", marketplaceNs},
			{"catalogsourceconfig", "cscremaintest", marketplaceNs},
		}
		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	// Test the csc remain after the marketplace restart
	g.It("test the csc remain after restart the marketplace pod", func() {

		// Create one opsrc
		opsrcYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=opsrcremaintest", "NAMESPACE=marketplace_e2e", "LABEL=opsrcremaintest", "DISPLAYNAME=opsrcremaintest", "PUBLISHER=opsrcremaintest", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		// Wait for the opsrc is created finished
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("operatorsource", "opsrcremaintest", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create opsrcremaintest, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})

		o.Expect(err).NotTo(o.HaveOccurred())

		// Create one csc
		cscYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cscYamltem, "-p", "NAME=cscremaintest", fmt.Sprintf("NAMESPACE=%s", allNs), fmt.Sprintf("MARKETPLACE=%s", marketplaceNs), "PACKAGES=camel-k-marketplace-e2e-tests", "DISPLAYNAME=cscremaintest", "PUBLISHER=cscremaintest").OutputToFile("config.json")
		err = createResources(oc, cscYaml)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for the csc is created finished
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("catalogsourceconfig", "cscremaintest", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create cscremaintest, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})

		o.Expect(err).NotTo(o.HaveOccurred())

		// Get the podname of marketplace-operator
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", marketplaceNs, "-l name=marketplace-operator", "-oname").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Dlete pod/marketplace-operator-758c7d869b-hmkcj
		_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args(podName, "-n", marketplaceNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Check the status of csc cscremaintest
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsourceconfig", "cscremaintest", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create cscremaintest, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})

		cscResourceList := [][]string{
			{"catalogsourceconfig", "cscremaintest", marketplaceNs},
			{"deployment", "cscremaintest", marketplaceNs},
			{"catalogsource", "cscremaintest", allNs},
			{"service", "cscremaintest", marketplaceNs},
		}
		for _, source := range cscResourceList {
			msg, _ := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeTrue())
		}
	})
})
