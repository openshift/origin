package networking

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	frrapi "github.com/metallb/frr-k8s/api/v1beta1"
	ratypes "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/routeadvertisements/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	// for all tests
	bgpNamespacePrefix = "bgp"
	bgpAgentPodName    = "bgp-prober-pod"
	targetProtocol     = "http"
	serverPort         = 8000

	// We run an agnhost container in the dev-scripts host
	v4ExternalIP   = "172.20.0.100"
	v4ExternalCIDR = "172.20.0.0/16"

	v6ExternalIP   = "2001:db8:2::100"
	v6ExternalCIDR = "2001:db8:2::/64"

	frrNamespace = "openshift-frr-k8s"
	raLabel      = "k8s.ovn.org/route-advertisements"

	// IP allocation parameters
	// The pools being defined here for the BGP test cases should not be used by
	// any other tests in the same suite
	allocationConfigMapNamespace = "default"
	allocationConfigMapName      = "e2e-test-ovn-bgp-allocations"
	subnetAllocationKey          = "allocated-subnets"
	subnetAllocationTemplatev4   = "203.%d.0.0/16"
	subnetAllocationTemplatev6   = "2014:100:200:%02x0::0/60"
	subnetAllocationMin          = 110
	subnetAllocationMax          = 140
	eipAllocationKey             = "allocated-eips"
	eipAllocationTemplatev4      = "192.168.111.%d"
	eipAllocationTemplatev6      = "fd2e:6f44:5dd8:c956::%0x"
	eipAllocationMin             = 110
	eipAllocationMax             = 170

	// These variables are specific to VRF Lite testing. Thay make assumptions
	// that are aligned to how the job itself configures the infra environment.
	// 'agnhost_extranet' is the hostname of the external agnhost container that
	// is only connected to the extra network. This extra network is accessible
	// to the cluster nodes through enp3s0 interface.
	// TODO: Don't make assumptions and find a way to know about these
	vrfLiteCUDNName         = "extranet"
	vrfLiteExternalHostname = "agnhost_extranet"
	vrfLiteSnifferInterface = "enp3s0"
	// This policy attaches enp3s0 to the extra network / CUDN VRF
	// TODO: consider bringing in NNCP API
	vrfLiteOnNNCP = `
apiVersion: nmstate.io/v1
kind: NodeNetworkConfigurationPolicy
metadata:
  name: extranet
spec:
  desiredState:
    interfaces:
    - name: enp3s0
      state: up 
      controller: extranet
`
	vrfLiteOffNNCP = `
apiVersion: nmstate.io/v1
kind: NodeNetworkConfigurationPolicy
metadata:
  name: extranet
spec:
  desiredState:
    interfaces:
    - name: enp3s0
      state: up 
      controller: ""
`
	nncpRenderedRegex = `(\d+)/(\d+) nodes successfully configured`
)

var (
	nncpRenderedCompiledRegex = regexp.MustCompile(nncpRenderedRegex)
)

type response struct {
	Responses []string `json:"responses"`
}

