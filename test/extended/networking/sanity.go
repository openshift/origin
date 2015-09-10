package networking

import (
	"k8s.io/kubernetes/test/e2e"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("networking: sanity", func() {
	var svcname = "net-sanity"

	f := e2e.NewFramework(svcname)

	It("should function for pod communication", func() {

		By("Picking multiple nodes")
		nodes := getMultipleNodes(f)
		node1 := nodes.Items[0]
		node2 := nodes.Items[1]

		By("Creating a webserver pod")
		podName := "same-node-webserver"
		defer f.Client.Pods(f.Namespace.Name).Delete(podName, nil)
		ip := launchWebserverPod(f, podName, node1.Name)

		By("Checking that the webserver is accessible from a pod on the same node")
		expectNoError(checkConnectivityToHost(f, node1.Name, "same-node-wget", ip, 10))

		By("Checking that the webserver is accessible from a pod on a different node")
		expectNoError(checkConnectivityToHost(f, node2.Name, "different-node-wget", ip, 10))
	})
})
