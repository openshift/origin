package networking

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/test/e2e"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[networking] network isolation", func() {
	InSingleTenantContext(func() {
		f1 := e2e.NewDefaultFramework("net-isolation1")
		f2 := e2e.NewDefaultFramework("net-isolation2")

		It("should allow communication between pods in different namespaces on the same node", func() {
			Expect(checkPodIsolation(f1, f2, 1)).To(Succeed())
		})

		It("should allow communication between pods in different namespaces on different nodes", func() {
			Expect(checkPodIsolation(f1, f2, 2)).To(Succeed())
		})
	})

	InMultiTenantContext(func() {
		f1 := e2e.NewDefaultFramework("net-isolation1")
		f2 := e2e.NewDefaultFramework("net-isolation2")

		It("should prevent communication between pods in different namespaces on the same node", func() {
			err := checkPodIsolation(f1, f2, 1)
			Expect(err).To(HaveOccurred())
		})

		It("should prevent communication between pods in different namespaces on different nodes", func() {
			err := checkPodIsolation(f1, f2, 2)
			Expect(err).To(HaveOccurred())
		})

		// The test framework doesn't allow us to easily make use of the actual "default"
		// namespace, so we test default namespace behavior by changing either f1's or
		// f2's NetNamespace to have VNID 0 instead. But this only works under the
		// multi-tenant plugin since the single-tenant one doesn't create NetNamespaces at
		// all (and so there's not really any point in even running these tests anyway).
		It("should allow communication from default to non-default namespace on the same node", func() {
			makeNamespaceGlobal(f2.Namespace)
			Expect(checkPodIsolation(f1, f2, 1)).To(Succeed())
		})
		It("should allow communication from default to non-default namespace on a different node", func() {
			makeNamespaceGlobal(f2.Namespace)
			Expect(checkPodIsolation(f1, f2, 2)).To(Succeed())
		})
		It("should allow communication from non-default to default namespace on the same node", func() {
			makeNamespaceGlobal(f1.Namespace)
			Expect(checkPodIsolation(f1, f2, 1)).To(Succeed())
		})
		It("should allow communication from non-default to default namespace on a different node", func() {
			makeNamespaceGlobal(f1.Namespace)
			Expect(checkPodIsolation(f1, f2, 2)).To(Succeed())
		})
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
