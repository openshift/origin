package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	kapi "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = Describe("[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] services", func() {
	// disable automatic namespace creation, we need to add the required UDN label
	oc := exutil.NewCLIWithoutNamespace("network-segmentation-e2e-services")
	f := oc.KubeFramework()
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	ocDefault := exutil.NewCLIWithPodSecurityLevel("network-segmentation-e2e-services-default", admissionapi.LevelPrivileged)
	fDefault := ocDefault.KubeFramework()

	Context("on a user defined primary network", func() {
		const (
			nadName                      = "tenant-red"
			servicePort                  = 88
			serviceTargetPort            = 80
			userDefinedNetworkIPv4Subnet = "10.111.0.0/16" // 10.128.0.0/16 is the cluster subnet
			userDefinedNetworkIPv6Subnet = "2014:100:200::0/60"
			clientContainer              = "frr"
		)

		var (
			cs        clientset.Interface
			nadClient nadclient.K8sCniCncfIoV1Interface
		)

		BeforeEach(func() {
			cs = f.ClientSet

			var err error
			nadClient, err = nadclient.NewForConfig(f.ClientConfig())
			Expect(err).NotTo(HaveOccurred())

			namespace, err := f.CreateNamespace(context.TODO(), f.BaseName, map[string]string{
				"e2e-framework":           f.BaseName,
				RequiredUDNNamespaceLabel: "",
			})
			f.Namespace = namespace
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable(
			// The test creates a client and nodeport service in a UDN backed by one pod and similarly
			// a nodeport service and a client in the default network. We expect ClusterIPs to be
			// reachable only within the same network. We expect NodePort services to be exposed
			// to all networks.
			// We verify the following scenarios:
			// - UDN client --> UDN service, with backend pod and client running on the same node:
			//   + clusterIP succeeds
			//   + nodeIP:nodePort works, when we only target the local node
			//
			// - UDN client --> UDN service, with backend pod and client running on different nodes:
			//   + clusterIP succeeds
			//   + nodeIP:nodePort succeeds, when we only target the local node
			//
			// - default-network client --> UDN service:
			//   + clusterIP fails
			//   + nodeIP:nodePort fails FOR NOW, when we only target the local node
			//
			// -  UDN service --> default-network:
			//   + clusterIP fails
			//   + nodeIP:nodePort fails FOR NOW, when we only target the local node

			"should be reachable through their cluster IP, node port and load balancer",
			func(
				ctx context.Context,
				netConfigParams networkAttachmentConfigParams,
			) {
				namespace := f.Namespace.Name
				jig := e2eservice.NewTestJig(cs, namespace, "udn-service")

				// UDN namespaces must be annotated
				err := annotateFrameworkNamespaceForUDN(ctx, f)
				Expect(err).NotTo(HaveOccurred())

				netConfigParams.cidr = sanitizeCIDRString(oc, netConfigParams.cidr)

				infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				cloudType := infra.Spec.PlatformSpec.Type

				By("Selecting at most 3 schedulable nodes (min 1)")
				nodes, err := e2enode.GetBoundedReadySchedulableNodes(ctx, f.ClientSet, 3)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodes.Items)).To(BeNumerically(">", 0))

				By("Selecting nodes for pods and service")
				serverPodNodeName := nodes.Items[0].Name
				clientNode := nodes.Items[0].Name
				if len(nodes.Items) > 1 {
					clientNode = nodes.Items[1].Name // when client runs on a different node than the server
				}

				By("Creating the attachment configuration")
				netConfig := newNetworkAttachmentConfig(netConfigParams)
				netConfig.namespace = f.Namespace.Name
				_, err = nadClient.NetworkAttachmentDefinitions(f.Namespace.Name).Create(
					ctx,
					generateNAD(netConfig),
					metav1.CreateOptions{},
				)
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("Creating a UDN LoadBalancer service"))
				policy := v1.IPFamilyPolicyPreferDualStack
				udnService, err := jig.CreateUDPService(ctx, func(s *v1.Service) {
					s.Spec.Ports = []v1.ServicePort{
						{
							Name:       "udp",
							Protocol:   v1.ProtocolUDP,
							Port:       servicePort,
							TargetPort: intstr.FromInt(serviceTargetPort),
						},
					}
					s.Spec.Type = v1.ServiceTypeLoadBalancer
					s.Spec.IPFamilyPolicy = &policy
					if cloudType == configv1.AWSPlatformType {
						s.Annotations = map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": "nlb"}
					}

				})
				framework.ExpectNoError(err)

				By("Wait for UDN LoadBalancer Ingress to pop up")
				udnService, err = jig.WaitForLoadBalancer(ctx, 180*time.Second)
				framework.ExpectNoError(err)

				By("Creating a UDN backend pod")
				udnServerPod := e2epod.NewAgnhostPod(
					namespace, "backend-pod", nil, nil,
					[]v1.ContainerPort{
						{ContainerPort: (serviceTargetPort), Protocol: "UDP"}},
					"-c",
					fmt.Sprintf(`
set -xe
iface=ovn-udn1
ips=$(ip -o addr show dev $iface| grep global |awk '{print $4}' | cut -d/ -f1 | paste -sd, -)
./agnhost netexec --udp-port=%d --udp-listen-addresses=$ips
`, serviceTargetPort))
				udnServerPod.Spec.Containers[0].Command = []string{"/bin/bash"}

				udnServerPod.Labels = jig.Labels
				udnServerPod.Spec.NodeName = serverPodNodeName
				udnServerPod = e2epod.NewPodClient(f).CreateSync(ctx, udnServerPod)

				By(fmt.Sprintf("Creating a UDN client pod on the same node (%s)", udnServerPod.Spec.NodeName))
				udnClientPod := e2epod.NewAgnhostPod(namespace, "udn-client", nil, nil, nil)
				udnClientPod.Spec.NodeName = udnServerPod.Spec.NodeName
				udnClientPod = e2epod.NewPodClient(f).CreateSync(ctx, udnClientPod)

				// UDN -> UDN
				By("Connect to the UDN service cluster IP from the UDN client pod on the same node")
				checkConnectionToClusterIPs(ctx, f, udnClientPod, udnService, udnServerPod.Name)
				By("Connect to the UDN service nodePort on all 3 nodes from the UDN client pod")
				checkConnectionToLoadBalancers(ctx, f, udnClientPod, udnService, udnServerPod.Name)
				nodeRoles := []string{"same node", "other node", "other node"}
				for i := range nodes.Items {
					checkConnectionToNodePort(ctx, f, udnClientPod, udnService, &nodes.Items[i], nodeRoles[i], udnServerPod.Name)
				}

				var udnClientPod2 *v1.Pod
				if len(nodes.Items) > 1 {
					By(fmt.Sprintf("Creating a UDN client pod on a different node (%s)", clientNode))
					udnClientPod2 = e2epod.NewAgnhostPod(namespace, "udn-client2", nil, nil, nil)
					udnClientPod2.Spec.NodeName = clientNode
					udnClientPod2 = e2epod.NewPodClient(f).CreateSync(ctx, udnClientPod2)

					By("Connect to the UDN service from the UDN client pod on a different node")
					checkConnectionToClusterIPs(ctx, f, udnClientPod2, udnService, udnServerPod.Name)
					checkConnectionToLoadBalancers(ctx, f, udnClientPod2, udnService, udnServerPod.Name)
					nodeRoles = []string{"server node", "local node", "other node"}
					for i := range nodes.Items {
						checkConnectionToNodePort(ctx, f, udnClientPod2, udnService, &nodes.Items[i], nodeRoles[i], udnServerPod.Name)
					}
				}
				// TODO: Deploy an external container (on the bootstrap node?) and uncomment the lines below
				// By("Connect to the UDN service from the UDN client external container")
				// // checkConnectionToLoadBalancersFromExternalContainer(f, clientContainer, udnService, udnServerPod.Name)
				// checkConnectionToNodePortFromExternalContainer(f, clientContainer, udnService, &nodes.Items[0], "server node", udnServerPod.Name)
				// checkConnectionToNodePortFromExternalContainer(f, clientContainer, udnService, &nodes.Items[1], "other node", udnServerPod.Name)
				// checkConnectionToNodePortFromExternalContainer(f, clientContainer, udnService, &nodes.Items[2], "other node", udnServerPod.Name)

				// Default network -> UDN
				// Check that it cannot connect
				By(fmt.Sprintf("Create a client pod in the default network on node %s", clientNode))
				defaultNetNamespace := fDefault.Namespace // origin specific
				f.AddNamespacesToDelete(defaultNetNamespace)
				defaultClient, err := createPodOnNode(fDefault.ClientSet, "default-net-pod", clientNode, defaultNetNamespace.GetName(), nil, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Verify the connection of the client in the default network to the UDN service")
				checkNoConnectionToClusterIPs(ctx, fDefault, defaultClient, udnService)
				// checkNoConnectionToLoadBalancers(ctx, fDefault, defaultClient, udnService)
				nodeRoles = []string{"server node", "local node", "other node"}
				for i, node := range nodes.Items {
					if node.Name == defaultClient.Spec.NodeName {
						// TODO change to checkConnectionToNodePort when we have full UDN support in ovnkube-node
						checkNoConnectionToNodePort(ctx, fDefault, defaultClient, udnService, &node, nodeRoles[i])
					} else {
						checkConnectionToNodePort(ctx, fDefault, defaultClient, udnService, &node, nodeRoles[i], udnServerPod.Name)
					}
				}
				// UDN -> Default network
				// Create a backend pod and service in the default network and verify that the client pod in the UDN
				// cannot reach it
				By(fmt.Sprintf("Creating a backend pod in the default network on node %s", serverPodNodeName))
				defaultLabels := map[string]string{"app": "default-app"}

				defaultServerPod, err := createPodOnNode(fDefault.ClientSet, "backend-pod-default", serverPodNodeName,
					defaultNetNamespace.GetName(), []string{"/agnhost"}, defaultLabels,
					func(pod *corev1.Pod) {
						pod.Spec.Containers[0].Ports = []corev1.ContainerPort{{ContainerPort: (serviceTargetPort), Protocol: "UDP"}}
						pod.Spec.Containers[0].Args = []string{"netexec", "--udp-port=" + fmt.Sprint(serviceTargetPort)}
					})
				Expect(err).NotTo(HaveOccurred())

				By("create a node port service in the default network")
				defaultService := &v1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: "service-default"},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{
							{
								Name:       "udp-port",
								Port:       int32(servicePort),
								Protocol:   v1.ProtocolUDP,
								TargetPort: intstr.FromInt(serviceTargetPort),
							},
						},
						Selector:       defaultLabels,
						Type:           v1.ServiceTypeNodePort,
						IPFamilyPolicy: &policy,
					},
				}

				defaultService, err = f.ClientSet.CoreV1().Services(defaultNetNamespace.Name).Create(ctx, defaultService, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				// UDN -> default
				By("Verify the UDN client connection to the default network service")
				udnClientPodTmp := udnClientPod
				nodeRoles = []string{"server node"}
				if len(nodes.Items) > 1 {
					udnClientPodTmp = udnClientPod2
					nodeRoles = []string{"server node", "local node", "other node"}
				}
				checkNoConnectionToLoadBalancers(ctx, f, udnClientPodTmp, defaultService)
				for i, node := range nodes.Items {
					if node.Name == udnClientPodTmp.Spec.NodeName {
						checkNoConnectionToNodePort(ctx, f, udnClientPodTmp, defaultService, &node, nodeRoles[i])
					} else {
						checkConnectionToNodePort(ctx, f, udnClientPodTmp, defaultService, &node, nodeRoles[i], defaultServerPod.Name)
					}
				}
				checkNoConnectionToClusterIPs(ctx, f, udnClientPodTmp, defaultService)

				// Make sure that restarting OVNK after applying a UDN with an affected service won't result
				// in OVNK in CLBO state https://issues.redhat.com/browse/OCPBUGS-41499
				if netConfigParams.topology == "layer3" { // no need to run it for layer 2 as well
					By("Restart ovnkube-node on one node and verify that the new ovnkube-node pod goes to the running state")
					err = restartOVNKubeNodePod(cs, ovnNamespace, clientNode)
					Expect(err).NotTo(HaveOccurred())
				}
			},

			Entry(
				"L3 primary UDN, cluster-networked pods, NodePort service",
				networkAttachmentConfigParams{
					name:     nadName,
					topology: "layer3",
					cidr:     strings.Join([]string{userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet}, ","),
					role:     "primary",
				},
			),
			Entry(
				"L2 primary UDN, cluster-networked pods, NodePort service",
				networkAttachmentConfigParams{
					name:     nadName,
					topology: "layer2",
					cidr:     strings.Join([]string{userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet}, ","),
					role:     "primary",
				},
			),
		)

	})

})

