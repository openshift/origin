package marketplace

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Marketplace] Marketplace Default Sources", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("")

	//[OCP-21921]:[Marketplace]Default resources of Marketplace operator
	g.It("[ocp-21921]marketplace operators", func() {
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ns", "openshift-marketplace", "-o=jsonpath={.status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.Equal("Active"))

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("crd", "catalogsourceconfigs.operators.coreos.com", "-o=jsonpath={.status.acceptedNames.shortNames}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("csc"))

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("crd", "operatorsources.operators.coreos.com", "-o=jsonpath={.status.acceptedNames.shortNames}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("opsrc"))

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "sa").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("marketplace-operator"))

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterrole").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("marketplace-operator"))

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterrolebinding", "marketplace-operator", fmt.Sprintf("-o=jsonpath='{.subjects[?(@.kind==\"%s\")]}'", "ServiceAccount")).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("name:marketplace-operator"))

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "deployment").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("marketplace-operator"))

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "catalogsource").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("certified-operators"))
		o.Expect(msg).To(o.ContainSubstring("community-operators"))
		o.Expect(msg).To(o.ContainSubstring("redhat-operators "))
	})

	//[OCP-21630]:[Marketplace] Default OperatorSources is installed and controled by CVO
	//[OCP-24411]:[Marketplace] MarketplaceOperator manage the default OperatorSources
	//author: chuo@redhat.com
	g.It("[ocp-21630] OperatorSource installed and controled by CVO in 4.1 and [ocp-24411] by MarketplaceOperator in 4.2", func() {

		var defaultOperatorSources = [3]string{"certified-operators", "community-operators", "redhat-operators"}
		for _, opsrc := range defaultOperatorSources {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "opsrc", opsrc, "-o=jsonpath={.status.currentPhase.phase.message}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.Equal("The object has been successfully reconciled"))
		}

		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("-n", "openshift-marketplace", "opsrc", "redhat-operators", "--type", "merge", "-p", `{"spec":{"registryNamespace":"wrong"}}`).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(180 * time.Second)

		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "opsrc", "redhat-operators", "-o=jsonpath={.spec.registryNamespace}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.Equal("redhat-operators"))
	})
})