var _ = g.Describe("[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io]", func() {
	oc := exutil.NewCLIWithPodSecurityLevel(bgpNamespacePrefix, admissionapi.LevelPrivileged)
	InOVNKubernetesContext(func() {
		var (
			networkPlugin string

			f         *framework.Framework
			clientset kubernetes.Interface
			tmpDirBGP string

			workerNodesOrdered      []corev1.Node
			workerNodesOrderedNames []string
			advertisedPodsNodes     []string
			egressIPNodes           []string
			externalNodeName        string
			targetNamespace         string
			snifferNamespace        string
			cloudType               configv1.PlatformType
			deployName              string
			svcUrl                  string
			packetSnifferDaemonSet  *v1.DaemonSet
			podList                 *corev1.PodList
			v4PodIPSet              map[string]string
			v6PodIPSet              map[string]string
			clusterIPFamily         IPFamily
		)

		g.BeforeEach(func() {
			g.By("Verifying that this cluster uses a network plugin that is supported for this test")
			networkPlugin = networkPluginName()
			if networkPlugin != OVNKubernetesPluginName {
				skipper.Skipf("This cluster does not use OVNKubernetes")
			}

			g.By("Creating a temp directory")
			var err error
			tmpDirBGP, err = os.MkdirTemp("", "bgp-e2e")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Getting the kubernetes clientset")
			f = oc.KubeFramework()
			clientset = f.ClientSet
			targetNamespace = f.Namespace.Name

			g.By("Determining the cloud infrastructure type")
			infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying that the platform is baremetal")
			if infra.Spec.PlatformSpec.Type != configv1.BareMetalPlatformType {
				skipper.Skipf("This cloud platform (%s) is not supported for this test", cloudType)
			}

			// The RouteAdvertisements feature must be enabled by featuregate.
			// Otherwise, skip this test.
			g.By("Verifying that the RouteAdvertisements feature is enabled by featuregate")
			isBGPSupported := false
			featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(context.Background(), "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, feature := range featureGate.Status.FeatureGates[0].Enabled {
				if feature.Name == "RouteAdvertisements" {
					isBGPSupported = true
					break
				}
			}
			if !isBGPSupported {
				skipper.Skipf("The RouteAdvertisements feature is not enabled by featuregate")
			}

			g.By("Verifying that the RouteAdvertisements is enabled in the cluster")
			networkOperator, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			if networkOperator.Spec.AdditionalRoutingCapabilities == nil ||
				networkOperator.Spec.DefaultNetwork.OVNKubernetesConfig == nil ||
				networkOperator.Spec.DefaultNetwork.OVNKubernetesConfig.RouteAdvertisements != "Enabled" {
				skipper.Skipf("The RouteAdvertisements feature is not enabled in the network.operator CR")
			}

			g.By("Getting all worker nodes in alphabetical order")
			// Get all worker nodes, order them alphabetically with stable
			// sort order.
			workerNodesOrdered, err = getWorkerNodesOrdered(clientset)
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, s := range workerNodesOrdered {
				workerNodesOrderedNames = append(workerNodesOrderedNames, s.Name)
			}
			if len(workerNodesOrdered) < 3 {
				skipper.Skipf("This test requires a minimum of 3 worker nodes. However, this environment has %d worker nodes.", len(workerNodesOrdered))
			}

			g.By("Selecting a node to act as as an external host")
			o.Expect(len(workerNodesOrderedNames)).Should(o.BeNumerically(">", 1))
			externalNodeName = workerNodesOrderedNames[0]
			advertisedPodsNodes = workerNodesOrderedNames[1:]
			egressIPNodes = workerNodesOrderedNames[1:]

			g.By("Creating a project for the sniffer pod")
			snifferNamespace = oc.SetupProject()

			clusterIPFamily = getIPFamilyForCluster(f)
		})

		// Do not check for errors in g.AfterEach as the other cleanup steps will fail, otherwise.
		g.AfterEach(func() {
			o.Expect(os.RemoveAll(tmpDirBGP)).To(o.Succeed())
		})

		g.JustAfterEach(func() {
			specReport := g.CurrentSpecReport()
			if specReport.Failed() {
				gatherDebugInfo(oc, packetSnifferDaemonSet, targetNamespace, workerNodesOrderedNames)
			}
		})

		g.Context("[PodNetwork] Advertising the default network [apigroup:user.openshift.io][apigroup:security.openshift.io]", func() {
			g.BeforeEach(func() {
				g.By("Setup packet sniffer at nodes")
				var err error
				packetSnifferDaemonSet, err = setupPacketSniffer(oc, clientset, snifferNamespace, advertisedPodsNodes, networkPlugin)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Ensure the RouteAdvertisements is accepted")
				waitForRouteAdvertisements(oc, "default")

				g.By("Makes sure the FRR configuration is generated for each node")
				for _, nodeName := range workerNodesOrderedNames {
					frr, err := getGeneratedFrrConfigurationForNode(oc, nodeName, "default")
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(frr).NotTo(o.BeNil())
				}

				g.By("Deploy the test pods")
				deployName, _, podList, err = setupTestDeployment(oc, clientset, targetNamespace, advertisedPodsNodes)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(podList.Items)).To(o.Equal(len(advertisedPodsNodes)))
				svcUrl = fmt.Sprintf("%s-0-service.%s.svc.cluster.local:%d", targetNamespace, targetNamespace, serverPort)

				g.By("Extract test pod IPs")
				v4PodIPSet, v6PodIPSet = extractPodIPs(podList)
			})

			g.It("Pods should communicate with external host without being SNATed", func() {
				g.By("Checking that routes are advertised to each node")
				for _, nodeName := range workerNodesOrderedNames {
					verifyLearnedBgpRoutesForNode(oc, nodeName, "default")
				}

				numberOfRequestsToSend := 10
				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and PodIP %v were seen", numberOfRequestsToSend, v4PodIPSet))
				svcUrl := fmt.Sprintf("%s-0-service.%s.svc.cluster.local:%d", targetNamespace, targetNamespace, serverPort)
				if clusterIPFamily == DualStack || clusterIPFamily == IPv4 {
					g.By("sending to IPv4 external host")
					spawnProberSendEgressIPTrafficCheckLogs(oc, snifferNamespace, probePodName, svcUrl, targetProtocol, v4ExternalIP, serverPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, v4PodIPSet)
				}
				if clusterIPFamily == DualStack || clusterIPFamily == IPv6 {
					g.By("sending to IPv6 external host")
					spawnProberSendEgressIPTrafficCheckLogs(oc, snifferNamespace, probePodName, svcUrl, targetProtocol, v6ExternalIP, serverPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, v6PodIPSet)
				}
			})

			g.It("External host should be able to query route advertised pods by the pod IP", func() {
				g.By("Launching an agent pod")
				nodeSelection := e2epod.NodeSelection{}
				e2epod.SetAffinity(&nodeSelection, externalNodeName)
				proberPod := createProberPod(oc, snifferNamespace, bgpAgentPodName)

				if clusterIPFamily == DualStack || clusterIPFamily == IPv4 {
					g.By("checking the external host to pod traffic works for IPv4")
					for podIP := range v4PodIPSet {
						checkExternalResponse(oc, proberPod, podIP, v4ExternalIP, serverPort, packetSnifferDaemonSet, targetProtocol)
					}
				}

				if clusterIPFamily == DualStack || clusterIPFamily == IPv6 {
					g.By("checking the external host to pod traffic works for IPv6")
					for podIP := range v6PodIPSet {
						checkExternalResponse(oc, proberPod, podIP, v6ExternalIP, serverPort, packetSnifferDaemonSet, targetProtocol)
					}
				}
			})
		})

		g.Describe("[PodNetwork] Advertising a cluster user defined network [apigroup:user.openshift.io][apigroup:security.openshift.io]", func() {
			var testCUDNName, testTargetVRF, testCUDNTopology, testSnifferInterface string
			var userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet string
			var cleanup func()

			isTargetVRFLite := func(targetVRF string) bool {
				return targetVRF == "auto"
			}

			toExternalCheck := func() {
				g.GinkgoHelper()

				g.By("Checking that routes are advertised to each node")
				for _, nodeName := range workerNodesOrderedNames {
					verifyLearnedBgpRoutesForNode(oc, nodeName, testCUDNName)
				}

				numberOfRequestsToSend := 10
				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and PodIP %v were seen", numberOfRequestsToSend, v4PodIPSet))

				svcUrl := fmt.Sprintf("%s-0-service:%d", targetNamespace, serverPort)
				if clusterIPFamily == DualStack || clusterIPFamily == IPv4 {
					g.By("sending to IPv4 external host")
					spawnProberSendEgressIPTrafficCheckLogs(oc, targetNamespace, probePodName, svcUrl, targetProtocol, v4ExternalIP, serverPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, v4PodIPSet)
				}
				if clusterIPFamily == DualStack || clusterIPFamily == IPv6 {
					g.By("sending to IPv6 external host")
					spawnProberSendEgressIPTrafficCheckLogs(oc, targetNamespace, probePodName, svcUrl, targetProtocol, v6ExternalIP, serverPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, v6PodIPSet)
				}

				// For VRF-Lite configuration specifically check that the
				// external hostname is that of the one reachable through the
				// CUDN VRF, and not the one reachable on the default VRF, both
				// having same IPs.
				if isTargetVRFLite(testTargetVRF) {
					meetsExpectations := func(og o.Gomega, node string) {
						probe := func(ip string) {
							g.GinkgoHelper()
							responseSet, err := probeForHostnames(oc, targetNamespace, probePodName, svcUrl, ip, serverPort, 1, func(p *corev1.Pod) { p.Spec.NodeName = node })
							og.Expect(err).NotTo(o.HaveOccurred())
							og.Expect(responseSet).To(o.HaveLen(1))
							og.Expect(responseSet).To(o.HaveKey(vrfLiteExternalHostname))
						}
						if clusterIPFamily == DualStack || clusterIPFamily == IPv4 {
							probe(v4ExternalIP)
						}
						if clusterIPFamily == DualStack || clusterIPFamily == IPv6 {
							probe(v6ExternalIP)
						}
					}
					for _, node := range workerNodesOrderedNames {
						g.By("verifying pods on node " + node + " reach the external host through the VRF")
						o.Eventually(meetsExpectations).WithArguments(node).WithTimeout(3 * timeOut).WithPolling(interval).Should(o.Succeed())
						o.Consistently(meetsExpectations).WithArguments(node).WithTimeout(timeOut).WithPolling(interval).Should(o.Succeed())
					}
				}
			}

			fromExternalCheck := func() {
				g.GinkgoHelper()

				g.By("Launching an agent pod")
				nodeSelection := e2epod.NodeSelection{}
				e2epod.SetAffinity(&nodeSelection, externalNodeName)
				proberPod := createProberPod(oc, targetNamespace, bgpAgentPodName)

				if clusterIPFamily == DualStack || clusterIPFamily == IPv4 {
					g.By("checking the external host to pod traffic works for IPv4")
					for podIP := range v4PodIPSet {
						checkExternalResponse(oc, proberPod, podIP, v4ExternalIP, serverPort, packetSnifferDaemonSet, targetProtocol)
					}
				}

				if clusterIPFamily == DualStack || clusterIPFamily == IPv6 {
					g.By("checking the external host to pod traffic works for IPv6")
					for podIP := range v6PodIPSet {
						checkExternalResponse(oc, proberPod, podIP, v6ExternalIP, serverPort, packetSnifferDaemonSet, targetProtocol)
					}
				}
			}

			beforeEach := func(cudnName, targetVRF, cudnTopology, snifferInterface string) {
				g.GinkgoHelper()

				testCUDNName, testTargetVRF, testCUDNTopology, testSnifferInterface = cudnName, targetVRF, cudnTopology, snifferInterface

				var err error
				var snifferPodsNodes []string
				// Check if the cluster is in local gateway mode
				network, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(context.TODO(), "cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				if network.Spec.DefaultNetwork.OVNKubernetesConfig.GatewayConfig != nil && network.Spec.DefaultNetwork.OVNKubernetesConfig.GatewayConfig.RoutingViaHost && testCUDNTopology == "layer2" {
					// TODO: unskip once CORENET-5881 is done.
					skipper.Skipf("Skipping Layer2 UDN advertisements test for local gateway mode")
				}
				if testCUDNTopology == "layer2" {
					// Running the packet sniffer on all nodes in the cluster for Layer2 UDN
					nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					for _, node := range nodes.Items {
						snifferPodsNodes = append(snifferPodsNodes, node.Name)
					}
				} else {
					snifferPodsNodes = advertisedPodsNodes
				}
				g.By("Setup packet sniffer at nodes")
				packetSnifferDaemonSet, err = setupPacketSnifferOnInterface(oc, clientset, snifferNamespace, snifferPodsNodes, networkPlugin, testSnifferInterface)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Create a namespace with UDPN")
				ns, err := f.CreateNamespace(context.TODO(), f.BaseName, map[string]string{
					"e2e-framework":           f.BaseName,
					RequiredUDNNamespaceLabel: "",
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				err = udnWaitForOpenShift(oc, ns.Name)
				o.Expect(err).NotTo(o.HaveOccurred())
				targetNamespace = ns.Name
				f.Namespace = ns

				g.By("Creating a cluster user defined network")
				nc := &networkAttachmentConfigParams{
					name:      testCUDNName,
					topology:  testCUDNTopology,
					role:      "primary",
					namespace: targetNamespace,
				}

				g.By("Allocating subnets for a CUDN")
				userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet, err = allocateSubnets(oc)
				o.Expect(err).NotTo(o.HaveOccurred())
				framework.Logf("Allocated subnets %s %s", userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet)

				nc.cidr = correctCIDRFamily(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet)
				cudnManifest := generateClusterUserDefinedNetworkManifest(nc)
				cleanup, err = createManifest("", cudnManifest)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Eventually(clusterUserDefinedNetworkReadyFunc(oc.AdminDynamicClient(), testCUDNName), 60*time.Second, time.Second).Should(o.Succeed())

				g.By("Labeling the UDN for advertisement")
				_, err = runOcWithRetry(oc.AsAdmin(), "label", "clusteruserdefinednetworks", "-n", targetNamespace, testCUDNName, "network="+testCUDNName)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Create the route advertisement for UDN")
				raManifest := newRouteAdvertisementsManifest(testCUDNName, testTargetVRF, true, false)
				err = applyManifest("", raManifest)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("Ensure the RouteAdvertisements %s is accepted", testCUDNName))
				waitForRouteAdvertisements(oc, testCUDNName)

				g.By("Makes sure the FRR configuration is generated for each node")
				for _, nodeName := range workerNodesOrderedNames {
					frr, err := getGeneratedFrrConfigurationForNode(oc, nodeName, testCUDNName)
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(frr).NotTo(o.BeNil())
				}

				g.By("Deploy the test pods")
				deployName, _, podList, err = setupTestDeployment(oc, clientset, targetNamespace, advertisedPodsNodes)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(podList.Items)).To(o.Equal(len(advertisedPodsNodes)))
				svcUrl = fmt.Sprintf("%s-0-service.%s.svc.cluster.local:%d", targetNamespace, targetNamespace, serverPort)

				g.By("Extract test pod UDN IPs")
				v4PodIPSet, v6PodIPSet = extractPodUdnIPs(podList, nc, targetNamespace, clientset)
			}

			afterEach := func() error {
				err := runOcWithRetryIgnoreOutput(oc.AsAdmin(), "delete", "deploy", deployName)
				errors.Join(err, runOcWithRetryIgnoreOutput(oc.AsAdmin(), "delete", "pod", "--all"))
				errors.Join(err, runOcWithRetryIgnoreOutput(oc.AsAdmin().WithoutNamespace(), "delete", "ra", testCUDNName))
				errors.Join(err, runOcWithRetryIgnoreOutput(oc.AsAdmin().WithoutNamespace(), "delete", "ra", testCUDNName))
				errors.Join(err, runOcWithRetryIgnoreOutput(oc.AsAdmin().WithoutNamespace(), "delete", "clusteruserdefinednetwork", testCUDNName))
				errors.Join(err, deallocateSubnets(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet))
				cleanup()
				return err
			}

			g.DescribeTableSubtree("Over the default VRF",
				func(topology string) {
					g.BeforeEach(func() {
						beforeEach(randomNetworkMetaName(), "", topology, "")
					})
					g.AfterEach(func() {
						o.Expect(afterEach()).To(o.Succeed())
					})
					g.It("Pods should communicate with external host without being SNATed", toExternalCheck)
					g.It("External host should be able to query route advertised pods by the pod IP", fromExternalCheck)
				},
				g.Entry("When the network topology is Layer 3", "layer3"),
				g.Entry("When the network topology is Layer 2", "layer2"),
			)

			// VRF-Lite test cases are serial as they depend on shared provider network configuration
			g.Describe("Over a VRF-Lite configuration", func() {
				g.BeforeEach(func() {
					g.By("Verifying that RoutingViaHost is enabled")
					network, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					isRoutingViaHost := network.Spec.DefaultNetwork.OVNKubernetesConfig != nil &&
						network.Spec.DefaultNetwork.OVNKubernetesConfig.GatewayConfig != nil &&
						network.Spec.DefaultNetwork.OVNKubernetesConfig.GatewayConfig.RoutingViaHost

					if !isRoutingViaHost {
						skipper.Skipf("These tests only apply when RoutingViaHost is enabled")
					}

					// Deploy nmstate handler which is used for rolling out VRF-Lite config
					// via NodeNetworkConfigurationPolicy.
					g.By("deploy nmstate handler")
					err = deployNmstateHandler(oc)
					o.Expect(err).NotTo(o.HaveOccurred())

					// This is a workaround for OCPBUGS-56488: scale down
					// nmstate operator and disable liveness probe
					o.Eventually(workaroundOCPBUGS56488).WithArguments(oc).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(o.BeTrue())
				})

				g.AfterEach(func() {
					o.Expect(func() error {
						err := runOcWithRetryIgnoreOutput(oc.AsAdmin(), "delete", "nncp", "extranet")
						errors.Join(err, afterEach())
						return err
					}()).To(o.Succeed())
				})

				test := func(topology string) {
					// general prerequisites
					beforeEach(vrfLiteCUDNName, "auto", topology, vrfLiteSnifferInterface)

					// we shouldn't need to do this but we have to because of
					// https://issues.redhat.com/browse/RHEL-89914
					// recycle the VRF until it becomes managed
					g.By("ensuring the CUDN VRF is managed by NetworkManager")
					ensureManaged := func(g o.Gomega) {
						status, err := exutil.DebugAllNodesRetryWithOptionsAndChroot(oc, f.Namespace.Name, "nmcli", "-f", "GENERAL.STATE", "device", "show", vrfLiteCUDNName)
						g.Expect(err).NotTo(o.HaveOccurred())
						var recycled bool
						for node, state := range status {
							if strings.Contains(state, "connected") {
								continue
							}
							// add and remove a dummy address to the interface so that NM manages it
							_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, node, f.Namespace.Name, "ip", "address", "add", "dev", vrfLiteCUDNName, "203.0.113.113/32")
							g.Expect(err).NotTo(o.HaveOccurred())
							_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, node, f.Namespace.Name, "ip", "address", "del", "dev", vrfLiteCUDNName, "203.0.113.113/32")
							g.Expect(err).NotTo(o.HaveOccurred())
							recycled = true
						}
						g.Expect(recycled).To(o.BeFalse())
					}
					o.Eventually(ensureManaged).WithTimeout(3 * timeOut).WithPolling(interval).Should(o.Succeed())

					g.By("attaching the secondary network interface to the CUDN VRF")
					applyNNCP(oc, "extranet", vrfLiteOnNNCP)
					g.By("checking that a pod in the network can communicate with external host without being SNATed")
					toExternalCheck()
					g.By("checking that the external host is able to query route advertised pods by pod IP")
					fromExternalCheck()
					g.By("removing the secondary network interface from the CUDN VRF")
					applyNNCP(oc, "extranet", vrfLiteOffNNCP)
				}

				// All VRF-Lite checks for 'vrfLiteCUDNName' need to happen on a
				// single test (or otherwise we should resort to a serial job)
				// This test has a long timeout due to its serial nature and also
				// because of OCPBUGS-56488
				g.It("Pods should be able to communicate on a secondary network [Timeout:30m]", func() {
					g.By("testing with a layer 3 CUDN", func() { test("layer3") })
					// TODO: Add test for layer 2 UDN once CORENET-5881 is done.
					//g.By("testing with a layer 2 CUDN", func() { test("layer2") })
				})
			})
		})

		g.Describe("[EgressIP] Advertising EgressIP [apigroup:user.openshift.io][apigroup:security.openshift.io]", func() {
			var err error
			var egressIPYamlPath, egressIPObjectName string
			var egressIPSet, newEgressIPSet map[string]string

			g.BeforeEach(func() {
				egressIPYamlPath = tmpDirBGP + "/" + egressIPYaml
				g.By("Setting the EgressIP nodes as EgressIP assignable")
				// in order for these tests to run in parallel, they  all use
				// the same egress nodes (all ordered worker nodes but the
				// first), so we will label as necessary but not unlabel. There
				// shouldn't be any other tests in the suite labeling a
				// different set of nodes, or unlabeling any nodes.
				for _, node := range egressIPNodes {
					_, err = runOcWithRetry(oc.AsAdmin(), "label", "node", node, "k8s.ovn.org/egress-assignable=")
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				g.By("Setup packet sniffer at nodes")
				packetSnifferDaemonSet, err = setupPacketSniffer(oc, clientset, snifferNamespace, egressIPNodes, networkPlugin)
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.AfterEach(func() {
				o.Expect(func() error {
					err := runOcWithRetryIgnoreOutput(oc.AsAdmin(), "delete", "egressip", egressIPObjectName, "--cascade=foreground", "--ignore-not-found=true")
					errors.Join(err, deallocateEgressIPSets(oc, egressIPSet, newEgressIPSet))
					return err
				}()).To(o.Succeed())
			})

			g.Describe("For the default network", func() {
				g.BeforeEach(func() {
					egressIPObjectName = targetNamespace

					g.By("Turn on the BGP advertisement of EgressIPs")
					_, err = runOcWithRetry(oc.AsAdmin(), "patch", "ra", "default", "--type=merge", `-p={"spec":{"advertisements":["EgressIP","PodNetwork"]}}`)
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Ensure the RouteAdvertisements is accepted")
					waitForRouteAdvertisements(oc, "default")

					g.By("Makes sure the FRR configuration is generated for each node")
					for _, nodeName := range workerNodesOrderedNames {
						frr, err := getGeneratedFrrConfigurationForNode(oc, nodeName, "default")
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(frr).NotTo(o.BeNil())
					}
				})

				g.DescribeTable("Pods should have the assigned EgressIPs and EgressIPs can be created, updated and deleted [apigroup:route.openshift.io]",
					func(ipFamily IPFamily, externalIP string) {
						if clusterIPFamily != ipFamily && clusterIPFamily != DualStack {
							skipper.Skipf("Skipping test for IPFamily: %s", ipFamily)
							return
						}

						g.By("Allocating an EgressIP sets for the test")
						egressIPSet, newEgressIPSet, err = allocateEgressIPSet(oc, ipFamily, egressIPNodes)
						o.Expect(err).NotTo(o.HaveOccurred())
						framework.Logf("Allocated EgressIP sets: %v %v", egressIPSet, newEgressIPSet)

						g.By("Deploy the test pods")
						deployName, _, podList, err = setupTestDeployment(oc, clientset, targetNamespace, egressIPNodes)
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(len(podList.Items)).To(o.Equal(len(egressIPNodes)))
						svcUrl = fmt.Sprintf("%s-0-service.%s.svc.cluster.local:%d", targetNamespace, targetNamespace, serverPort)

						numberOfRequestsToSend := 10
						// Run this twice to make sure that repeated EgressIP creation, update and deletion works.
						for i := 0; i < 2; i++ {
							g.By("Creating the EgressIP object")
							ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, targetNamespace, "", egressIPSet)

							g.By("Applying the EgressIP object")
							applyEgressIPObject(oc, nil, egressIPYamlPath, targetNamespace, egressIPSet, egressUpdateTimeout)

							g.By("Makes sure the EgressIP is advertised by FRR")
							for eip, nodeName := range egressIPSet {
								o.Expect(nodeName).ShouldNot(o.BeEmpty())
								o.Eventually(func() bool {
									return isRouteToEgressIPPresent(oc, eip, "default", nodeName)
								}).WithTimeout(3 * timeOut).WithPolling(interval).Should(o.BeTrue())
							}

							g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
							spawnProberSendEgressIPTrafficCheckLogs(oc, snifferNamespace, probePodName, svcUrl, targetProtocol, externalIP, serverPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)

							g.By("Updating the EgressIP object")
							ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, targetNamespace, "", newEgressIPSet)

							g.By("Applying the updated EgressIP object")
							applyEgressIPObject(oc, nil, egressIPYamlPath, targetNamespace, newEgressIPSet, egressUpdateTimeout)

							g.By("Makes sure the updated EgressIP is advertised by FRR ")
							for eip, nodeName := range newEgressIPSet {
								o.Expect(nodeName).ShouldNot(o.BeEmpty())
								o.Eventually(func() bool {
									return isRouteToEgressIPPresent(oc, eip, "default", nodeName)
								}).WithTimeout(3 * timeOut).WithPolling(interval).Should(o.BeTrue())
							}

							g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and updated EgressIPs %v were seen", numberOfRequestsToSend, newEgressIPSet))
							spawnProberSendEgressIPTrafficCheckLogs(oc, snifferNamespace, probePodName, svcUrl, targetProtocol, externalIP, serverPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, newEgressIPSet)

							g.By("Deleting the EgressIP object")
							// Use cascading foreground deletion to make sure that the EgressIP object and its dependencies are gone.
							_, err = runOcWithRetry(oc.AsAdmin(), "delete", "egressip", egressIPObjectName, "--cascade=foreground")
							o.Expect(err).NotTo(o.HaveOccurred())

							g.By("Makes sure the EgressIP is not advertised by FRR")
							for eip, nodeName := range newEgressIPSet {
								o.Eventually(func() bool {
									return isRouteToEgressIPPresent(oc, eip, "default", nodeName)
								}).WithTimeout(3 * timeOut).WithPolling(interval).Should(o.BeFalse())
							}

							g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", 0, newEgressIPSet))
							spawnProberSendEgressIPTrafficCheckLogs(oc, snifferNamespace, probePodName, svcUrl, targetProtocol, externalIP, serverPort, numberOfRequestsToSend, 0, packetSnifferDaemonSet, newEgressIPSet)
						}
					},
					g.Entry("When the network is IPv4", IPv4, v4ExternalIP),
					g.Entry("When the network is IPv6", IPv6, v6ExternalIP),
				)
			})

			g.DescribeTableSubtree("For cluster user defined networks",
				func(udnTopology string) {
					var testCUDNName string
					var userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet string
					var cleanup func()

					g.BeforeEach(func() {
						g.By("Create a namespace with UDPN")
						ns, err := f.CreateNamespace(context.TODO(), f.BaseName, map[string]string{
							"e2e-framework":           f.BaseName,
							RequiredUDNNamespaceLabel: "",
						})
						o.Expect(err).NotTo(o.HaveOccurred())
						err = udnWaitForOpenShift(oc, ns.Name)
						o.Expect(err).NotTo(o.HaveOccurred())
						targetNamespace = ns.Name
						f.Namespace = ns
						egressIPObjectName = targetNamespace
						testCUDNName = ns.Name

						g.By("Creating a cluster user defined network")
						nc := &networkAttachmentConfigParams{
							name:      testCUDNName,
							topology:  udnTopology,
							role:      "primary",
							namespace: targetNamespace,
						}

						g.By("Allocating subnets for a CUDN")
						userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet, err = allocateSubnets(oc)
						o.Expect(err).NotTo(o.HaveOccurred())
						framework.Logf("Allocated subnets %s %s", userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet)

						nc.cidr = correctCIDRFamily(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet)
						cudnManifest := generateClusterUserDefinedNetworkManifest(nc)
						cleanup, err = createManifest(targetNamespace, cudnManifest)

						o.Expect(err).NotTo(o.HaveOccurred())
						o.Eventually(clusterUserDefinedNetworkReadyFunc(oc.AdminDynamicClient(), testCUDNName), 60*time.Second, time.Second).Should(o.Succeed())
						g.By("Labeling the UDN for advertisement")
						_, err = runOcWithRetry(oc.AsAdmin(), "label", "clusteruserdefinednetworks", "-n", targetNamespace, testCUDNName, "network="+testCUDNName)
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("Create the route advertisement for UDN")
						raManifest := newRouteAdvertisementsManifest(testCUDNName, "", false, true)
						err = applyManifest("", raManifest)
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By(fmt.Sprintf("Ensure the RouteAdvertisements %s is accepted", testCUDNName))
						waitForRouteAdvertisements(oc, testCUDNName)
					})

					g.AfterEach(func() {
						o.Expect(func() error {
							err := runOcWithRetryIgnoreOutput(oc.AsAdmin(), "delete", "deploy", deployName)
							errors.Join(err, runOcWithRetryIgnoreOutput(oc.AsAdmin(), "delete", "ra", testCUDNName))
							errors.Join(err, runOcWithRetryIgnoreOutput(oc.AsAdmin(), "delete", "pod", "--all"))
							errors.Join(err, runOcWithRetryIgnoreOutput(oc.AsAdmin(), "delete", "clusteruserdefinednetwork", testCUDNName))
							errors.Join(err, deallocateSubnets(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet))
							return err
						}()).To(o.Succeed())
						cleanup()
					})

					g.DescribeTable("UDN pods should have the assigned EgressIPs and EgressIPs can be created, updated and deleted [apigroup:route.openshift.io]",
						func(ipFamily IPFamily, externalIP string) {
							if clusterIPFamily != ipFamily && clusterIPFamily != DualStack {
								skipper.Skipf("Skipping test for IPFamily: %s", ipFamily)
								return
							}

							g.By("Allocating EgressIP sets for the test")
							egressIPSet, newEgressIPSet, err = allocateEgressIPSet(oc, ipFamily, egressIPNodes)
							o.Expect(err).NotTo(o.HaveOccurred())
							framework.Logf("Allocated EgressIP sets: %v %v", egressIPSet, newEgressIPSet)

							g.By("Deploy the test pods")
							deployName, _, podList, err = setupTestDeployment(oc, clientset, targetNamespace, egressIPNodes)
							o.Expect(err).NotTo(o.HaveOccurred())
							o.Expect(len(podList.Items)).To(o.Equal(len(egressIPNodes)))
							svcUrl = fmt.Sprintf("%s-0-service.%s.svc.cluster.local:%d", targetNamespace, targetNamespace, serverPort)

							numberOfRequestsToSend := 10
							// Run this twice to make sure that repeated EgressIP creation and deletion works.
							for i := 0; i < 2; i++ {
								g.By("Creating the EgressIP object")
								ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, targetNamespace, "", egressIPSet)

								g.By("Applying the EgressIP object")
								applyEgressIPObject(oc, nil, egressIPYamlPath, targetNamespace, egressIPSet, egressUpdateTimeout)

								g.By("Makes sure the EgressIP is advertised by FRR")
								for eip, nodeName := range egressIPSet {
									o.Expect(nodeName).ShouldNot(o.BeEmpty())
									o.Eventually(func() bool {
										return isRouteToEgressIPPresent(oc, eip, testCUDNName, nodeName)
									}).WithTimeout(3 * timeOut).WithPolling(interval).Should(o.BeTrue())
								}

								svcUrl := fmt.Sprintf("%s-0-service:%d", targetNamespace, serverPort)
								g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
								spawnProberSendEgressIPTrafficCheckLogs(oc, targetNamespace, probePodName, svcUrl, targetProtocol, externalIP, serverPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)

								g.By("Updating the EgressIP object")
								ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, targetNamespace, "", newEgressIPSet)

								g.By("Applying the updated EgressIP object")
								applyEgressIPObject(oc, nil, egressIPYamlPath, targetNamespace, newEgressIPSet, egressUpdateTimeout)

								g.By("Makes sure the updated EgressIP is advertised by FRR ")
								for eip, nodeName := range newEgressIPSet {
									o.Expect(nodeName).ShouldNot(o.BeEmpty())
									o.Eventually(func() bool {
										return isRouteToEgressIPPresent(oc, eip, testCUDNName, nodeName)
									}).WithTimeout(3 * timeOut).WithPolling(interval).Should(o.BeTrue())
								}

								g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and updated EgressIPs %v were seen", numberOfRequestsToSend, newEgressIPSet))
								spawnProberSendEgressIPTrafficCheckLogs(oc, targetNamespace, probePodName, svcUrl, targetProtocol, externalIP, serverPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, newEgressIPSet)

								g.By("Deleting the EgressIP object")
								// Use cascading foreground deletion to make sure that the EgressIP object and its dependencies are gone.
								_, err = runOcWithRetry(oc.AsAdmin(), "delete", "egressip", egressIPObjectName, "--cascade=foreground")
								o.Expect(err).NotTo(o.HaveOccurred())

								g.By("Makes sure the EgressIP is not advertised by FRR")
								for eip, nodeName := range newEgressIPSet {
									o.Eventually(func() bool {
										return isRouteToEgressIPPresent(oc, eip, testCUDNName, nodeName)
									}).WithTimeout(3 * timeOut).WithPolling(interval).Should(o.BeFalse())
								}

								g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", 0, newEgressIPSet))
								spawnProberSendEgressIPTrafficCheckLogs(oc, targetNamespace, probePodName, svcUrl, targetProtocol, externalIP, serverPort, numberOfRequestsToSend, 0, packetSnifferDaemonSet, newEgressIPSet)
							}
						},
						g.Entry("When the network is IPv4", IPv4, v4ExternalIP),
						g.Entry("When the network is IPv6", IPv6, v6ExternalIP),
					)
				},
				g.Entry("When the network topology is Layer 3", "layer3"),
				// [TODO] Add test for layer 2 UDN once OCPBUGS-55157 is fixed.
				//g.Entry("When the network topology is Layer 2", "layer2"),
			)
		})
	})
})

