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

	// OCP-21953 ensure the marketplace-operator running on the master node
	g.It("ensure the marketplace-operator pod running on the master node", func() {
		// Get the podname of marektplace-operator
		podName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", marketplaceNs, "-l name=marketplace-operator", "-oname").Output()
		// Get the nodes of marketplace-operator pod
		nodeName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(podName, "-n", marketplaceNs, "-o=jsonpath={.spec.nodeName}").Output()
		// Get the node labels
		nodeRole, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-o=jsonpath={.metadata.labels}").Output()

		// Node role should contain master
		o.Expect(nodeRole).Should(o.ContainSubstring("master"))
	})
})
