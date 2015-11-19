package networking

import (
	"k8s.io/kubernetes/test/e2e"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// This test requires a network plugin that supports namespace isolation.
var _ = Describe("networking: isolation", func() {
	f1 := e2e.NewFramework("net-isolation1")
	f2 := e2e.NewFramework("net-isolation2")

	It("should prevent communication between pods in different namespaces", func() {
		By("Picking multiple nodes")
		nodes := getMultipleNodes(f1)
		node1 := nodes.Items[0]
		node2 := nodes.Items[1]

		By("Running a webserver in one namespace")
		podName := "isolation-webserver"
		defer f1.Client.Pods(f1.Namespace.Name).Delete(podName, nil)
		ip := launchWebserverPod(f1, podName, node1.Name)

		By("Checking that the webserver is not accessible from a pod in a different namespace on the same node")
		err := checkConnectivityToHost(f2, node1.Name, "isolation-same-node-wget", ip, 10)
		Expect(err).To(HaveOccurred())

		By("Checking that the webserver is not accessible from a pod in a different namespace on a different node")
		err = checkConnectivityToHost(f2, node2.Name, "isolation-different-node-wget", ip, 10)
		Expect(err).To(HaveOccurred())
	})
})
