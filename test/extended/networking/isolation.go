package networking

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/test/e2e"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// This test requires a network plugin that supports namespace isolation.
// NOTE: if you change the test description, update networking.sh too!
var _ = Describe("[networking] network isolation plugin", func() {
	f1 := e2e.NewFramework("net-isolation1")
	f2 := e2e.NewFramework("net-isolation2")

	It("should prevent communication between pods in different namespaces on the same node", func() {
		checkPodIsolation(f1, f2, false)
	})

	It("should prevent communication between pods in different namespaces on different nodes", func() {
		checkPodIsolation(f1, f2, true)
	})
})

func checkPodIsolation(f1, f2 *e2e.Framework, differentNodes bool) {
	nodes, err := e2e.GetReadyNodes(f1)
	if err != nil {
		e2e.Failf("Failed to list nodes: %v", err)
	}
	var serverNode, clientNode *api.Node
	serverNode = &nodes.Items[0]
	if differentNodes {
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

	err = checkConnectivityToHost(f2, clientNode.Name, "isolation-wget", ip, 10)
	Expect(err).To(HaveOccurred())
}
