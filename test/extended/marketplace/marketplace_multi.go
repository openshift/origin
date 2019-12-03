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
		resourceWait  = 60 * time.Second

		opsrcYamltem = exutil.FixturePath("testdata", "marketplace", "opsrc", "02-opsrc.yaml")
	)

	g.AfterEach(func() {
		//clear the sub,csv resource
		allresourcelist := [][]string{
			{"operatorsource", "flatted", marketplaceNs},
			{"operatorsource", "nested", marketplaceNs},
			{"operatorsource", "multiformat", marketplaceNs},
		}

		for _, source := range allresourcelist {
			err := clearResources(oc, source[0], source[1], source[2])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	//OCP-23090 check the multi format operators support
	g.It("[ocp-23090 25675]create opsrc with multi format operators and operator with error", func() {

		//create the opsrc that only has the flatted operator
		opsrcYaml, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=flatted", "NAMESPACE=marketplace_e2e", "LABEL=flatted", "DISPLAYNAME=flatted", "PUBLISHER=flatted", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml)
		o.Expect(err).NotTo(o.HaveOccurred())

		//wait for the opsrc is created finished
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("operatorsource", "flatted", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create flatted, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		podNameFlat, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", marketplaceNs, "-l marketplace.operatorSource=flatted", "-oname").Output()
		//wait for pod is running
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args(podNameFlat, "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to get the pods podNameFlat status, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "1/1") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		logOfFlat, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podNameFlat, "-n", marketplaceNs).Output()
		o.Expect(logOfFlat).Should(o.ContainSubstring("decoded 2 flattened and 0 nested"))

		////create the opsrc that only has the nested operator
		opsrcYaml1, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=nested", "NAMESPACE=kaka1", "LABEL=nested", "DISPLAYNAME=nested", "PUBLISHER=nested", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml1)
		o.Expect(err).NotTo(o.HaveOccurred())

		//wait for the opsrc is created finished
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("operatorsource", "nested", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create nested, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		podNameNest, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", marketplaceNs, "-l marketplace.operatorSource=nested", "-oname").Output()
		//wait for pod is running
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args(podNameNest, "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to get the pods podNameNest status, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "1/1") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		logOfNest, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podNameNest, "-n", marketplaceNs).Output()
		o.Expect(logOfNest).Should(o.ContainSubstring("decoded 0 flattened and 1 nested"))

		// OCP-25675 create the opsrc that has the flatted & nested operator and one operator has error
		opsrcYaml2, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", opsrcYamltem, "-p", "NAME=multiformat", "NAMESPACE=kaka", "LABEL=multiformat", "DISPLAYNAME=multiformat", "PUBLISHER=multiformat", fmt.Sprintf("MARKETPLACE=%s", marketplaceNs)).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = createResources(oc, opsrcYaml2)
		o.Expect(err).NotTo(o.HaveOccurred())

		//wait for the opsrc is created finished
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("operatorsource", "multiformat", "-o=jsonpath={.status.currentPhase.phase.message}", "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to create multiformat, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "has been successfully reconciled") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		podNameMult, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", marketplaceNs, "-l marketplace.operatorSource=multiformat", "-oname").Output()
		//wait for pod is running
		err = wait.Poll(5*time.Second, resourceWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args(podNameMult, "-n", marketplaceNs).Output()
			if err != nil {
				e2e.Failf("Failed to get the pods podNameMult status, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "1/1") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		logOfMult, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podNameMult, "-n", marketplaceNs).Output()
		o.Expect(logOfMult).Should(o.ContainSubstring("decoded 1 flattened and 1 nested"))
		o.Expect(logOfMult).Should(o.ContainSubstring("error loading manifests from appregistry"))
	})
})
