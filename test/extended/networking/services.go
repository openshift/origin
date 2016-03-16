package networking

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/test/e2e"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[networking] services", func() {
	f1 := e2e.NewDefaultFramework("net-services2")
	f2 := e2e.NewDefaultFramework("net-services2")

	It("should allow connections to another pod on the same node via a service IP", func() {
		Expect(checkServiceConnectivity(f1, f1, 1)).To(Succeed())
	})

	It("should allow connections to another pod on a different node via a service IP", func() {
		Expect(checkServiceConnectivity(f1, f1, 2)).To(Succeed())
	})

	Specify("single-tenant plugins should allow connections to pods in different namespaces on the same node via service IPs", func() {
		skipIfMultiTenant()
		Expect(checkServiceConnectivity(f1, f2, 1)).To(Succeed())
	})

	Specify("single-tenant plugins should allow connections to pods in different namespaces on different nodes via service IPs", func() {
		skipIfMultiTenant()
		Expect(checkServiceConnectivity(f1, f2, 2)).To(Succeed())
	})

	Specify("multi-tenant plugins should prevent connections to pods in different namespaces on the same node via service IPs", func() {
		skipIfSingleTenant()
		err := checkServiceConnectivity(f1, f2, 1)
		Expect(err).To(HaveOccurred())
	})

	Specify("multi-tenant plugins should prevent connections to pods in different namespaces on different nodes via service IPs", func() {
		skipIfSingleTenant()
		err := checkServiceConnectivity(f1, f2, 2)
		Expect(err).To(HaveOccurred())
	})
})

func checkServiceConnectivity(serverFramework, clientFramework *e2e.Framework, numNodes int) error {
	nodes, err := e2e.GetReadyNodes(serverFramework)
	if err != nil {
		e2e.Failf("Failed to list nodes: %v", err)
	}
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
