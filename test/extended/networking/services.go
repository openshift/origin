package networking

import (
	"k8s.io/kubernetes/pkg/api"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[networking] services", func() {
	Context("basic functionality", func() {
		f1 := e2e.NewDefaultFramework("net-services1")

		It("should allow connections to another pod on the same node via a service IP", func() {
			Expect(checkServiceConnectivity(f1, f1, 1)).To(Succeed())
		})

		It("should allow connections to another pod on a different node via a service IP", func() {
			Expect(checkServiceConnectivity(f1, f1, 2)).To(Succeed())
		})
	})

	InSingleTenantContext(func() {
		f1 := e2e.NewDefaultFramework("net-services1")
		f2 := e2e.NewDefaultFramework("net-services2")

		It("should allow connections to pods in different namespaces on the same node via service IPs", func() {
			Expect(checkServiceConnectivity(f1, f2, 1)).To(Succeed())
		})

		It("should allow connections to pods in different namespaces on different nodes via service IPs", func() {
			Expect(checkServiceConnectivity(f1, f2, 2)).To(Succeed())
		})
	})

	InMultiTenantContext(func() {
		f1 := e2e.NewDefaultFramework("net-services1")
		f2 := e2e.NewDefaultFramework("net-services2")

		It("should prevent connections to pods in different namespaces on the same node via service IPs", func() {
			err := checkServiceConnectivity(f1, f2, 1)
			Expect(err).To(HaveOccurred())
		})

		It("should prevent connections to pods in different namespaces on different nodes via service IPs", func() {
			err := checkServiceConnectivity(f1, f2, 2)
			Expect(err).To(HaveOccurred())
		})

		It("should allow connections to services in the default namespace from a pod in another namespace on the same node", func() {
			makeNamespaceGlobal(f1.Namespace)
			Expect(checkServiceConnectivity(f1, f2, 1)).To(Succeed())
		})
		It("should allow connections to services in the default namespace from a pod in another namespace on a different node", func() {
			makeNamespaceGlobal(f1.Namespace)
			Expect(checkServiceConnectivity(f1, f2, 2)).To(Succeed())
		})
		It("should allow connections from pods in the default namespace to a service in another namespace on the same node", func() {
			makeNamespaceGlobal(f2.Namespace)
			Expect(checkServiceConnectivity(f1, f2, 1)).To(Succeed())
		})
		It("should allow connections from pods in the default namespace to a service in another namespace on a different node", func() {
			makeNamespaceGlobal(f2.Namespace)
			Expect(checkServiceConnectivity(f1, f2, 2)).To(Succeed())
		})
	})
})

func checkServiceConnectivity(serverFramework, clientFramework *e2e.Framework, numNodes int) error {
	nodes := e2e.GetReadySchedulableNodesOrDie(serverFramework.Client)
	var serverNode, clientNode *api.Node
	serverNode = &nodes.Items[0]
	if numNodes == 2 {
		if len(nodes.Items) == 1 {
			e2e.Skipf("Only one node is available in this environment")
		}
		clientNode = &nodes.Items[1]
	} else {
		clientNode = serverNode
	}

	podName := "service-webserver"
	defer serverFramework.Client.Pods(serverFramework.Namespace.Name).Delete(podName, nil)
	ip := launchWebserverService(serverFramework, podName, serverNode.Name)

	return checkConnectivityToHost(clientFramework, clientNode.Name, "service-wget", ip, 10)
}