func allocateSubnets(oc *exutil.CLI) (string, string, error) {
	subnetsv4, subnetsv6, err := allocateIPs(
		oc,
		allocationConfigMapNamespace,
		allocationConfigMapName,
		subnetAllocationKey,
		subnetAllocationTemplatev4,
		subnetAllocationTemplatev6,
		1, // allocate 1 IPv4 subnet
		1, // allocate 1 IPv6 subnet
		subnetAllocationMin,
		subnetAllocationMax)
	if err != nil {
		return "", "", err
	}
	return subnetsv4[0], subnetsv6[0], nil
}

func deallocateSubnets(oc *exutil.CLI, subnets ...string) error {
	return deallocateIPs(oc, allocationConfigMapNamespace, allocationConfigMapName, subnetAllocationKey, subnets...)
}

func allocateEgressIPSet(oc *exutil.CLI, ipFamily IPFamily, eipNodes []string) (map[string]string, map[string]string, error) {
	egressIPSet := make(map[string]string)
	newEgressIPSet := make(map[string]string)
	n := len(eipNodes)
	var ips []string
	var err error
	switch ipFamily {
	case IPv4:
		ips, _, err = allocateIPs(
			oc,
			allocationConfigMapNamespace,
			allocationConfigMapName,
			eipAllocationKey,
			eipAllocationTemplatev4,
			"",  // no IPv6 allocation
			n*2, // allocate this many IPv4 EIPs
			0,   // no IPv6 allocation
			eipAllocationMin,
			eipAllocationMax,
		)
	case IPv6:
		_, ips, err = allocateIPs(
			oc,
			allocationConfigMapNamespace,
			allocationConfigMapName,
			eipAllocationKey,
			"", // no IPv4 allocation
			eipAllocationTemplatev6,
			0,   // no IPv4 allocation
			n*2, // allocate this many IPv6 EIPs
			eipAllocationMin,
			eipAllocationMax,
		)
	default:
		return nil, nil, fmt.Errorf("unknown IP family")
	}
	if err != nil {
		return nil, nil, err
	}
	for i := 0; i < len(ips); i = i + 2 {
		egressIPSet[ips[i]] = ""
		newEgressIPSet[ips[i+1]] = ""
	}
	return egressIPSet, newEgressIPSet, nil
}

