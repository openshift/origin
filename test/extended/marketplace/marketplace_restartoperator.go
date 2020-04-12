package marketplace

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("[sig-operator][Feature:Marketplace] Marketplace restart from the bad status", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace")
		allNs         = "openshift-operators"
		marketplaceNs = "openshift-marketplace"
		resourceWait  = 600 * time.Second

		subYamltem = exutil.FixturePath("testdata", "marketplace", "sub", "01-subofdes.yaml")
	)

	g.AfterEach(func() {
		// Clear the resource
		allresourcelist := [][]string{
			{"subscription", "camel-k-marketplace-e2e-tests", allNs},
		}

		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	// Restart marketplace-operator from the bad status
	g.It("restart marketplace from the bat status [Serial]", func() {

		// Create one wrong sub without the source
		subYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", subYamltem, "-p", "NAME=camel-k-marketplace-e2e-tests", fmt.Sprintf("NAMESPACE=%s", allNs), "SOURCE=noexist", "CSV=camel-k-operator.v0.2.0").OutputToFile("config.json")
		err = createResources(oc, subYaml)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Delete the marketplace pod
		// Get the podname of marketplace-operator
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", marketplaceNs, "-l name=marketplace-operator", "-oname").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Dlete pod/marketplace-operator
		_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args(podName, "-n", marketplaceNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Check the marketplace restart status
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			restartStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "marketplace-operator", "-o=jsonpath={.status.availableReplicas}", "-n", marketplaceNs).Output()
			if err != nil {
				return false, err
			}
			if strings.Contains(restartStatus, "1") {
				return true, nil
			}
			return false, nil
		})
	})
})
