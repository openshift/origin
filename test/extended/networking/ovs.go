package networking

import (
	"net"
	"reflect"
	"regexp"
	"strings"
	"time"

	testexutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"

	kapi "k8s.io/kubernetes/pkg/api"
	kapiunversioned "k8s.io/kubernetes/pkg/api/unversioned"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	hostSubnetTimeout = 30 * time.Second
)

var _ = Describe("[networking] OVS", func() {
	Context("generic", func() {
		f1 := e2e.NewDefaultFramework("net-ovs1")
		oc := testexutil.NewCLI("get-flows", testexutil.KubeConfigPath())

		It("should add and remove flows when pods are added and removed", func() {
			nodes := e2e.GetReadySchedulableNodesOrDie(f1.ClientSet)
			origFlows := getFlowsForAllNodes(oc, nodes.Items)
			Expect(len(origFlows)).To(Equal(len(nodes.Items)))
			for _, flows := range origFlows {
				Expect(len(flows)).ToNot(Equal(0))
			}

			podName := "ovs-test-webserver"
			deployNodeName := nodes.Items[0].Name
			ipPort := e2e.LaunchWebserverPod(f1, podName, deployNodeName)
			ip := strings.Split(ipPort, ":")[0]

			newFlows := getFlowsForAllNodes(oc, nodes.Items)
			for _, node := range nodes.Items {
				if node.Name != deployNodeName {
					Expect(reflect.DeepEqual(origFlows[node.Name], newFlows[node.Name])).To(BeTrue(), "Flows on non-deployed-to nodes should be unchanged")
				}
			}

			foundPodFlow := false
			for _, flow := range newFlows[deployNodeName] {
				if strings.Contains(flow, "="+ip+",") || strings.Contains(flow, "="+ip+" ") {
					foundPodFlow = true
					break
				}
			}
			Expect(foundPodFlow).To(BeTrue(), "Should have flows referring to pod IP address")

			err := f1.ClientSet.Core().Pods(f1.Namespace.Name).Delete(podName, &kapi.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			postDeleteFlows := getFlowsForNode(oc, deployNodeName)
			Expect(reflect.DeepEqual(origFlows[deployNodeName], postDeleteFlows)).To(BeTrue(), "Flows after deleting pod should be same as before creating it")
		})

		It("should add and remove flows when nodes are added and removed", func() {
			var err error
			nodes := e2e.GetReadySchedulableNodesOrDie(f1.ClientSet)
			origFlows := getFlowsForAllNodes(oc, nodes.Items)

			// The SDN/OVS code doesn't care that the node doesn't actually exist,
			// but we try to pick an IP on our local subnet to avoid sending
			// traffic into the real world.
			highNodeIP := ""
			for _, node := range nodes.Items {
				if node.Status.Addresses[0].Address > highNodeIP {
					highNodeIP = node.Status.Addresses[0].Address
				}
			}
			Expect(highNodeIP).NotTo(Equal(""))
			ip := net.ParseIP(highNodeIP)
			Expect(ip).NotTo(BeNil())
			ip = ip.To4()
			Expect(ip).NotTo(BeNil())
			Expect(ip[3]).NotTo(Equal(255))
			ip[3] += 1
			newNodeIP := ip.String()

			nodeName := "ovs-test-node"
			node := &kapi.Node{
				TypeMeta: kapiunversioned.TypeMeta{
					Kind: "Node",
				},
				ObjectMeta: kapi.ObjectMeta{
					Name: nodeName,
				},
				Spec: kapi.NodeSpec{
					Unschedulable: true,
				},
				Status: kapi.NodeStatus{
					Addresses: []kapi.NodeAddress{
						{
							Type:    kapi.NodeInternalIP,
							Address: newNodeIP,
						},
					},
				},
			}
			node, err = f1.ClientSet.Core().Nodes().Create(node)
			Expect(err).NotTo(HaveOccurred())
			defer f1.ClientSet.Core().Nodes().Delete(node.Name, &kapi.DeleteOptions{})

			osClient, err := testutil.GetClusterAdminClient(testexutil.KubeConfigPath())
			Expect(err).NotTo(HaveOccurred())

			e2e.Logf("Waiting up to %v for HostSubnet to be created", hostSubnetTimeout)
			for start := time.Now(); time.Since(start) < hostSubnetTimeout; time.Sleep(time.Second) {
				_, err = osClient.HostSubnets().Get(node.Name)
				if err == nil {
					break
				}
			}
			Expect(err).NotTo(HaveOccurred())

			newFlows := getFlowsForAllNodes(oc, nodes.Items)
			for nodeName := range newFlows {
				foundNodeFlow := false
				for _, flow := range newFlows[nodeName] {
					if strings.Contains(flow, "="+newNodeIP+",") || strings.Contains(flow, "="+newNodeIP+" ") {
						foundNodeFlow = true
						break
					}
				}
				Expect(foundNodeFlow).To(BeTrue(), "Should have flows referring to node IP address")
			}

			err = f1.ClientSet.Core().Nodes().Delete(node.Name, &kapi.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			e2e.Logf("Waiting up to %v for HostSubnet to be deleted", hostSubnetTimeout)
			for start := time.Now(); time.Since(start) < hostSubnetTimeout; time.Sleep(time.Second) {
				_, err = osClient.HostSubnets().Get(node.Name)
				if err != nil {
					break
				}
			}
			Expect(err).NotTo(BeNil())

			postDeleteFlows := getFlowsForAllNodes(oc, nodes.Items)
			Expect(reflect.DeepEqual(origFlows, postDeleteFlows)).To(BeTrue(), "Flows after deleting node should be same as before creating it")
		})
	})

	InMultiTenantContext(func() {
		f1 := e2e.NewDefaultFramework("net-ovs1")
		oc := testexutil.NewCLI("get-flows", testexutil.KubeConfigPath())

		It("should add and remove flows when services are added and removed", func() {
			nodes := e2e.GetReadySchedulableNodesOrDie(f1.ClientSet)
			origFlows := getFlowsForAllNodes(oc, nodes.Items)

			serviceName := "ovs-test-service"
			deployNodeName := nodes.Items[0].Name
			ipPort := launchWebserverService(f1, serviceName, deployNodeName)
			ip := strings.Split(ipPort, ":")[0]

			newFlows := getFlowsForAllNodes(oc, nodes.Items)
			for _, node := range nodes.Items {
				foundServiceFlow := false
				for _, flow := range newFlows[node.Name] {
					if strings.Contains(flow, "nw_dst="+ip+",") || strings.Contains(flow, "nw_dst="+ip+" ") {
						foundServiceFlow = true
						break
					}
				}
				Expect(foundServiceFlow).To(BeTrue(), "Each node contains a rule for the service")
			}

			err := f1.ClientSet.Core().Pods(f1.Namespace.Name).Delete(serviceName, &kapi.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			err = f1.ClientSet.Core().Services(f1.Namespace.Name).Delete(serviceName, &kapi.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			postDeleteFlows := getFlowsForAllNodes(oc, nodes.Items)
			Expect(reflect.DeepEqual(origFlows, postDeleteFlows)).To(BeTrue(), "Flows after deleting service should be same as before creating it")
		})
	})
})

func doGetFlowsForNode(oc *testexutil.CLI, nodeName string) ([]string, error) {
	pod := &kapi.Pod{
		TypeMeta: kapiunversioned.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: kapi.ObjectMeta{
			GenerateName: "flow-check",
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:  "flow-check",
					Image: "openshift/openvswitch",
					// kubernetes seems to get confused sometimes if the pod exits too quickly
					Command: []string{"sh", "-c", "ovs-ofctl -O OpenFlow13 dump-flows br0 && sleep 1"},
					VolumeMounts: []kapi.VolumeMount{
						{
							Name:      "ovs-socket",
							MountPath: "/var/run/openvswitch/br0.mgmt",
						},
					},
				},
			},
			Volumes: []kapi.Volume{
				{
					Name: "ovs-socket",
					VolumeSource: kapi.VolumeSource{
						HostPath: &kapi.HostPathVolumeSource{
							Path: "/var/run/openvswitch/br0.mgmt",
						},
					},
				},
			},
			NodeName:      nodeName,
			RestartPolicy: kapi.RestartPolicyNever,
			// We don't actually need HostNetwork, we just set it so that deploying this pod won't cause any OVS flows to be added
			SecurityContext: &kapi.PodSecurityContext{
				HostNetwork: true,
			},
		},
	}
	f := oc.KubeFramework()
	podClient := f.ClientSet.Core().Pods(f.Namespace.Name)
	pod, err := podClient.Create(pod)
	if err != nil {
		return nil, err
	}
	defer podClient.Delete(pod.Name, &kapi.DeleteOptions{})
	err = waitForPodSuccessInNamespace(f.ClientSet, pod.Name, "flow-check", f.Namespace.Name)
	if err != nil {
		return nil, err
	}
	logs, err := oc.Run("logs").Args(pod.Name).Output()
	if err != nil {
		return nil, err
	}

	// For ease of comparison, strip out the parts of the rules that change
	flows := strings.Split(logs, "\n")
	strip_re := regexp.MustCompile(`(duration|n_packets|n_bytes)=[^,]*, `)
	for i := range flows {
		flows[i] = strip_re.ReplaceAllLiteralString(flows[i], "")
	}
	return flows, nil
}

func getFlowsForNode(oc *testexutil.CLI, nodeName string) []string {
	flows, err := doGetFlowsForNode(oc, nodeName)
	expectNoError(err)
	return flows
}

func getFlowsForAllNodes(oc *testexutil.CLI, nodes []kapi.Node) map[string][]string {
	var err error
	flows := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		flows[node.Name], err = doGetFlowsForNode(oc, node.Name)
		expectNoError(err)
	}
	return flows
}
