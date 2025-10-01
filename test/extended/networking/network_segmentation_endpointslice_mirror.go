package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	nadapi "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"
	utilnet "k8s.io/utils/net"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = Describe("[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring", func() {
	defer GinkgoRecover()
	// disable automatic namespace creation, we need to add the required UDN label
	oc := exutil.NewCLIWithoutNamespace("endpointslices-mirror-e2e")
	f := oc.KubeFramework()
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged
	InOVNKubernetesContext(func() {
		const (
			userDefinedNetworkIPv4Subnet = "203.203.0.0/16"
			userDefinedNetworkIPv6Subnet = "2014:100:200::0/60"
			nadName                      = "gryffindor"
		)

		var (
			cs        clientset.Interface
			nadClient nadclient.K8sCniCncfIoV1Interface
		)

		BeforeEach(func() {
			cs = f.ClientSet
			namespace, err := f.CreateNamespace(context.TODO(), f.BaseName, map[string]string{
				"e2e-framework":           f.BaseName,
				RequiredUDNNamespaceLabel: "",
			})
			f.Namespace = namespace
			Expect(err).NotTo(HaveOccurred())
			err = udnWaitForOpenShift(oc, namespace.Name)
			Expect(err).NotTo(HaveOccurred())
			nadClient, err = nadclient.NewForConfig(f.ClientConfig())
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTableSubtree("created using",
			func(createNetworkFn func(c networkAttachmentConfigParams) error) {

				DescribeTable(
					"mirrors EndpointSlices managed by the default controller for namespaces with user defined primary networks",
					func(
						netConfig networkAttachmentConfigParams,
						isHostNetwork bool,
					) {
						By("creating the network")
						// correctCIDRFamily makes use of the ginkgo framework so it needs to be in the testcase
						netConfig.cidr = correctCIDRFamily(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet)
						netConfig.namespace = f.Namespace.Name
						Expect(createNetworkFn(netConfig)).To(Succeed())

						By("deploying the backend pods")
						replicas := 3
						isDualStack := false
						for i := 0; i < replicas; i++ {
							p := runUDNPod(cs, f.Namespace.Name,
								*podConfig(fmt.Sprintf("backend-%d", i), func(cfg *podConfiguration) { cfg.namespace = f.Namespace.Name }),
								func(pod *corev1.Pod) {
									pod.Spec.HostNetwork = isHostNetwork
									if pod.Labels == nil {
										pod.Labels = map[string]string{}
									}
									pod.Labels["app"] = "test"
								})
							isDualStack = getIPFamily(p.Status.PodIPs) == DualStack
						}

						By("creating the service")
						svc := e2eservice.CreateServiceSpec("test-service", "", false, map[string]string{"app": "test"})
						familyPolicy := corev1.IPFamilyPolicyPreferDualStack
						svc.Spec.IPFamilyPolicy = &familyPolicy
						_, err := cs.CoreV1().Services(f.Namespace.Name).Create(context.Background(), svc, metav1.CreateOptions{})
						framework.ExpectNoError(err, "Failed creating service %v", err)

						nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
						framework.ExpectNoError(err, "Failed listing worker nodes %v", err)
						Expect(len(nodes.Items)).To(BeNumerically(">", 0))

						By("asserting the mirrored EndpointSlice exists and contains PODs primary IPs")
						Eventually(func() error {
							return validateMirroredEndpointSlices(cs, f.Namespace.Name, svc.Name, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet, replicas, isDualStack, isHostNetwork)
						}, 2*time.Minute, 6*time.Second).Should(Succeed())

						By("removing the mirrored EndpointSlice so it gets recreated")
						err = cs.DiscoveryV1().EndpointSlices(f.Namespace.Name).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", "k8s.ovn.org/service-name", svc.Name)})
						framework.ExpectNoError(err, "Failed removing the mirrored EndpointSlice %v", err)
						Eventually(func() error {
							return validateMirroredEndpointSlices(cs, f.Namespace.Name, svc.Name, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet, replicas, isDualStack, isHostNetwork)
						}, 2*time.Minute, 6*time.Second).Should(Succeed())

						By("removing the service so both EndpointSlices get removed")
						err = cs.CoreV1().Services(f.Namespace.Name).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
						framework.ExpectNoError(err, "Failed removing the service %v", err)
						Eventually(func() error {
							esList, err := cs.DiscoveryV1().EndpointSlices(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", "k8s.ovn.org/service-name", svc.Name)})
							if err != nil {
								return err
							}

							if len(esList.Items) != 0 {
								return fmt.Errorf("expected no mirrored EndpointSlice, got: %d", len(esList.Items))
							}
							return nil
						}, 2*time.Minute, 6*time.Second).Should(Succeed())

					},
					Entry(
						"L2 primary UDN, cluster-networked pods",
						networkAttachmentConfigParams{
							name:     nadName,
							topology: "layer2",
							role:     "primary",
						},
						false,
					),
					Entry(
						"L3 primary UDN, cluster-networked pods",
						networkAttachmentConfigParams{
							name:     nadName,
							topology: "layer3",
							role:     "primary",
						},
						false,
					),
					Entry(
						"L2 primary UDN, host-networked pods",
						networkAttachmentConfigParams{
							name:     nadName,
							topology: "layer2",
							role:     "primary",
						},
						true,
					),
					Entry(
						"L3 primary UDN, host-networked pods",
						networkAttachmentConfigParams{
							name:     nadName,
							topology: "layer3",
							role:     "primary",
						},
						true,
					),
				)
			},
			Entry("NetworkAttachmentDefinitions", func(c networkAttachmentConfigParams) error {
				netConfig := newNetworkAttachmentConfig(c)
				nad := generateNAD(oc, netConfig)
				_, err := nadClient.NetworkAttachmentDefinitions(f.Namespace.Name).Create(context.Background(), nad, metav1.CreateOptions{})
				return err
			}),
			Entry("UserDefinedNetwork", func(c networkAttachmentConfigParams) error {
				udnManifest := generateUserDefinedNetworkManifest(oc, &c)
				cleanup, err := createManifest(f.Namespace.Name, udnManifest)
				DeferCleanup(cleanup)
				Eventually(userDefinedNetworkReadyFunc(oc.AdminDynamicClient(), f.Namespace.Name, c.name), 5*time.Second, time.Second).Should(Succeed())
				return err
			}),
		)

		DescribeTableSubtree("created using",
			func(createNetworkFn func(c networkAttachmentConfigParams) error) {
				DescribeTable(
					"does not mirror EndpointSlices in namespaces not using user defined primary networks",
					func(
						netConfig networkAttachmentConfigParams,
					) {
						netConfig.cidr = correctCIDRFamily(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet)
						By("creating default net namespace")
						defaultNSName := f.BaseName + "-default"
						defaultNetNamespace, err := f.CreateNamespace(context.TODO(), defaultNSName, map[string]string{
							"e2e-framework": defaultNSName,
						})
						Expect(err).NotTo(HaveOccurred())
						err = udnWaitForOpenShift(oc, defaultNetNamespace.Name)
						Expect(err).NotTo(HaveOccurred())
						By("creating the network")
						netConfig.namespace = defaultNetNamespace.Name
						Expect(createNetworkFn(netConfig)).To(Succeed())

						By("deploying the backend pods")
						replicas := 3
						for i := 0; i < replicas; i++ {
							runUDNPod(cs, defaultNetNamespace.Name,
								*podConfig(fmt.Sprintf("backend-%d", i), func(cfg *podConfiguration) {
									cfg.namespace = defaultNetNamespace.Name
									// Add the net-attach annotation for secondary networks
									if netConfig.role == "secondary" {
										cfg.attachments = []nadapi.NetworkSelectionElement{{Name: netConfig.name}}
									}
								}),
								func(pod *corev1.Pod) {
									if pod.Labels == nil {
										pod.Labels = map[string]string{}
									}
									pod.Labels["app"] = "test"

								})
						}

						By("creating the service")
						svc := e2eservice.CreateServiceSpec("test-service", "", false, map[string]string{"app": "test"})
						familyPolicy := corev1.IPFamilyPolicyPreferDualStack
						svc.Spec.IPFamilyPolicy = &familyPolicy
						_, err = cs.CoreV1().Services(defaultNetNamespace.Name).Create(context.Background(), svc, metav1.CreateOptions{})
						framework.ExpectNoError(err, "Failed creating service %v", err)

						By("asserting the mirrored EndpointSlice does not exist")
						Eventually(func() error {
							esList, err := cs.DiscoveryV1().EndpointSlices(defaultNetNamespace.Name).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", "k8s.ovn.org/service-name", svc.Name)})
							if err != nil {
								return err
							}

							if len(esList.Items) != 0 {
								return fmt.Errorf("expected no mirrored EndpointSlice, got: %d", len(esList.Items))
							}
							return nil
						}, 2*time.Minute, 6*time.Second).Should(Succeed())
					},
					Entry(
						"L2 dualstack primary UDN",
						networkAttachmentConfigParams{
							name:     nadName,
							topology: "layer2",
							role:     "secondary",
						},
					),
					Entry(
						"L3 dualstack primary UDN",
						networkAttachmentConfigParams{
							name:     nadName,
							topology: "layer3",
							role:     "secondary",
						},
					),
				)
			},
			Entry("NetworkAttachmentDefinitions", func(c networkAttachmentConfigParams) error {
				netConfig := newNetworkAttachmentConfig(c)
				nad := generateNAD(oc, netConfig)
				_, err := nadClient.NetworkAttachmentDefinitions(c.namespace).Create(context.Background(), nad, metav1.CreateOptions{})
				return err
			}),
			Entry("UserDefinedNetwork", func(c networkAttachmentConfigParams) error {
				udnManifest := generateUserDefinedNetworkManifest(oc, &c)
				cleanup, err := createManifest(c.namespace, udnManifest)
				DeferCleanup(cleanup)
				Eventually(userDefinedNetworkReadyFunc(oc.AdminDynamicClient(), c.namespace, c.name), 5*time.Second, time.Second).Should(Succeed())
				return err
			}),
		)
	})
})

// parseHostSubnet parses the "k8s.ovn.org/host-cidrs" annotation and returns the v4 and v6 host CIDRs
func parseHostSubnet(node *corev1.Node) (string, string, error) {
	const ovnNodeHostCIDRAnnot = "k8s.ovn.org/host-cidrs"
	var v4HostSubnet, v6HostSubnet string
	addrAnnotation, ok := node.Annotations[ovnNodeHostCIDRAnnot]
	if !ok {
		return "", "", fmt.Errorf("%s annotation not found for node %q", ovnNodeHostCIDRAnnot, node.Name)
	}

	var subnets []string
	if err := json.Unmarshal([]byte(addrAnnotation), &subnets); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal %s annotation %s for node %q: %v", ovnNodeHostCIDRAnnot,
			addrAnnotation, node.Name, err)
	}

	for _, subnet := range subnets {
		ipFamily := utilnet.IPFamilyOfCIDRString(subnet)
		if ipFamily == utilnet.IPv6 {
			v6HostSubnet = subnet
		} else if ipFamily == utilnet.IPv4 {
			v4HostSubnet = subnet
		}
	}
	return v4HostSubnet, v6HostSubnet, nil
}

func validateMirroredEndpointSlices(cs clientset.Interface, namespace, svcName, expectedV4Subnet, expectedV6Subnet string, expectedEndpoints int, isDualStack, isHostNetwork bool) error {
	esList, err := cs.DiscoveryV1().EndpointSlices(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", "k8s.ovn.org/service-name", svcName)})
	if err != nil {
		return err
	}

	expectedEndpointSlicesCount := 1
	if isDualStack {
		expectedEndpointSlicesCount = 2
	}
	if len(esList.Items) != expectedEndpointSlicesCount {
		return fmt.Errorf("expected %d mirrored EndpointSlice, got: %d", expectedEndpointSlicesCount, len(esList.Items))
	}

	for _, endpointSlice := range esList.Items {
		if len(endpointSlice.Endpoints) != expectedEndpoints {
			return fmt.Errorf("expected %d endpoints, got: %d", expectedEndpoints, len(esList.Items))
		}

		subnet := expectedV4Subnet
		if endpointSlice.AddressType == discoveryv1.AddressTypeIPv6 {
			subnet = expectedV6Subnet
		}

		for _, endpoint := range endpointSlice.Endpoints {
			if len(endpoint.Addresses) != 1 {
				return fmt.Errorf("expected 1 endpoint, got: %d", len(endpoint.Addresses))
			}

			if isHostNetwork {
				if endpoint.NodeName == nil {
					return fmt.Errorf("expected node name for endpoint, got: nil")
				}

				nodeIP, err := getNodeIP(cs, *endpoint.NodeName, endpointSlice.AddressType == discoveryv1.AddressTypeIPv6)
				if err != nil {
					return err
				}
				if !nodeIP.Equal(net.ParseIP(endpoint.Addresses[0])) {
					return fmt.Errorf("ip %q is not equal to the node IP %v", endpoint.Addresses[0], nodeIP)
				}
			} else {
				if err := inRange(subnet, endpoint.Addresses[0]); err != nil {
					return err
				}
			}

		}
	}
	return nil
}

func getNodeIP(cs clientset.Interface, nodeName string, isIPv6 bool) (net.IP, error) {
	node, err := cs.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP && utilnet.IsIPv6String(addr.Address) == isIPv6 {
			return net.ParseIP(addr.Address), nil
		}
	}
	return nil, fmt.Errorf("no matching node IPs found in %s", node.Status.Addresses)
}