func deallocateEgressIPSets(oc *exutil.CLI, eipsets ...map[string]string) error {
	var eips []string
	for _, set := range eipsets {
		for eip := range set {
			eips = append(eips, eip)
		}
	}
	return deallocateIPs(oc, allocationConfigMapNamespace, allocationConfigMapName, eipAllocationKey, eips...)
}

// isRouteToEgressIPPresent checks that routes to the egress IPs are being advertised by FRR.
func isRouteToEgressIPPresent(oc *exutil.CLI, eip, netName, nodeName string) bool {
	advertised := false
	frr, err := getGeneratedFrrConfigurationForNode(oc, nodeName, netName)
	if err != nil && err.Error() == "FRR configuration for node "+nodeName+" not found" {
		return advertised
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(frr.Spec.BGP.Routers) == 0 {
		return advertised
	}

	// Parse IP to determine if it's IPv4 or IPv6
	ip := net.ParseIP(eip)
	o.Expect(ip).NotTo(o.BeNil())

	var prefix string
	if ip.To4() != nil {
		prefix = fmt.Sprintf("%s/32", eip)
	} else {
		prefix = fmt.Sprintf("%s/128", eip)
	}

	if slices.Contains(frr.Spec.BGP.Routers[0].Prefixes, prefix) {
		advertised = true
	}
	return advertised
}

// getRouteAdvertisements uses the dynamic admin client to return a pointer to
// an existing RouteAdvertisements, or error.
func getRouteAdvertisements(oc *exutil.CLI, name string) (*ratypes.RouteAdvertisements, error) {
	dynamic := oc.AdminDynamicClient()
	unstructured, err := dynamic.Resource(ratypes.SchemeGroupVersion.WithResource("routeadvertisements")).Namespace("").Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	ra := &ratypes.RouteAdvertisements{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), ra)
	if err != nil {
		return nil, err
	}
	return ra, nil
}

