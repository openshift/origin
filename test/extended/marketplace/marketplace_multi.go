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

var _ = g.Describe("[Feature:Marketplace] Marketplace support multi format operators", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace", exutil.KubeConfigPath())
		marketplaceNs = "openshift-marketplace"
		resourceWait  = 90 * time.Second
		timeWait      = 90

		opsrcYamltem = exutil.FixturePath("testdata", "marketplace", "opsrc", "02-opsrc.yaml")
	)

	g.AfterEach(func() {
		// Clear the resource
		allresourcelist := [][]string{
			{"operatorsource", "multiformat", marketplaceNs},
		}

		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	// OCP-23090 check the multi format operators support
	g.It("[ocp-23090 25675]create opsrc with multi format operators and operator with error", func() {

		// OCP-25675 create the opsrc that has the flatted & nested operator and one operator has error
		opsrcYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=multiformat", "NAMESPACE=kaka", "LABEL=multiformat", "DISPLAYNAME=multiformat", "PUBLISHER=multiformat", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for the opsrc is created finished
		err = waitForSource(oc, "operatorsource", "multiformat", timeWait, marketplaceNs)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for deployment is ready
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("deployment", "multiformat", "-o=jsonpath={.status.readyReplicas}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("The deployment multiformat is wrong, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "1") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		podNameMult, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", marketplaceNs, "-l marketplace.operatorSource=multiformat", "-oname").Output()
		logOfMult, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podNameMult, "-n", marketplaceNs).Output()
		o.Expect(logOfMult).Should(o.ContainSubstring("decoded 1 flattened and 1 nested"))
		o.Expect(logOfMult).Should(o.ContainSubstring("error loading manifests from appregistry"))
	})
})
