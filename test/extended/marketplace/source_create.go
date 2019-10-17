package marketplace

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Feature:Marketplace] Marketplace basic", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace", exutil.KubeConfigPath())
		allNs         = "openshift-operators"
		marketplaceNs = "openshift-marketplace"
		resourceWait  = 100 * time.Second

		opsrcYamltem = exutil.FixturePath("testdata", "marketplace", "opsrc", "01-opsrc.yaml")
		cscYamltem   = exutil.FixturePath("testdata", "marketplace", "csc", "01-csc.yaml")
		subYamltem   = exutil.FixturePath("testdata", "marketplace", "sub", "01-subofdes.yaml")
	)

	g.AfterEach(func() {
		//clear the sub,csv resource
		allresourcelist := [][]string{
			{"operatorsource", "opsrctest", marketplaceNs},
			{"catalogsourceconfig", "csctest", marketplaceNs},
			{"csv", "camel-k-operator.v0.2.0", allNs},
			{"subscription", "camel-k-marketplace-e2e-tests", allNs},
		}
		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	//OCP-21405 create one opsrc,OCP-21419 create one csc,OCP-21479 sub to openshift-operators,OCP-21667 delete the csc
	g.It("[ocp-21405 21419 21479 21667]create and delete the basic source,sub one operator", func() {

		//create one opsrc
		opsrcYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=opsrctest", "NAMESPACE=marketplace_e2e", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
		//wait for the opsrc is created finished
		err = wait.Poll(5*time.Second, ResourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("operatorsource", "opsrctest", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create opsrctest, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})

		o.Expect(err).NotTo(o.HaveOccurred())

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
		err = wait.Poll(5*time.Second, ResourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("operatorsource", "opsrctest", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create opsrctest, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})

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
		cscYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cscYamltem, "-p", "NAME=csctest", fmt.Sprintf("NAMESPACE=%s", allNs), fmt.Sprintf("MARKETPLACE=%s", marketplaceNs), "PACKAGES=camel-k-marketplace-e2e-tests").OutputToFile("config.json")
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
		msgofpkg, _ := existResources(oc, "packagemanifest", "camel-k-marketplace-e2e-tests", "openshift-operators")
		o.Expect(msgofpkg).Should(o.BeTrue())
		subYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", subYamltem, "-p", "NAME=camel-k-marketplace-e2e-tests", fmt.Sprintf("NAMESPACE=%s", allNs), "SOURCE=csctest", "CSV=camel-k-operator.v0.2.0").OutputToFile("config.json")
		err = createResources(oc, subYaml)
		o.Expect(err).NotTo(o.HaveOccurred())

		subResourceList := [][]string{
			{"subscription", "camel-k-marketplace-e2e-tests", allNs},
			{"csv", "camel-k-operator.v0.2.0", allNs},
		}
		for _, source := range subResourceList {
			msg, _ := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeTrue())
		}

		//clear the csv resource
		err = clearResources(oc, "csv", "camel-k-operator.v0.2.0", "openshift-operators")
		o.Expect(err).NotTo(o.HaveOccurred())
		msgofcsv, err := existResources(oc, "csv", "camel-k-operator.v0.2.0", "openshift-operators")
		o.Expect(msgofcsv).Should(o.BeFalse())
		o.Expect(err).NotTo(o.HaveOccurred())

		//clear the sub resource
		err = clearResources(oc, "subscription", "camel-k-marketplace-e2e-tests", "openshift-operators")
		o.Expect(err).NotTo(o.HaveOccurred())
		msgofsub, err := existResources(oc, "subscription", "camel-k-marketplace-e2e-tests", "openshift-operators")
		o.Expect(msgofsub).Should(o.BeFalse())
		o.Expect(err).NotTo(o.HaveOccurred())

		time.Sleep(60 * time.Second)

		//OCP-21667 delete the csc
		err = clearResources(oc, "catalogsourceconfig", "csctest", "openshift-marketplace")
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, source := range cscResourceList {
			msg, err := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeFalse())
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		time.Sleep(60 * time.Second)

		//delete the opsrc
		err = clearResources(oc, "operatorsource", "opsrctest", "openshift-marketplace")
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, source := range opsrcResourceList {
			msg, err := existResources(oc, source[0], source[1], source[2])
			o.Expect(msg).Should(o.BeFalse())
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
})