// getGeneratedFrrConfigurationForNode returns the FRR configuration for the node
func getGeneratedFrrConfigurationForNode(oc *exutil.CLI, nodeName, raName string) (*frrapi.FRRConfiguration, error) {
	dynamic := oc.AdminDynamicClient()
	unstructuredList, err := dynamic.Resource(frrapi.SchemeGroupVersion.WithResource("frrconfigurations")).Namespace("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", raLabel, raName),
	})
	if err != nil {
		return nil, err
	}
	frrList := &frrapi.FRRConfigurationList{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredList.UnstructuredContent(), frrList)
	if err != nil {
		return nil, err
	}
	for _, frr := range frrList.Items {
		if frr.Spec.NodeSelector.MatchLabels["kubernetes.io/hostname"] == nodeName {
			return &frr, nil
		}
	}
	return nil, fmt.Errorf("FRR configuration for node %s not found", nodeName)
}

func getNodeSubnets(oc *exutil.CLI, network string) (map[string][]net.IPNet, error) {
	// Run the oc command to get node subnets
	out, err := runOcWithRetry(oc.AsAdmin(), "get", "nodes", "-o",
		`jsonpath={range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.k8s\.ovn\.org/node-subnets}{"\n"}{end}`)
	if err != nil {
		return nil, fmt.Errorf("failed to get node subnets: %v", err)
	}

	// Create map to store results
	nodeSubnets := make(map[string][]net.IPNet)

	// Parse each line
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Split line into node name and subnet JSON
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		nodeName := parts[0]
		subnetJSON := parts[1]

		// Parse the JSON subnet data
		var subnetMap map[string][]string
		if err := json.Unmarshal([]byte(subnetJSON), &subnetMap); err != nil {
			return nil, fmt.Errorf("failed to parse subnet JSON for node %s: %v", nodeName, err)
		}

		// Extract subnets for the specified network
		if subnets, ok := subnetMap[network]; ok {
			ipNets := make([]net.IPNet, 0, len(subnets))
			for _, subnet := range subnets {
				_, ipNet, err := net.ParseCIDR(subnet)
				if err != nil {
					return nil, fmt.Errorf("failed to parse CIDR %q for node %s: %v", subnet, nodeName, err)
				}
				ipNets = append(ipNets, *ipNet)
			}
			nodeSubnets[nodeName] = ipNets
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning output: %v", err)
	}

	return nodeSubnets, nil
}

