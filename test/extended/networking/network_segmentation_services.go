package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = Describe("[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] services", func() {
	// TODO: so far, only the isolation tests actually require this PSA ... Feels wrong to run everything priviliged.
	// I've tried to have multiple kubeframeworks (from multiple OCs) running (with different project names) but
	// it didn't work.
	oc := exutil.NewCLIWithPodSecurityLevel("network-segmentation-e2e-services", admissionapi.LevelPrivileged)
	f := oc.KubeFramework()
	ocDefault := exutil.NewCLIWithPodSecurityLevel("network-segmentation-e2e-services-default", admissionapi.LevelPrivileged)
	fDefault := ocDefault.KubeFramework()

	Context("on a user defined primary network", func() {
		const (
			nadName                      = "tenant-red"
			servicePort                  = 80
			serviceTargetPort            = 80
			userDefinedNetworkIPv4Subnet = "10.127.0.0/16"
			userDefinedNetworkIPv6Subnet = "2014:100:200::0/60"
		)

		var (
			cs                  clientset.Interface
			nadClient           nadclient.K8sCniCncfIoV1Interface
			defaultNetNamespace string
		)

		BeforeEach(func() {
			cs = f.ClientSet

			var err error
			nadClient, err = nadclient.NewForConfig(f.ClientConfig())
			Expect(err).NotTo(HaveOccurred())
		})

		cleanupFn := func() {
			By("Removing the namespace so all resources get deleted")
			err := cs.CoreV1().Namespaces().Delete(context.TODO(), f.Namespace.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err, "Failed to remove the namespace %s %v", f.Namespace.Name, err)
			if defaultNetNamespace != "" {
				err = cs.CoreV1().Namespaces().Delete(context.TODO(), defaultNetNamespace, metav1.DeleteOptions{})
				framework.ExpectNoError(err, "Failed to remove the namespace %v", defaultNetNamespace, err)
			}

		}

		AfterEach(func() {
			cleanupFn()
		})

		DescribeTable(
			// The test creates a client and nodeport service in a UDN backed by one pod and similarly
			// a nodeport service and a client in the default network. We expect ClusterIPs to be
			// reachable only within the same network. We expect NodePort services to be exposed
			// to all networks.
			// We verify the following scenarios:
			// - UDN client --> UDN service, with backend pod and client running on the same node:
			//   + clusterIP succeeds
			//   + nodeIP:nodePort works, when we only target the local node (*)
			//
			// - UDN client --> UDN service, with backend pod and client running on different nodes:
			//   + clusterIP succeeds
			//   + nodeIP:nodePort succeeds, when we only target the local node (*)
			//
			// - default-network client --> UDN service:
			//   + clusterIP fails
			//   + nodeIP:nodePort fails FOR NOW, when we only target the local node (*)
			//
			// -  UDN service --> default-network:
			//   + clusterIP fails
			//   + nodeIP:nodePort fails FOR NOW, when we only target the local node (*)
			//
			// (*) TODO connect to node ports on other nodes too once ovnkube-node fully supports UDN,
			//     that is when https://github.com/ovn-org/ovn-kubernetes/pull/4648 and
			//     https://github.com/ovn-org/ovn-kubernetes/pull/4554 merge

			"should be reachable through their cluster IP and node port",
			func(
				netConfigParams networkAttachmentConfigParams,
			) {
				namespace := f.Namespace.Name
				jig := e2eservice.NewTestJig(cs, namespace, "udn-service")

				By("Selecting 3 schedulable nodes")
				nodes, err := e2enode.GetBoundedReadySchedulableNodes(context.TODO(), f.ClientSet, 3)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodes.Items)).To(BeNumerically(">", 2))

				By("Selecting nodes for pods and service")
				serverPodNodeName := nodes.Items[0].Name
				clientNode := nodes.Items[1].Name // when client runs on a different node than the server

				By("Creating the attachment configuration")
				netConfig := newNetworkAttachmentConfig(netConfigParams)
				netConfig.namespace = f.Namespace.Name
				_, err = nadClient.NetworkAttachmentDefinitions(f.Namespace.Name).Create(
					context.Background(),
					generateNAD(netConfig),
					metav1.CreateOptions{},
				)
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("Creating a UDN NodePort service"))
				policy := corev1.IPFamilyPolicyPreferDualStack
				udnService, err := jig.CreateUDPService(context.TODO(), func(s *corev1.Service) {
					s.Spec.Ports = []corev1.ServicePort{
						{
							Name:       "udp",
							Protocol:   corev1.ProtocolUDP,
							Port:       80,
							TargetPort: intstr.FromInt(int(serviceTargetPort)),
						},
					}
					s.Spec.Type = corev1.ServiceTypeNodePort
					s.Spec.IPFamilyPolicy = &policy
				})
				framework.ExpectNoError(err)

				By("Creating a UDN backend pod")
				udnServerPod := e2epod.NewAgnhostPod(
					namespace, "backend-pod", nil, nil,
					[]corev1.ContainerPort{
						{ContainerPort: (serviceTargetPort), Protocol: "UDP"}},
					"netexec",
					"--udp-port="+fmt.Sprint(serviceTargetPort))

				udnServerPod.Labels = jig.Labels
				udnServerPod.Spec.NodeName = serverPodNodeName
				udnServerPod = e2epod.NewPodClient(f).CreateSync(context.TODO(), udnServerPod)

				By(fmt.Sprintf("Creating a UDN client pod on the same node (%s)", udnServerPod.Spec.NodeName))
				udnClientPod := e2epod.NewAgnhostPod(namespace, "udn-client", nil, nil, nil)
				udnClientPod.Spec.NodeName = udnServerPod.Spec.NodeName
				udnClientPod = e2epod.NewPodClient(f).CreateSync(context.TODO(), udnClientPod)

				// UDN -> UDN
				By("Connect to the UDN service cluster IP from the UDN client pod on the same node")
				checkConnectionToClusterIPs(f, udnClientPod, udnService, udnServerPod.Name)
				checkConnectionToNodePort(f, udnClientPod, udnService, &nodes.Items[0], "endpoint node", udnServerPod.Name)
				// TODO uncomment below as soon as ovnkube-node supports UDN
				// checkConnectionToNodePort(f, clientPod2, udnService, &nodes.Items[1], "client node", udnServerPod.Name)
				// checkConnectionToNodePort(f, clientPod2, udnService, &nodes.Items[2], "other node", udnServerPod.Name)

				By(fmt.Sprintf("Creating a UDN client pod on a different node (%s)", clientNode))
				udnClientPod2 := e2epod.NewAgnhostPod(namespace, "udn-client2", nil, nil, nil)
				udnClientPod2.Spec.NodeName = clientNode
				udnClientPod2 = e2epod.NewPodClient(f).CreateSync(context.TODO(), udnClientPod2)

				By("Connect to the UDN service from the UDN client pod on a different node")
				checkConnectionToClusterIPs(f, udnClientPod2, udnService, udnServerPod.Name)
				// TODO uncomment below as soon as ovnkube-node supports UDN
				checkConnectionToNodePort(f, udnClientPod2, udnService, &nodes.Items[1], "local node", udnServerPod.Name)
				// checkConnectionToNodePort(f, clientPod2, udnService, &nodes.Items[0], "server node", udnServerPod.Name)
				// checkConnectionToNodePort(f, clientPod2, udnService, &nodes.Items[2], "other node", udnServerPod.Name)

				// Default network -> UDN
				// Check that it cannot connect
				By(fmt.Sprintf("Create a client pod in the default network on node %s", clientNode))
				defaultNetNamespace = fDefault.Namespace.GetName()
				defaultClient, err := createPodOnNode(fDefault.ClientSet, "default-net-pod", clientNode, defaultNetNamespace, nil, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Verify that the client in the default network cannot connect to the UDN service")
				checkNoConnectionToClusterIPs(fDefault, defaultClient, udnService)
				checkNoConnectionToNodePort(fDefault, defaultClient, udnService, &nodes.Items[1], "local node") // TODO change to checkConnectionToNodePort when we have full UDN support in ovnkube-node
				// TODO uncomment below as soon as ovnkube-node supports UDN
				// checkConnectionToNodePort(f, defaultClient, udnService, &nodes.Items[0], "server node")
				// checkConnectionToNodePort(f, defaultClient, udnService, &nodes.Items[2], "other node")

				// UDN -> Default network
				// Create a backend pod and service in the default network and verify that the client pod in the UDN
				// cannot reach it
				By(fmt.Sprintf("Creating a backend pod in the default network on node %s", serverPodNodeName))
				defaultLabels := map[string]string{"app": "default-app"}

				_, err = createPodOnNode(fDefault.ClientSet, "backend-pod-default", serverPodNodeName,
					defaultNetNamespace, []string{"/agnhost"}, defaultLabels,
					func(pod *corev1.Pod) {
						pod.Spec.Containers[0].Ports = []corev1.ContainerPort{{ContainerPort: (serviceTargetPort), Protocol: "UDP"}}
						pod.Spec.Containers[0].Args = []string{"netexec", "--udp-port=" + fmt.Sprint(serviceTargetPort)}
					})
				Expect(err).NotTo(HaveOccurred())

				By("create a node port service in the default network")
				defaultService := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: "service-default"},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:     "udp-port",
								Port:     int32(servicePort),
								Protocol: corev1.ProtocolUDP,
							},
						},
						Selector:       defaultLabels,
						Type:           corev1.ServiceTypeNodePort,
						IPFamilyPolicy: &policy,
					},
				}

				defaultService, err = fDefault.ClientSet.CoreV1().Services(defaultNetNamespace).Create(context.TODO(), defaultService, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("verify that the client pod in the UDN cannot connect to the default-network service")
				checkNoConnectionToClusterIPs(f, udnClientPod2, defaultService)
				// TODO uncomment below when below OVN_DISABLE_SNAT_MULTIPLE_GWS=true is supported
				// checkConnectionToNodePort(f, udnClientPod2, defaultService, &nodes.Items[0], "server node", defaultServerPod.Name)
				// TODO change line below to checkConnectionToNodePort when we have full UDN support in ovnkube-node
				checkNoConnectionToNodePort(f, udnClientPod2, defaultService, &nodes.Items[1], "local node")
				// TODO uncomment below when OVN_DISABLE_SNAT_MULTIPLE_GWS=true is supported
				// checkConnectionToNodePort(f, udnClientPod2, defaultService, &nodes.Items[2], "other node", defaultServerPod.Name)
			},

			Entry(
				"L3 dualstack primary UDN, cluster-networked pods, NodePort service",
				networkAttachmentConfigParams{
					name:     nadName,
					topology: "layer3",
					cidr:     fmt.Sprintf("%s,%s", userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet),
					role:     "primary",
				},
			),

			// TODO add L2 NADs once L2 UDN support is available for services
		)

	})

})

