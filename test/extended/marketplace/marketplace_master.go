package marketplace

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Marketplace] Marketplace pod master", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace", exutil.KubeConfigPath())
		marketplaceNs = "openshift-marketplace"
	)

	//OCP-21953 ensure the marketplace-operator running on the master node
	g.It("ensure the marketplace-operator pod running on the master node", func() {
		podName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", marketplaceNs, "-l name=marketplace-operator", "-oname").Output()
		//oc get pods -l name=marketplace-operator -o name
		nodeName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(podName, "-n", marketplaceNs, "-o=jsonpath={.spec.nodeName}").Output()
		//oc get pod/marketplace-operator-758c7d869b-hmkcj -o=jsonpath={.spec.nodeName}
		nodeRole, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-o=jsonpath={.metadata.labels}").Output()
		//oc get nodes control-plane-0 -o=jsonpath={.metadata.labels}

		//node-role.kubernetes.io/master
		o.Expect(nodeRole).Should(o.ContainSubstring("master"))
	})
})