// getLearnedBgpRoutesByNode returns the BGP routes learned by the node
func getLearnedBgpRoutesByNode(oc *exutil.CLI, nodeName, network string) (map[string]string, map[string]string, error) {
	var podName string
	var out string
	var err error
	var v4bgpRoutes, v6bgpRoutes map[string]string

	out, err = runOcWithRetry(oc.AsAdmin(), "get",
		"pods",
		"-o", "name",
		"-n", frrNamespace,
		"--field-selector", fmt.Sprintf("spec.nodeName=%s", nodeName),
		"-l", "app=frr-k8s")
	if err != nil {
		return nil, nil, err
	}
	outReader := bufio.NewScanner(strings.NewReader(out))
	re := regexp.MustCompile("^pod/(.*)")
	for outReader.Scan() {
		match := re.FindSubmatch([]byte(outReader.Text()))
		if len(match) != 2 {
			continue
		}
		podName = string(match[1])
		break
	}
	if podName == "" {
		return nil, nil, fmt.Errorf("could not find valid frr pod on node %q", nodeName)
	}

	isDefault := network == "default"
	cmdv4 := "ip route show proto bgp"
	cmdv6 := "ip -6 route show proto bgp"
	if !isDefault {
		table, err := adminExecInPod(oc, frrNamespace, podName, "frr", "ip -d link show dev "+network+" | grep -oP 'table \\d+' | grep -oP '\\d+'")
		if err != nil {
			return nil, nil, err
		}
		cmdv4 += " table " + table
		cmdv6 += " table " + table
	}

	out, err = adminExecInPod(oc, frrNamespace, podName, "frr", cmdv4)
	if err != nil {
		return nil, nil, err
	}
	framework.Logf("BGP v4 routes for node %s: %s", nodeName, out)
	v4bgpRoutes = parseRoutes(out)

	out, err = adminExecInPod(oc, frrNamespace, podName, "frr", cmdv6)
	if err != nil {
		return nil, nil, err
	}
	framework.Logf("BGP v6 routes for node %s: %s", nodeName, out)
	v6bgpRoutes = parseRoutes(out)

	return v4bgpRoutes, v6bgpRoutes, nil
}

func parseRoutes(routeOutput string) map[string]string {
	routes := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(routeOutput))

	for scanner.Scan() {
		line := scanner.Text()
		// Extract CIDR and via address using regex for both IPv4 and IPv6
		re := regexp.MustCompile(`([\d\.]+/\d+|[a-fA-F0-9:]+/\d+).*via\s+([a-fA-F0-9:.]+)`)
		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			cidr := matches[1] // e.g., "10.128.0.0/23" or "fd01:0:0:1::/64"
			via := matches[2]  // e.g., "192.168.111.22" or "fd2e:6f44:5dd8:c956::16"

			routes[cidr] = via
		}
	}
	return routes
}

func newRouteAdvertisementsManifest(name, vrf string, podNetwork, egressip bool) string {
	advertisements := []string{}
	if podNetwork {
		advertisements = append(advertisements, "PodNetwork")
	}
	if egressip {
		advertisements = append(advertisements, "EgressIP")
	}
	network := "default"
	if vrf == "auto" {
		network = name
	}
	return fmt.Sprintf(`
apiVersion: k8s.ovn.org/v1
kind: RouteAdvertisements
metadata:
  name: %s
spec:
  targetVRF: "%s"
  nodeSelector: {}
  frrConfigurationSelector:
    matchLabels:
      network: %s
  advertisements: [%s]
  networkSelectors:
    - networkSelectionType: ClusterUserDefinedNetworks
      clusterUserDefinedNetworkSelector:
        networkSelector:
          matchLabels:
            network: %s
`, name, vrf, network, strings.Join(advertisements, ","), name)
}

// verifyLearnedBgpRoutesForNode encapsulates the verification of learned BGP routes for a node.
func verifyLearnedBgpRoutesForNode(oc *exutil.CLI, nodeName string, network string) {
	var lastErr error
	nodeSubnets, err := getNodeSubnets(oc, network)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("Checking routes for node %s in network %s", nodeName, network))
	o.Eventually(func() bool {
		bgpV4Routes, bgpV6Routes, err := getLearnedBgpRoutesByNode(oc, nodeName, network)
		if err != nil {
			lastErr = fmt.Errorf("failed to get BGP routes: %v", err)
			return false
		}

		if !verifyExternalRoutes(bgpV4Routes, bgpV6Routes) {
			lastErr = fmt.Errorf("missing external routes")
			return false
		}

		if !verifyNodeSubnetRoutes(nodeName, nodeSubnets, bgpV4Routes, bgpV6Routes) {
			lastErr = fmt.Errorf("missing node subnet routes")
			return false
		}

		return true
	}, 3*timeOut, interval).Should(o.BeTrue(), func() string {
		return fmt.Sprintf("Route verification failed for node %s: %v", nodeName, lastErr)
	})
}

func verifyExternalRoutes(v4Routes, v6Routes map[string]string) bool {
	if _, ok := v4Routes[v4ExternalCIDR]; !ok {
		framework.Logf("Missing v4 external route %s in %v", v4ExternalCIDR, v4Routes)
		return false
	}
	if _, ok := v6Routes[v6ExternalCIDR]; !ok {
		framework.Logf("Missing v6 external route %s in %v", v6ExternalCIDR, v6Routes)
		return false
	}
	return true
}