// TODO Once https://github.com/ovn-org/ovn-kubernetes/pull/4567 merges, use the vendored *TestJig.Run(), which tests
// the reachability of a service through its name and through its cluster IP. For now only test the cluster IP.

const OVNNodeHostCIDRs = "k8s.ovn.org/host-cidrs"

// ParseNodeHostCIDRsDropNetMask returns the parsed host IP addresses found on a node's host CIDR annotation. Removes the mask.
func ParseNodeHostCIDRsDropNetMask(node *kapi.Node) (sets.Set[string], error) {
	addrAnnotation, ok := node.Annotations[OVNNodeHostCIDRs]
	if !ok {
		return nil, newAnnotationNotSetError("%s annotation not found for node %q", OVNNodeHostCIDRs, node.Name)
	}

	var cfg []string
	if err := json.Unmarshal([]byte(addrAnnotation), &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal host cidrs annotation %s for node %q: %v",
			addrAnnotation, node.Name, err)
	}

	for i, cidr := range cfg {
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil || ip == nil {
			return nil, fmt.Errorf("failed to parse node host cidr: %v", err)
		}
		cfg[i] = ip.String()
	}
	return sets.New(cfg...), nil
}

func checkConnectionToAgnhostPod(f *framework.Framework, clientPod *corev1.Pod, expectedOutput, cmd string) error {
	return wait.PollImmediate(200*time.Millisecond, 5*time.Second, func() (bool, error) {
		stdout, stderr, err2 := e2epod.ExecShellInPodWithFullOutput(context.Background(), f, clientPod.Name, cmd)
		fmt.Printf("stdout=%s\n", stdout)
		fmt.Printf("stderr=%s\n", stderr)
		fmt.Printf("err=%v\n", err2)

		if stderr != "" {
			return false, fmt.Errorf("stderr=%s", stderr)
		}

		if err2 != nil {
			return false, err2
		}
		return stdout == expectedOutput, nil
	})
}