// TODO Once https://github.com/ovn-org/ovn-kubernetes/pull/4567 merges, use the vendored *TestJig.Run(), which tests
// the reachability of a service through its name and through its cluster IP. For now only test the cluster IP.

const OvnNodeIfAddr = "k8s.ovn.org/node-primary-ifaddr"

type primaryIfAddrAnnotation struct {
	IPv4 string `json:"ipv4,omitempty"`
	IPv6 string `json:"ipv6,omitempty"`
}

// ParseNodeHostIPDropNetMask returns the parsed host IP addresses found on a node's host CIDR annotation. Removes the mask.
func ParseNodeHostIPDropNetMask(node *kapi.Node) (sets.Set[string], error) {
	nodeIfAddrAnnotation, ok := node.Annotations[OvnNodeIfAddr]
	if !ok {
		return nil, newAnnotationNotSetError("%s annotation not found for node %q", OvnNodeIfAddr, node.Name)
	}
	nodeIfAddr := &primaryIfAddrAnnotation{}
	if err := json.Unmarshal([]byte(nodeIfAddrAnnotation), nodeIfAddr); err != nil {
		return nil, fmt.Errorf("failed to unmarshal annotation: %s for node %q, err: %v", OvnNodeIfAddr, node.Name, err)
	}

	var cfg []string
	if nodeIfAddr.IPv4 != "" {
		cfg = append(cfg, nodeIfAddr.IPv4)
	}
	if nodeIfAddr.IPv6 != "" {
		cfg = append(cfg, nodeIfAddr.IPv6)
	}
	if len(cfg) == 0 {
		return nil, fmt.Errorf("node: %q does not have any IP information set", node.Name)
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

func checkConnectionToAgnhostPod(ctx context.Context, f *framework.Framework, clientPod *v1.Pod, expectedOutput, cmd string, timeout int) error {
	return wait.PollUntilContextTimeout(ctx, 200*time.Millisecond, time.Duration(timeout)*time.Second, true, func(ctx context.Context) (bool, error) {
		stdout, stderr, err2 := e2epod.ExecShellInPodWithFullOutput(ctx, f, clientPod.Name, cmd)
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

func checkNoConnectionToAgnhostPod(ctx context.Context, f *framework.Framework, clientPod *v1.Pod, cmd string, timeout int) error {
	err := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, time.Duration(timeout)*time.Second, true, func(ctx context.Context) (bool, error) {
		stdout, stderr, err2 := e2epod.ExecShellInPodWithFullOutput(ctx, f, clientPod.Name, cmd)
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

func checkConnectionToClusterIPs(ctx context.Context, f *framework.Framework, clientPod *v1.Pod, service *v1.Service, expectedOutput string) {
	checkConnectionOrNoConnectionToClusterIPs(ctx, f, clientPod, service, expectedOutput, true)
}

func checkNoConnectionToClusterIPs(ctx context.Context, f *framework.Framework, clientPod *v1.Pod, service *v1.Service) {
	checkConnectionOrNoConnectionToClusterIPs(ctx, f, clientPod, service, "", false)
}

func checkConnectionOrNoConnectionToClusterIPs(ctx context.Context, f *framework.Framework, clientPod *v1.Pod, service *v1.Service, expectedOutput string, shouldConnect bool) {
	var err error
	servicePort := service.Spec.Ports[0].Port
	notStr := ""
	if !shouldConnect {
		notStr = "not "
	}

	for _, clusterIP := range service.Spec.ClusterIPs {
		msg := fmt.Sprintf("Client %s/%s should %sreach service %s/%s on cluster IP %s port %d",
			clientPod.Namespace, clientPod.Name, notStr, service.Namespace, service.Name, clusterIP, servicePort)
		By(msg)

		cmd := fmt.Sprintf(`/bin/sh -c 'echo hostname | nc -u -w 1 %s %d '`, clusterIP, servicePort)

		if shouldConnect {
			err = checkConnectionToAgnhostPod(ctx, f, clientPod, expectedOutput, cmd, 5)
		} else {
			err = checkNoConnectionToAgnhostPod(ctx, f, clientPod, cmd, 5)
		}
		framework.ExpectNoError(err, fmt.Sprintf("Failed to verify that %s", msg))
	}
}

func checkConnectionToNodePort(ctx context.Context, f *framework.Framework, clientPod *v1.Pod, service *v1.Service, node *v1.Node, nodeRoleMsg, expectedOutput string) {
	checkConnectionOrNoConnectionToNodePort(ctx, f, clientPod, service, node, nodeRoleMsg, expectedOutput, true)
}

func checkNoConnectionToNodePort(ctx context.Context, f *framework.Framework, clientPod *v1.Pod, service *v1.Service, node *v1.Node, nodeRoleMsg string) {
	checkConnectionOrNoConnectionToNodePort(ctx, f, clientPod, service, node, nodeRoleMsg, "", false)
}

func checkConnectionOrNoConnectionToNodePort(ctx context.Context, f *framework.Framework, clientPod *v1.Pod, service *v1.Service, node *v1.Node, nodeRoleMsg, expectedOutput string, shouldConnect bool) {
	var err error
	nodePort := service.Spec.Ports[0].NodePort
	notStr := ""
	if !shouldConnect {
		notStr = "not "
	}
	nodeIPs, err := ParseNodeHostIPDropNetMask(node)
	Expect(err).NotTo(HaveOccurred())

	for nodeIP := range nodeIPs {
		msg := fmt.Sprintf("Client %s/%s should %sconnect to NodePort service %s/%s on %s:%d (node %s, %s)",
			clientPod.Namespace, clientPod.Name, notStr, service.Namespace, service.Name, nodeIP, nodePort, node.Name, nodeRoleMsg)
		By(msg)
		cmd := fmt.Sprintf(`/bin/sh -c 'echo hostname | nc -u -w 1 %s %d '`, nodeIP, nodePort)

		if shouldConnect {
			err = checkConnectionToAgnhostPod(ctx, f, clientPod, expectedOutput, cmd, 30)
		} else {
			err = checkNoConnectionToAgnhostPod(ctx, f, clientPod, cmd, 30)
		}
		framework.ExpectNoError(err, fmt.Sprintf("Failed to verify that %s", msg))
	}
}

func checkConnectionToLoadBalancers(ctx context.Context, f *framework.Framework, clientPod *v1.Pod, service *v1.Service, expectedOutput string) {
	checkConnectionOrNoConnectionToLoadBalancers(ctx, f, clientPod, service, expectedOutput, true)
}

func checkNoConnectionToLoadBalancers(ctx context.Context, f *framework.Framework, clientPod *v1.Pod, service *v1.Service) {
	checkConnectionOrNoConnectionToLoadBalancers(ctx, f, clientPod, service, "", false)
}

func checkConnectionOrNoConnectionToLoadBalancers(ctx context.Context,
	f *framework.Framework, clientPod *v1.Pod, service *v1.Service,
	expectedOutput string, shouldConnect bool) {

	var err error
	port := service.Spec.Ports[0].Port
	notStr := ""
	if !shouldConnect {
		notStr = "not "
	}
	for _, lbIngress := range service.Status.LoadBalancer.Ingress {
		lbTarget := lbIngress.IP
		if lbIngress.IP == "" {
			lbTarget = lbIngress.Hostname
			// The propagation of the new DNS record might not be fast on all cloud platforms.
			waitForURLToResolve(lbTarget)
		}

		msg := fmt.Sprintf("Client %s/%s should %sreach service %s/%s on LoadBalancer %s (loadbalancer=%+v) port %d",
			clientPod.Namespace, clientPod.Name, notStr, service.Namespace, service.Name, lbTarget, lbIngress, port)
		By(msg)

		cmd := fmt.Sprintf(`/bin/sh -c 'echo hostname | nc -u -w 1 %s %d '`, lbTarget, port)

		if shouldConnect {
			err = checkConnectionToAgnhostPod(ctx, f, clientPod, expectedOutput, cmd, 600) // use a high timeout for LB services
		} else {
			err = checkNoConnectionToAgnhostPod(ctx, f, clientPod, cmd, 120)
		}
		framework.ExpectNoError(err, fmt.Sprintf("Failed to verify that %s", msg))
	}
}

func waitForURLToResolve(url string) {
	fmt.Printf("Waiting for url %s to resolve to an IP\n", url)
	Eventually(func() (string, error) {
		ips, err := net.LookupIP(url)
		if err != nil {
			return "", err
		}
		if len(ips) == 0 {
			return "", fmt.Errorf("no IPs found for hostname %s", url)
		}
		fmt.Printf("Resolved hostname %s to IP %s\n", url, ips[0].String())
		return ips[0].String(), nil

	}).WithTimeout(3 * time.Minute).
		WithPolling(200 * time.Millisecond).
		ShouldNot(BeEmpty())
}

// TODO Deploy an external container (on the bootstrap node?) and uncomment the lines below
// func checkConnectionToNodePortFromExternalContainer(f *framework.Framework, containerName string, service *v1.Service, node *v1.Node, nodeRoleMsg, expectedOutput string) {
// 	GinkgoHelper()
// 	var err error
// 	nodePort := service.Spec.Ports[0].NodePort
// 	nodeIPs, err := ParseNodeHostIPDropNetMask(node)
// 	Expect(err).NotTo(HaveOccurred())

// 	for nodeIP := range nodeIPs {
// 		msg := fmt.Sprintf("Client at external container %s should connect to NodePort service %s/%s on %s:%d (node %s, %s)",
// 			containerName, service.Namespace, service.Name, nodeIP, nodePort, node.Name, nodeRoleMsg)
// 		By(msg)
// 		cmd := []string{containerRuntime, "exec", containerName, "/bin/bash", "-c", fmt.Sprintf("echo hostname | nc -u -w 1 %s %d", nodeIP, nodePort)}
// 		Eventually(func() (string, error) {
// 			return runCommand(cmd...)
// 		}).
// 			WithTimeout(5*time.Second).
// 			WithPolling(200*time.Millisecond).
// 			Should(Equal(expectedOutput), "Failed to verify that %s", msg)
// 	}
// }

// func checkConnectionToLoadBalancersFromExternalContainer(f *framework.Framework, containerName string, service *v1.Service, expectedOutput string) {
// 	GinkgoHelper()
// 	port := service.Spec.Ports[0].Port
// 	for _, lbIngress := range service.Status.LoadBalancer.Ingress {
// 		msg := fmt.Sprintf("Client at external container %s should reach service %s/%s on LoadBalancer IP %s port %d",
// 			containerName, service.Namespace, service.Name, lbIngress.IP, port)
// 		By(msg)
// 		cmd := []string{containerRuntime, "exec", containerName, "/bin/bash", "-c", fmt.Sprintf("echo hostname | nc -u -w 1 %s %d", lbIngress.IP, port)}
// 		Eventually(func() (string, error) {
// 			return runCommand(cmd...)
// 		}).
// 			// It takes some time for the container to receive the dynamic routing
// 			WithTimeout(20*time.Second).
// 			WithPolling(200*time.Millisecond).
// 			Should(Equal(expectedOutput), "Failed to verify that %s", msg)
// 	}
// }

func annotateFrameworkNamespaceForUDN(ctx context.Context, f *framework.Framework) error {
	namespace := f.Namespace
	annotationKey := "k8s.ovn.org/primary-user-defined-network"

	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations": {"%s":""}}}`, annotationKey))
	if err := retry.RetryOnConflict(wait.Backoff{Steps: 10, Duration: time.Second}, func() error {
		_, err := f.ClientSet.CoreV1().Namespaces().Patch(ctx, namespace.Name, types.MergePatchType,
			patch,
			metav1.PatchOptions{})
		return err
	}); err != nil {
		return err
	}

	return nil
}