func verifyNodeSubnetRoutes(nodeName string, nodeSubnets map[string][]net.IPNet, v4Routes, v6Routes map[string]string) bool {
	for node, subnets := range nodeSubnets {
		if node == nodeName {
			continue
		}
		for _, subnet := range subnets {
			if subnet.IP.To4() != nil {
				if _, ok := v4Routes[subnet.String()]; !ok {
					framework.Logf("Missing v4 route for node %s subnet %s", node, subnet.String())
					return false
				}
			} else {
				if _, ok := v6Routes[subnet.String()]; !ok {
					framework.Logf("Missing v6 route for node %s subnet %s", node, subnet.String())
					return false
				}
			}
		}
	}
	return true
}

func checkExternalResponse(oc *exutil.CLI, proberPod *corev1.Pod, podIP, ExternalIP string, serverPort int, packetSnifferDaemonSet *v1.DaemonSet, targetProtocol string) {
	g.By("Sending request from the external host to the PodIPs")
	request := fmt.Sprintf("dial?protocol=http&host=%s&port=%d&request=clientip", podIP, serverPort)
	// Determine URL format based on whether ExternalIP is IPv4 or IPv6.
	ip := net.ParseIP(ExternalIP)
	var url string
	if ip != nil && ip.To4() != nil {
		url = fmt.Sprintf("%s:%d/%s", ExternalIP, serverPort, request)
	} else {
		url = fmt.Sprintf("[%s]:%d/%s", ExternalIP, serverPort, request)
	}

	g.By("Making sure that external host IP presents in the sniffer packet log")
	var lastErr error
	o.Eventually(func() bool {
		output, err := runOcWithRetry(oc.AsAdmin(), "exec", proberPod.Name, "--", "curl", "-m 3", "-s", url)
		if err != nil {
			lastErr = fmt.Errorf("failed to execute curl command: %v", err)
			return false
		}
		framework.Logf("output: %s prober IP: %s", output, ExternalIP)

		g.By("Making sure that external host IP presents in the response")
		var resp response
		if err := json.Unmarshal([]byte(output), &resp); err != nil {
			lastErr = fmt.Errorf("failed to unmarshal response: %v", err)
			return false
		}

		if len(resp.Responses) == 0 {
			lastErr = fmt.Errorf("no responses received")
			return false
		}
		framework.Logf("resp: %s prober IP: %s", resp.Responses[0], ExternalIP)

		if !strings.Contains(resp.Responses[0], ExternalIP) {
			lastErr = fmt.Errorf("response does not contain external IP %s", ExternalIP)
			return false
		}

		found, err := scanPacketSnifferDaemonSetPodLogs(oc, packetSnifferDaemonSet, targetProtocol, ExternalIP)
		if err != nil {
			lastErr = fmt.Errorf("failed to scan packet sniffer logs: %v", err)
			return false
		}

		if len(found) == 0 {
			lastErr = fmt.Errorf("no matching packets found in sniffer logs")
			return false
		}

		return true
	}, 3*timeOut, interval).Should(o.BeTrue(), func() string {
		return fmt.Sprintf("Failed to verify external response: %v", lastErr)
	})
}

// Add these helper functions after the imports
func setupPacketSniffer(oc *exutil.CLI, clientset kubernetes.Interface, snifferNamespace string, advertisedPodsNodes []string, networkPlugin string) (*v1.DaemonSet, error) {
	return setupPacketSnifferOnInterface(oc, clientset, snifferNamespace, advertisedPodsNodes, networkPlugin, "")
}

func setupPacketSnifferOnInterface(oc *exutil.CLI, clientset kubernetes.Interface, snifferNamespace string, advertisedPodsNodes []string, networkPlugin, packetSnifferInterface string) (*v1.DaemonSet, error) {
	// Add SCC privileged
	_, err := runOcWithRetry(oc.AsAdmin(), "adm", "policy", "add-scc-to-user", "privileged",
		fmt.Sprintf("system:serviceaccount:%s:default", snifferNamespace))
	if err != nil {
		return nil, err
	}

	// Find interface for packet sniffing
	if packetSnifferInterface == "" {
		packetSnifferInterface, err = findPacketSnifferInterface(oc, networkPlugin, advertisedPodsNodes)
		if err != nil {
			return nil, err
		}
	}
	framework.Logf("Using interface %s for packet captures", packetSnifferInterface)

	// Create packet sniffer daemonset
	packetSnifferDaemonSet, err := createPacketSnifferDaemonSet(oc, snifferNamespace, advertisedPodsNodes, targetProtocol, serverPort, packetSnifferInterface)
	if err != nil {
		return nil, err
	}

	return packetSnifferDaemonSet, nil
}

func waitForRouteAdvertisements(oc *exutil.CLI, name string) {
	o.Eventually(func() bool {
		ra, err := getRouteAdvertisements(oc, name)
		if err != nil {
			return false
		}
		condition := meta.FindStatusCondition(ra.Status.Conditions, "Accepted")
		if condition == nil {
			return false
		}
		return condition.Status == metav1.ConditionTrue
	}, 3*timeOut, interval).Should(o.BeTrue())
}

func setupTestDeployment(oc *exutil.CLI, clientset kubernetes.Interface, targetNamespace string, advertisedPodsNodes []string) (string, string, *corev1.PodList, error) {
	ingressDomain, err := getIngressDomain(oc)
	if err != nil {
		return "", "", nil, err
	}

	deployName, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, targetNamespace, "",
		ingressDomain, len(advertisedPodsNodes), advertisedPodsNodes)
	if err != nil {
		return "", "", nil, err
	}

	podList, err := clientset.CoreV1().Pods(targetNamespace).List(context.TODO(),
		metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", deployName)})
	if err != nil {
		return "", "", nil, err
	}

	return deployName, routeName, podList, nil
}

func extractPodIPs(podList *corev1.PodList) (map[string]string, map[string]string) {
	v4PodIPSet := make(map[string]string)
	v6PodIPSet := make(map[string]string)

	for _, pod := range podList.Items {
		for _, ip := range pod.Status.PodIPs {
			IP := net.ParseIP(ip.IP)
			if IP == nil {
				continue
			}
			if IP.To4() != nil {
				v4PodIPSet[ip.IP] = pod.Spec.NodeName
			} else {
				v6PodIPSet[ip.IP] = pod.Spec.NodeName
			}
		}
	}
	return v4PodIPSet, v6PodIPSet
}

func extractPodUdnIPs(podList *corev1.PodList, nc *networkAttachmentConfigParams, namespace string, clientset kubernetes.Interface) (map[string]string, map[string]string) {
	var err error
	v4PodIPSet := make(map[string]string)
	v6PodIPSet := make(map[string]string)
	var udnIP string
	for _, pod := range podList.Items {
		for i, cidr := range strings.Split(nc.cidr, ",") {
			if cidr != "" {
				g.By("asserting the pod has an UDN IP from the configured range")
				udnIP, err = podIPsForUserDefinedPrimaryNetwork(
					clientset,
					namespace,
					pod.Name,
					namespacedName(namespace, nc.name),
					i,
				)
				o.Expect(err).NotTo(o.HaveOccurred())

				ip := net.ParseIP(udnIP)
				o.Expect(ip).NotTo(o.BeNil())
				if ip.To4() != nil {
					v4PodIPSet[udnIP] = pod.Spec.NodeName
				} else {
					v6PodIPSet[udnIP] = pod.Spec.NodeName
				}
			}
		}
	}
	return v4PodIPSet, v6PodIPSet
}

// probeForHostnames spawns a prober pod inside the prober namespace. It then runs curl against http://%s/dial?host=%s&port=%d&request=/hostname
// for the specified number of iterations and returns a set of the clientIP addresses that were returned.
// At the end of the test, the prober pod is deleted again.
func probeForHostnames(oc *exutil.CLI, proberPodNamespace, proberPodName, url, targetIP string, targetPort, iterations int, tweak func(*corev1.Pod)) (map[string]struct{}, error) {
	return probeForRequest(oc, proberPodNamespace, proberPodName, url, targetIP, "hostname", targetPort, iterations, tweak)
}

func applyNNCP(oc *exutil.CLI, name, policy string) {
	g.GinkgoHelper()

	o.Expect(oc.AsAdmin().Run("apply").Args("-f", "-").InputString(policy).Execute()).To(o.Succeed())

	o.Eventually(func(g o.Gomega) {
		out, err := oc.AsAdmin().Run("get").Args("NodeNetworkConfigurationPolicy/"+name, "-o", "yaml").Output()
		g.Expect(err).NotTo(o.HaveOccurred())
		matches := nncpRenderedCompiledRegex.FindStringSubmatch(out)
		g.Expect(matches).To(o.HaveLen(3))
		g.Expect(matches[1]).To(o.Equal(matches[2]))
	}).WithTimeout(300 * time.Second).WithPolling(5 * time.Second).Should(o.Succeed())
}

