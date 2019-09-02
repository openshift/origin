package marketplace

import (
	"fmt"
	"time"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Marketplace] Marketplace resources with labels provider displayName", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace", exutil.KubeConfigPath())
		allNs         = "openshift-operators"
		marketplaceNs = "openshift-marketplace"

		//buildPruningBaseDir = exutil.FixturePath("testdata", "marketplace")
		opsrcYamltem = exutil.FixturePath("testdata", "marketplace", "opsrc", "02-opsrc.yaml")
		cscYamltem   = exutil.FixturePath("testdata", "marketplace", "csc", "02-csc.yaml")
	)

	g.AfterEach(func() {
		//clear the sub,csv resource
		allresourcelist := [][]string{
			{"operatorsource", "opsrctestlabel", marketplaceNs},
			{"catalogsourceconfig", "csctestlabel", marketplaceNs},
		}

		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1],source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	//OCP-21728 check the publisher,display,labels of opsrc&csc
	g.It("[ocp-21728]create opsrc with labels", func() {

		//create one opsrc with label
		//opsrcYaml := exutil.FixturePath("testdata", "marketplace", "opsrc", "02-opsrc.yaml")
		opsrcYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=opsrctestlabel", "NAMESPACE=jfan", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs), "LABEL=optestlabel", "DISPLAYNAME=optestlabel", "PUBLISHER=optestlabel").OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(30 * time.Second)

		opsrcResourceList := [][]string{
			{"operatorsource", "opsrctestlabel", "-o=jsonpath={.metadata.labels.opsrc-provider}", marketplaceNs},
			{"operatorsource", "opsrctestlabel", "-o=jsonpath={.spec.displayname}", marketplaceNs},
			{"operatorsource", "opsrctestlabel", "-o=jsonpath={.spec.publisher}", marketplaceNs},
			{"catalogsource", "opsrctestlabel", "-o=jsonpath={.metadata.labels.opsrc-provider}", marketplaceNs},
			{"catalogsource", "opsrctestlabel", "-o=jsonpath={.spec.displayname}", marketplaceNs},
			{"catalogsource", "opsrctestlabel", "-o=jsonpath={.spec.publisher", marketplaceNs},
		}
		//check the displayname,provider,labels of opsrc & catalogsource
		for _, source := range opsrcResourceList {
			msg, _ := getResourceByPath(oc, source[0], source[1], source[2], source[3])
			o.Expect(msg).Should(o.ContainSubstring("optestlabel"))
		}

		//create one csc with provider&display&labels 
		cscYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cscYamltem, "-p", "NAME=csctestlabel", fmt.Sprintf("NAMESPACE=%s", allNs), fmt.Sprintf("MARKETPLACE=%s", marketplaceNs), "PACKAGES=descheduler-test", "DISPLAYNAME=csctestlabel", "PUBLISHER=csctestlabel" ).OutputToFile("config.json")
		err = createResources(oc, cscYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(15 * time.Second)

		cscResourceList := [][]string{
			{"catalogsourceconfig", "csctestlabel", "-o=jsonpath={.spec.displayname}", marketplaceNs},
			{"catalogsourceconfig", "csctestlabel", "-o=jsonpath={.spec.publisher}", marketplaceNs},
			{"catalogsource", "csctestlabel", "-o=jsonpath={.spec.displayname}", allNs},
			{"catalogsource", "csctestlabel", "-o=jsonpath={.spec.publisher", allNs},
		}
		//check the displayname,provider oc csc & catalogsource
		for _, source := range opsrcResourceList {
			msg, _ := getResourceByPath(oc, source[0], source[1], source[2], source[3])
			o.Expect(msg).Should(o.ContainSubstring("csctestlabel"))
		}

		//get the packagelist of opsrctestlabel
		packageListOpsrc1, _ := getResourceByPath(oc, "operatorsource", "opsrctestlabel", "-o=jsonpath={.status.packages}", marketplaceNs)
		packageList := strings.Split(packageListOpsrc1, ",")

		//get the packagelist with label of opsrctestlabel 
		packageListOpsrc2, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "-lopsrc-provider=testkey", "-o=name", "-n", marketplaceNs).Output()
		
		for _, packages := range packageList {
			o.Expect(packageListOpsrc2).Should(o.ContainSubstring(packages))
		}

		//delete the csc
		err = clearResources(oc, "catalogsourceconfig", "csctestlabel", "openshift-marketplace")
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, source := range cscResourceList {
			msg, err := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeFalse())
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		//delete the opsrc
		err = clearResources(oc, "operatorsource", "opsrctestlabel", "openshift-marketplace")
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, source := range opsrcResourceList {
			msg, err := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeFalse())
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
})
