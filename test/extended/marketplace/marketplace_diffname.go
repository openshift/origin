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

var _ = g.Describe("[Feature:Marketplace] Marketplace diff name test", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace", exutil.KubeConfigPath())
		allNs         = "openshift-operators"
		marketplaceNs = "openshift-marketplace"
		resourceWait  = 60 * time.Second

		opsrcYamltem = exutil.FixturePath("testdata", "marketplace", "opsrc", "02-opsrc.yaml")
		cscYamltem   = exutil.FixturePath("testdata", "marketplace", "csc", "02-csc.yaml")
	)

	g.AfterEach(func() {
		//clear the resource
		allresourcelist := [][]string{
			{"operatorsource", "samename", marketplaceNs},
			{"catalogsourceconfig", "samename", marketplaceNs},
		}

		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	//OCP-25672 create a opsrc named "samename", then create a csc also named "samename"
	g.It("[ocp-25672] create the samename opsrc&csc", func() {

		//create one opsrc samename
		opsrcYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=samename", "NAMESPACE=marketplace_e2e", "LABEL=samename", "DISPLAYNAME=samename", "PUBLISHER=samename", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		//wait for the opsrc is created finished
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("operatorsource", "samename", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create samename, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})

		o.Expect(err).NotTo(o.HaveOccurred())

		opsrcResourceList := [][]string{
			{"operatorsource", "samename", marketplaceNs},
			{"deployment", "samename", marketplaceNs},
			{"catalogsource", "samename", marketplaceNs},
			{"service", "samename", marketplaceNs},
		}
		for _, source := range opsrcResourceList {
			msg, _ := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeTrue())
		}

		// create the csc samename
		cscYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cscYamltem, "-p", "NAME=samename", fmt.Sprintf("NAMESPACE=%s", allNs), fmt.Sprintf("MARKETPLACE=%s", marketplaceNs), "PACKAGES=camel-k-marketplace-e2e-tests", "DISPLAYNAME=samename", "PUBLISHER=samename").OutputToFile("config.json")
		err = createResources(oc, cscYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		//check the csc status
		outMesg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsourceconfig", "samename", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
		o.Expect(outMesg).Should(o.ContainSubstring("Deployment samename exists"))
		outStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsourceconfig", "samename", "-o=jsonpath={.status.currentPhase.phase.name}", "-n", marketplaceNs).Output()
		o.Expect(outStatus).Should(o.ContainSubstring("Configuring"))
	})
})