func runCommandInFrrPods(oc *exutil.CLI, command string) (map[string]string, error) {
	results := make(map[string]string)

	// Get all FRR pods
	out, err := runOcWithRetry(oc.AsAdmin(), "get", "pods",
		"-n", frrNamespace,
		"-l", "app=frr-k8s",
		"-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\t\"}{.spec.nodeName}{\"\\n\"}{end}")
	if err != nil {
		return nil, fmt.Errorf("failed to get FRR pods: %v", err)
	}

	// Process each pod
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Split line into pod name and node name
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			continue
		}
		podName := parts[0]
		nodeName := parts[1]

		// Execute command in pod
		output, err := adminExecInPod(oc, frrNamespace, podName, "frr", command)
		if err != nil {
			framework.Logf("Warning: Command failed in pod %s on node %s: %v", podName, nodeName, err)
			continue
		}
		results[nodeName] = output
	}

	return results, nil
}

func gatherDebugInfo(oc *exutil.CLI, snifferDaemonset *v1.DaemonSet, targetNamespace string, workerNodesOrderedNames []string) {
	if out, err := runOcWithRetry(oc.AsAdmin().WithoutNamespace(), "get", "ra", "-o", "yaml"); err == nil {
		framework.Logf("RouteAdvertisements:\n%s", out)
	}
	if out, err := runOcWithRetry(oc.AsAdmin().WithoutNamespace(), "get", "eip", "-o", "yaml"); err == nil {
		framework.Logf("EgressIPs:\n%s", out)
	}
	if out, err := runOcWithRetry(oc.AsAdmin().WithoutNamespace(), "get", "node", "-l", "k8s.ovn.org/egress-assignable="); err == nil {
		framework.Logf("EgressIP assignable nodes:\n%s", out)
	}
	if out, err := runOcWithRetry(oc.AsAdmin().WithoutNamespace(), "get", "clusteruserdefinednetwork", "-o", "yaml"); err == nil {
		framework.Logf("ClusterUserDefinedNetworks:\n%s", out)
	}
	if out, err := runOcWithRetry(oc.AsAdmin().WithoutNamespace(), "get", "pod", "-n", targetNamespace, "-o", "yaml"); err == nil {
		framework.Logf(" %s:\n%s", targetNamespace, out)
	}
	if out, err := runOcWithRetry(oc.AsAdmin().WithoutNamespace(), "get", "frrconfiguration", "-n", "openshift-frr-k8s", "-o", "yaml"); err == nil {
		framework.Logf("FrrConfiguration:\n%s", out)
	}
	if out, err := runOcWithRetry(oc.AsAdmin().WithoutNamespace(), "get", "frrnodestates", "-o", "yaml"); err == nil {
		framework.Logf("FrrNodeStates:\n%s", out)
	}
	if out, err := runOcWithRetry(oc.AsAdmin().WithoutNamespace(), "get", "nncp", "-o", "yaml"); err == nil {
		framework.Logf("FrrNodeStates:\n%s", out)
	}
	if snifferDaemonset != nil {
		if out, err := runOcWithRetry(oc.AsAdmin().WithoutNamespace(), "get", "ds", "-n", snifferDaemonset.Namespace, "-o", "yaml"); err == nil {
			framework.Logf("DaemonSets in namespace %s:\n%s", snifferDaemonset.Namespace, out)
		}
		if out, err := runOcWithRetry(oc.AsAdmin().WithoutNamespace(), "get", "pod", "-n", snifferDaemonset.Namespace, "-o", "yaml"); err == nil {
			framework.Logf("Pods in namespace %s:\n%s", snifferDaemonset.Namespace, out)
		}
		logs, err := getDaemonSetLogs(oc.KubeFramework().ClientSet, snifferDaemonset)
		if err != nil {
			framework.Logf("failed to gather packet sniffer logs: %v", err)
		} else {
			for node, log := range logs {
				framework.Logf("packet sniffer logs for node %s:\n%s", node, log)
			}
		}
	}

	// FRR debugging information
	framework.Logf("\n=== FRR Debugging Information ===")

	if results, err := runCommandInFrrPods(oc, "vtysh -c 'show ip route vrf all'"); err == nil {
		framework.Logf("\nBGP IPv4 route:")
		for node, output := range results {
			framework.Logf("Node %s:\n%s", node, output)
		}
	}

	if results, err := runCommandInFrrPods(oc, "vtysh -c 'show ipv6 route vrf all'"); err == nil {
		framework.Logf("\nBGP IPv6 route:")
		for node, output := range results {
			framework.Logf("Node %s:\n%s", node, output)
		}
	}

	if results, err := runCommandInFrrPods(oc, "vtysh -c 'show ip bgp vrf all ipv4 summary'"); err == nil {
		framework.Logf("\nBGP IPv4 Summary:")
		for node, output := range results {
			framework.Logf("Node %s:\n%s", node, output)
		}
	}

	if results, err := runCommandInFrrPods(oc, "vtysh -c 'show ip bgp vrf all ipv6 summary'"); err == nil {
		framework.Logf("\nBGP IPv6 Summary:")
		for node, output := range results {
			framework.Logf("Node %s:\n%s", node, output)
		}
	}

	if results, err := runCommandInFrrPods(oc, "vtysh -c 'show bgp vrf all ipv4'"); err == nil {
		framework.Logf("\nBGP IPv4 Routes:")
		for node, output := range results {
			framework.Logf("Node %s:\n%s", node, output)
		}
	}

	if results, err := runCommandInFrrPods(oc, "vtysh -c 'show bgp vrf all ipv6'"); err == nil {
		framework.Logf("\nBGP IPv6 Routes:")
		for node, output := range results {
			framework.Logf("Node %s:\n%s", node, output)
		}
	}

	if results, err := runCommandInFrrPods(oc, "vtysh -c 'show bgp vrf all ipv4 neighbor'"); err == nil {
		framework.Logf("\nBGP Neighbors:")
		for node, output := range results {
			framework.Logf("Node %s:\n%s", node, output)
		}
	}

	if results, err := runCommandInFrrPods(oc, "vtysh -c 'show bgp vrf all ipv6 neighbor'"); err == nil {
		framework.Logf("\nBGP Neighbors:")
		for node, output := range results {
			framework.Logf("Node %s:\n%s", node, output)
		}
	}

	if results, err := runCommandInFrrPods(oc, "vtysh -c 'show bfd vrf all peer'"); err == nil {
		framework.Logf("\nBFD Peers:")
		for node, output := range results {
			framework.Logf("Node %s:\n%s", node, output)
		}
	}

	if results, err := runCommandInFrrPods(oc, "vtysh -c 'show running-config'"); err == nil {
		framework.Logf("\nFRR Running Config:")
		for node, output := range results {
			framework.Logf("Node %s:\n%s", node, output)
		}
	}
}

func workaroundOCPBUGS56488(oc *exutil.CLI) (bool, error) {
	opPatch := []byte(`{"spec":{"replicas": 0}}`)
	dp, err := oc.AdminKubeClient().AppsV1().Deployments(nmstateNamespace).Patch(context.Background(), "nmstate-operator", types.MergePatchType, opPatch, metav1.PatchOptions{})
	if err != nil {
		return false, err
	}
	err = exutil.WaitForDeploymentReadyWithTimeout(oc, "nmstate-operator", nmstateNamespace, dp.Generation, 0)
	if err != nil {
		return false, err
	}
	hPatch := []byte(`{"spec":{"template":{"spec":{"containers":[{"name":"nmstate-handler","livenessProbe":{"exec":{"command": ["true"]}}}]}}}}`)
	ds, err := oc.AdminKubeClient().AppsV1().DaemonSets(nmstateNamespace).Patch(context.Background(), "nmstate-handler", types.StrategicMergePatchType, hPatch, metav1.PatchOptions{})
	if err != nil {
		return false, err
	}
	return isDaemonSetRunningOnGeneration(oc, nmstateNamespace, "nmstate-handler", ds.Generation)
}
