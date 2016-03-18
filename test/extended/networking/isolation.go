package networking

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/test/e2e"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[networking] network isolation", func() {
	f1 := e2e.NewDefaultFramework("net-isolation1")
	f2 := e2e.NewDefaultFramework("net-isolation2")

	Specify("multi-tenant plugins should prevent communication between pods in different namespaces on the same node", func() {
		skipIfSingleTenant()
		err := checkPodIsolation(f1, f2, 1)
		Expect(err).To(HaveOccurred())
	})

	Specify("multi-tenant plugins should prevent communication between pods in different namespaces on different nodes", func() {
		skipIfSingleTenant()
		err := checkPodIsolation(f1, f2, 2)
		Expect(err).To(HaveOccurred())
	})

	Specify("single-tenant plugins should allow communication between pods in different namespaces on the same node", func() {
		skipIfMultiTenant()
		Expect(checkPodIsolation(f1, f2, 1)).To(Succeed())
	})

	Specify("single-tenant plugins should allow communication between pods in different namespaces on different nodes", func() {
		skipIfMultiTenant()
		Expect(checkPodIsolation(f1, f2, 2)).To(Succeed())
	})
})

func checkPodIsolation(f1, f2 *e2e.Framework, numNodes int) error {
	nodes, err := e2e.GetReadyNodes(f1)
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

	podName := "isolation-webserver"
	defer f1.Client.Pods(f1.Namespace.Name).Delete(podName, nil)
	ip := e2e.LaunchWebserverPod(f1, podName, serverNode.Name)

	return checkConnectivityToHost(f2, clientNode.Name, "isolation-wget", ip, 10)
}