func checkNoConnectionToAgnhostPod(f *framework.Framework, clientPod *corev1.Pod, cmd string) error {
	err := wait.PollImmediate(500*time.Millisecond, 2*time.Second, func() (bool, error) {
		stdout, stderr, err2 := e2epod.ExecShellInPodWithFullOutput(context.Background(), f, clientPod.Name, cmd)
		fmt.Printf("stdout=%s\n", stdout)
		fmt.Printf("stderr=%s\n", stderr)
		fmt.Printf("err=%v\n", err2)

		if stderr != "" {
			return false, nil
		}

		if err2 != nil {
			return false, err2
		}
		if stdout != "" {
			return true, fmt.Errorf("Connection unexpectedly succeeded. Stdout: %s\n", stdout)
		}
		return false, nil
	})

	if err != nil {
		if wait.Interrupted(err) {
			// The timeout occurred without the connection succeeding, which is what we expect
			return nil
		}
		return err
	}
	return fmt.Errorf("Error: %s/%s was able to connect (cmd=%s) ", clientPod.Namespace, clientPod.Name, cmd)
}

func checkConnectionToClusterIPs(f *framework.Framework, clientPod *corev1.Pod, service *corev1.Service, expectedOutput string) {
	checkConnectionOrNoConnectionToClusterIPs(f, clientPod, service, expectedOutput, true)
}

