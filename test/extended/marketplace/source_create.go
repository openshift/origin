package marketplace

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Marketplace] Marketplace basic", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace", exutil.KubeConfigPath())
		allNs         = "openshift-operators"
		marketplaceNs = "openshift-marketplace"

		//buildPruningBaseDir = exutil.FixturePath("testdata", "marketplace")
		opsrcYamltem = exutil.FixturePath("testdata", "marketplace", "opsrc", "01-opsrc.yaml")
		cscYamltem   = exutil.FixturePath("testdata", "marketplace", "csc", "01-csc.yaml")
		subYamltem   = exutil.FixturePath("testdata", "marketplace", "sub", "01-subofdes.yaml")
	)

	g.AfterEach(func() {
		//clear the sub,csv resource
		allresourcelist := [][]string{
			{"operatorsource", "opsrctest", marketplaceNs},
			{"catalogsourceconfig", "csctest", marketplaceNs},
			{"csv", "descheduler.v0.0.3", allNs},
			{"subscription", "descheduler-test", allNs},
		}
		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	//OCP-21953 ensure the marketplace-operator running on the master node
	/*g.It("ensure the marketplace-operator pod running on the master node", func() {
		podName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", marketplaceNs, "-l name=marketplace-operator", "-oname").Output()
		//oc get pods -l name=marketplace-operator -o name
		nodeName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(podName, "-n", marketplaceNs, "-o=jsonpath={.spec.nodeName}").Output()
		//oc get pod/marketplace-operator-758c7d869b-hmkcj -o=jsonpath={.spec.nodeName}
		nodeRole, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-o=jsonpath={.metadata.labels}").Output()
		//oc get nodes control-plane-0 -o=jsonpath={.metadata.labels}

		//node-role.kubernetes.io/master
		o.Expect(nodeRole).Should(o.ContainSubstring("master"))
	})*/

	//OCP-21405 create one opsrc,OCP-21419 create one csc,OCP-21479 sub to openshift-operators,OCP-21667 delete the csc
	g.It("[ocp-21405 21419 21479 21667]create and delete the basic source,sub one operator", func() {

		//create one opsrc
		//opsrcYaml := exutil.FixturePath("testdata", "marketplace", "opsrc", "01-opsrc.yaml")
		opsrcYaml, err := oc.AsAdmin().WithoutNamespace().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=opsrctest", "NAMESPACE=jfan", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(30 * time.Second)

		opsrcResourceList := [][]string{
			{"operatorsource", "opsrctest", marketplaceNs},
			{"deployment", "opsrctest", marketplaceNs},
			{"catalogsource", "opsrctest", marketplaceNs},
			{"service", "opsrctest", marketplaceNs},
		}
		for _, source := range opsrcResourceList {
			msg, _ := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeTrue())
		}

		//OCP-21627 datastore cache is in sync with the OperatorSources present
		//get the packagelist with label opsrctest
		packageListOpsrc1, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("operatorsource", "opsrctest", "-o=jsonpath={.status.packages}").Output()
		//oc get pods -l name=marketplace-operator -o name
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", marketplaceNs, "-l name=marketplace-operator", "-oname").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		//oc delete pod/marketplace-operator-758c7d869b-hmkcj
		_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args(podName, "-n", marketplaceNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		//wait for the marketplace recover
		time.Sleep(60 * time.Second)

		//packageListLabel, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "-n", marketplaceNs, "-l", "opsrc-provider=opsrctest" "-o name").Output()
		//get the packagelist with label opsrctest
		packageListOpsrc2, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("operatorsource", "opsrctest", "-o=jsonpath={.status.packages}").Output()
		var packageEqual bool
		if packageListOpsrc1 == packageListOpsrc2 {
			packageEqual = true
		} else {
			packageEqual = false
		}
		o.Expect(packageEqual).Should(o.BeTrue())

		//OCP-21419 create one csc
		cscYaml, err := oc.AsAdmin().WithoutNamespace().Run("process").Args("--ignore-unknown-parameters=true", "-f", cscYamltem, "-p", "NAME=csctest", fmt.Sprintf("NAMESPACE=%s", allNs), fmt.Sprintf("MARKETPLACE=%s", marketplaceNs), "PACKAGES=descheduler-test").OutputToFile("config.json")
		err = createResources(oc, cscYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(15 * time.Second)

		cscResourceList := [][]string{
			{"catalogsourceconfig", "csctest", marketplaceNs},
			{"deployment", "csctest", marketplaceNs},
			{"catalogsource", "csctest", allNs},
			{"service", "csctest", marketplaceNs},
		}
		for _, source := range cscResourceList {
			msg, _ := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeTrue())
		}

		//OCP-21479 sub to openshift-operators
		time.Sleep(30 * time.Second)
		msgofpkg, _ := existResources(oc, "packagemanifest", "descheduler-test", "openshift-operators")
		o.Expect(msgofpkg).Should(o.BeTrue())
		subYaml, err := oc.AsAdmin().WithoutNamespace().Run("process").Args("--ignore-unknown-parameters=true", "-f", subYamltem, "-p", "NAME=descheduler-test", fmt.Sprintf("NAMESPACE=%s", allNs), "SOURCE=csctest", "CSV=descheduler.v0.0.3").OutputToFile("config.json")
		err = createResources(oc, subYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(80 * time.Second)

		subResourceList := [][]string{
			{"subscription", "descheduler-test", allNs},
			{"csv", "descheduler.v0.0.3", allNs},
		}
		for _, source := range subResourceList {
			msg, _ := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeTrue())
		}

		//clear the csv resource
		err = clearResources(oc, "csv", "descheduler.v0.0.3", "openshift-operators")
		o.Expect(err).NotTo(o.HaveOccurred())
		msgofcsv, err := existResources(oc, "csv", "descheduler.v0.0.3", "openshift-operators")
		o.Expect(msgofcsv).Should(o.BeFalse())
		o.Expect(err).NotTo(o.HaveOccurred())

		//clear the sub resource
		err = clearResources(oc, "subscription", "descheduler-test", "openshift-operators")
		o.Expect(err).NotTo(o.HaveOccurred())
		msgofsub, err := existResources(oc, "subscription", "descheduler-test", "openshift-operators")
		o.Expect(msgofsub).Should(o.BeFalse())
		o.Expect(err).NotTo(o.HaveOccurred())

		time.Sleep(60 * time.Second)

		//OCP-21667 delete the csc
		//msgofcsc, _ := existResources(oc, "catalogsourceconfig", "csctest", "openshift-marketplace")
		//o.Expect(msgofcsc).Should(o.BeTrue())
		err = clearResources(oc, "catalogsourceconfig", "csctest", "openshift-marketplace")
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, source := range cscResourceList {
			msg, err := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeFalse())
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		time.Sleep(60 * time.Second)

		//delete the opsrc
		//msgofopsrc, _ := existResources(oc, "operatorsource", "opsrctest", "openshift-marketplace")
		//o.Expect(msgofopsrc).Should(o.BeTrue())
		err = clearResources(oc, "operatorsource", "opsrctest", "openshift-marketplace")
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, source := range opsrcResourceList {
			msg, err := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeFalse())
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
})
