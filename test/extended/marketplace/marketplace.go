package marketplace

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Feature:Marketplace] Marketplace Default Sources", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("")

	//[OCP-21630]:[Marketplace] Default OperatorSources is installed and controled by CVO
	//author: chuo@redhat.com
	g.It("[ocp-21630][ocp-24411]OperatorSource installed and controled by CVO in 4.1 and MarketplaceOperator in 4.2", func() {

		var defaultOperatorSources = [3]string{"certified-operators", "community-operators", "redhat-operators"}
		for _, v := range defaultOperatorSources {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "opsrc", v, "-o=jsonpath={.status.currentPhase.phase.message}").Output()
			if err != nil {
				e2e.Failf("Unable to get %s, error:%v", msg, err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.Equal("The object has been successfully reconciled"))
		}

		msg, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("-n", "openshift-marketplace", "opsrc", "redhat-operators", "--type", "merge", "-p", `{"spec":{"registryNamespace":"wrong"}}`).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(180 * time.Second)

		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "opsrc", "redhat-operators", "-o=jsonpath={.spec.registryNamespace}").Output()
		if err != nil {
			e2e.Failf("Unable to get operatorsource redhat-operators.spec.registryNamespace :%v", err)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.Equal("redhat-operators")
	})
	
	//[OCP-21921]:[Marketplace]Default resources of Marketplace operator
	g.It("[ocp-21921]marketplace operators", func(){
		msg, err := oc.AsAdmin().WithoutNamespace().Run("descirbe").Args("-n", "openshift-marketplace").Output()
		e2e.Logf("the namespace is %s", msg)
		
	})
})
