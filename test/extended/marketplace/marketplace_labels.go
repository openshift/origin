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

var _ = g.Describe("[sig-operator][Feature:Marketplace] Marketplace resources with labels provider displayName", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace")
		marketplaceNs = "openshift-marketplace"
		resourceWait  = 2 * time.Minute

		opsrcYamltem = exutil.FixturePath("testdata", "marketplace", "opsrc", "02-opsrc.yaml")
	)

	g.AfterEach(func() {
		// Clear the sub,csv resource
		allresourcelist := [][]string{
			{"operatorsource", "opsrctestlabel", marketplaceNs},
		}

		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	// OCP-21728 check the publisher,display,labels of opsrc&csc
	g.It("[ocp-21728] create opsrc with labels [Serial]", func() {

		// Create one opsrc with label
		opsrcYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=opsrctestlabel", "NAMESPACE=marketplace_e2e", "LABEL=optestlabel", "DISPLAYNAME=optestlabel", "PUBLISHER=optestlabel", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for the opsrc is created finished
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("operatorsource", "opsrctestlabel", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create opsrctestlabel, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})

		o.Expect(err).NotTo(o.HaveOccurred())

		opsrcResourceList := [][]string{
			{"operatorsource", "opsrctestlabel", "-o=jsonpath={.metadata.labels.opsrc-provider}", marketplaceNs},
			{"operatorsource", "opsrctestlabel", "-o=jsonpath={.spec.displayName}", marketplaceNs},
			{"operatorsource", "opsrctestlabel", "-o=jsonpath={.spec.publisher}", marketplaceNs},
			{"catalogsource", "opsrctestlabel", "-o=jsonpath={.metadata.labels.opsrc-provider}", marketplaceNs},
			{"catalogsource", "opsrctestlabel", "-o=jsonpath={.spec.displayName}", marketplaceNs},
			{"catalogsource", "opsrctestlabel", "-o=jsonpath={.spec.publisher}", marketplaceNs},
		}
		// Check the displayname,provider,labels of opsrc & catalogsource
		for _, source := range opsrcResourceList {
			msg, _ := getResourceByPath(oc, source[0], source[1], source[2], source[3])
			o.Expect(msg).Should(o.ContainSubstring("optestlabel"))
		}

		// Get the packagelist of opsrctestlabel
		packageListOpsrc1, _ := getResourceByPath(oc, "operatorsource", "opsrctestlabel", "-o=jsonpath={.status.packages}", marketplaceNs)
		packageList := strings.Split(packageListOpsrc1, ",")

		// Get the packagelist with label of opsrctestlabel
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			packageListOpsrc2, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests", "-lopsrc-provider=optestlabel", "-o=name", "-n", marketplaceNs).Output()
			if err != nil {
				return false, err
			}
			for _, pkg := range packageList {
				if !strings.Contains(packageListOpsrc2, pkg) {
					return false, nil
				}
			}
			return true, nil
		})

		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