func checkNoConnectionToClusterIPs(f *framework.Framework, clientPod *corev1.Pod, service *corev1.Service) {
	checkConnectionOrNoConnectionToClusterIPs(f, clientPod, service, "", false)
}

func checkConnectionOrNoConnectionToClusterIPs(f *framework.Framework, clientPod *corev1.Pod, service *corev1.Service, expectedOutput string, shouldConnect bool) {
	var err error
	targetPort := service.Spec.Ports[0].TargetPort.String()
	notStr := ""
	if !shouldConnect {
		notStr = "not "
	}

	for _, clusterIP := range service.Spec.ClusterIPs {
		msg := fmt.Sprintf("Client %s/%s should %sreach service %s/%s on cluster IP %s port %s",
			clientPod.Namespace, clientPod.Name, notStr, service.Namespace, service.Name, clusterIP, targetPort)
		By(msg)

		cmd := fmt.Sprintf(`/bin/sh -c 'echo hostname | nc -u -w 1 %s %s '`, clusterIP, targetPort)

		if shouldConnect {
			err = checkConnectionToAgnhostPod(f, clientPod, expectedOutput, cmd)
		} else {
			err = checkNoConnectionToAgnhostPod(f, clientPod, cmd)
		}
		framework.ExpectNoError(err, fmt.Sprintf("Failed to verify that %s", msg))
	}
}

func checkConnectionToNodePort(f *framework.Framework, clientPod *corev1.Pod, service *corev1.Service, node *corev1.Node, nodeRoleMsg, expectedOutput string) {
	checkConnectionOrNoConnectionToNodePort(f, clientPod, service, node, nodeRoleMsg, expectedOutput, true)
}

func checkNoConnectionToNodePort(f *framework.Framework, clientPod *corev1.Pod, service *corev1.Service, node *corev1.Node, nodeRoleMsg string) {
	checkConnectionOrNoConnectionToNodePort(f, clientPod, service, node, nodeRoleMsg, "", false)
}

func checkConnectionOrNoConnectionToNodePort(f *framework.Framework, clientPod *corev1.Pod, service *corev1.Service, node *corev1.Node, nodeRoleMsg, expectedOutput string, shouldConnect bool) {
	var err error
	nodePort := service.Spec.Ports[0].NodePort
	notStr := ""
	if !shouldConnect {
		notStr = "not "
	}
	nodeIPs, err := ParseNodeHostCIDRsDropNetMask(node)
	Expect(err).NotTo(HaveOccurred())

	for nodeIP := range nodeIPs {
		msg := fmt.Sprintf("Client %s/%s should %sconnect to NodePort service %s/%s on %s:%d (node %s, %s)",
			clientPod.Namespace, clientPod.Name, notStr, service.Namespace, service.Name, nodeIP, nodePort, node.Name, nodeRoleMsg)
		By(msg)
		cmd := fmt.Sprintf(`/bin/sh -c 'echo hostname | nc -u -w 1 %s %d '`, nodeIP, nodePort)

		if shouldConnect {
			err = checkConnectionToAgnhostPod(f, clientPod, expectedOutput, cmd)
		} else {
			err = checkNoConnectionToAgnhostPod(f, clientPod, cmd)
		}
		framework.ExpectNoError(err, fmt.Sprintf("Failed to verify that %s", msg))
	}
}
