package networking

import (
	"k8s.io/kubernetes/test/e2e"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("[networking] basic openshift networking", func() {
	svcname := "net-sanity"
	timeout := 10

	f := e2e.NewDefaultFramework(svcname)

	It("should function for pod communication on a single node", func() {
		By("Picking a node")
		nodes, err := e2e.GetReadyNodes(f)
		if err != nil {
			e2e.Failf("Failed to list nodes: %v", err)
		}
		node := nodes.Items[0]

		By("Creating a webserver pod")
		podName := "same-node-webserver"
		defer f.Client.Pods(f.Namespace.Name).Delete(podName, nil)
		ip := e2e.LaunchWebserverPod(f, podName, node.Name)

		By("Checking that the webserver is accessible from a pod on the same node")
		expectNoError(checkConnectivityToHost(f, node.Name, "same-node-wget", ip, timeout))
	})

	It("should function for pod communication between nodes", func() {
		podClient := f.Client.Pods(f.Namespace.Name)

		By("Picking multiple nodes")
		nodes, err := e2e.GetReadyNodes(f)
		if err != nil {
			e2e.Failf("Failed to list nodes: %v", err)
		}
		if len(nodes.Items) == 1 {
			e2e.Skipf("Only one node is available in this environment")
		}
		node1 := nodes.Items[0]
		node2 := nodes.Items[1]

		By("Creating a webserver pod")
		podName := "different-node-webserver"
		defer podClient.Delete(podName, nil)
		ip := e2e.LaunchWebserverPod(f, podName, node1.Name)

		By("Checking that the webserver is accessible from a pod on a different node")
		expectNoError(checkConnectivityToHost(f, node2.Name, "different-node-wget", ip, timeout))
	})
})
