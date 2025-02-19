package networking

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
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

	userDefinedNetworkIPv4Subnet = "203.203.0.0/16"
	userDefinedNetworkIPv6Subnet = "2014:100:200::0/60"
	cudnName                     = "udn1"
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
			externalNodeName        string
			targetNamespace         string
			snifferNamespace        string
			cloudType               configv1.PlatformType
			deployName              string
			routeName               string
			packetSnifferDaemonSet  *v1.DaemonSet
			podList                 *corev1.PodList
			v4PodIPSet              map[string]string
			v6PodIPSet              map[string]string
			clusterIPFamily         IPFamily
		)

		g.BeforeEach(func() {
			g.By("Verifying that this cluster uses a network plugin that is supported for this test")
			networkPlugin = networkPluginName()
			if networkPlugin != OVNKubernetesPluginName &&
				networkPlugin != OpenshiftSDNPluginName {
				skipper.Skipf("This cluster neither uses OVNKubernetes nor OpenShiftSDN")
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

			g.By("Creating a project for the prober pod")
			// Create a target project and assign source and target namespace
			// to variables for later use.
			snifferNamespace = oc.SetupProject()

			clusterIPFamily = getIPFamilyForCluster(f)
		})

		// Do not check for errors in g.AfterEach as the other cleanup steps will fail, otherwise.
		g.AfterEach(func() {
			g.By("Removing the temp directory")
			os.RemoveAll(tmpDirBGP)
		})

		g.Context("[PodNetwork] Advertising the default network [apigroup:user.openshift.io][apigroup:security.openshift.io]", func() {
			g.JustBeforeEach(func() {
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
				deployName, routeName, podList, err = setupTestDeployment(oc, clientset, targetNamespace, advertisedPodsNodes)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(podList.Items)).To(o.Equal(len(advertisedPodsNodes)))

				g.By("Extract test pod IPs")
				v4PodIPSet, v6PodIPSet = extractPodIPs(podList)
			})

			g.It("pods should communicate with external host without being SNATed", func() {
				g.By("Checking that routes are advertised to each node")
				for _, nodeName := range workerNodesOrderedNames {
					verifyLearnedBgpRoutesForNode(oc, nodeName, "default")
				}

				numberOfRequestsToSend := 10
				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and PodIP %v were seen", numberOfRequestsToSend, v4PodIPSet))

				if clusterIPFamily == DualStack || clusterIPFamily == IPv4 {
					g.By("sending to IPv4 external host")
					spawnProberSendEgressIPTrafficCheckLogs(oc, snifferNamespace, probePodName, routeName, targetProtocol, v4ExternalIP, serverPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, v4PodIPSet)
				}
				if clusterIPFamily == DualStack || clusterIPFamily == IPv6 {
					// [TODO] enable IPv6 test once OCPBUGS-52194 is fixed
					// g.By("sending to IPv6 external host")
					// spawnProberSendEgressIPTrafficCheckLogs(oc, snifferNamespace, probePodName, routeName, targetProtocol, v6ExternalIP, serverPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, v6PodIPSet)
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
					// [TODO] enable IPv6 test once OCPBUGS-52194 is fixed
					// g.By("checking the external host to pod traffic works for IPv6")
					// for podIP := range v6PodIPSet {
					// 	checkExternalResponse(oc, proberPod, podIP, v6ExternalIP, serverPort, packetSnifferDaemonSet, targetProtocol)
					// }
				}
			})
		})

		g.Context("[PodNetwork] Advertising a cluster user defined network [Serial][apigroup:user.openshift.io][apigroup:security.openshift.io]", func() {
			g.BeforeEach(func() {
				var err error
				g.By("Setup packet sniffer at nodes")
				packetSnifferDaemonSet, err = setupPacketSniffer(oc, clientset, snifferNamespace, advertisedPodsNodes, networkPlugin)
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
					name:      cudnName,
					topology:  "layer3",
					role:      "primary",
					namespace: targetNamespace,
				}
				nc.cidr = correctCIDRFamily(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet)
				cudnManifest := generateClusterUserDefinedNetworkManifest(nc)
				cleanup, err := createManifest(targetNamespace, cudnManifest)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Eventually(clusterUserDefinedNetworkReadyFunc(oc.AdminDynamicClient(), cudnName), 60*time.Second, time.Second).Should(o.Succeed())

				g.By("Labeling the UDN for advertisement")
				_, err = runOcWithRetry(oc.AsAdmin(), "label", "clusteruserdefinednetworks", "-n", targetNamespace, cudnName, "advertise=true")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Create the route advertisement for UDN")
				raManifest := newRouteAdvertisementsManifest(cudnName, true, false)
				err = applyManifest(targetNamespace, raManifest)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Ensure the RouteAdvertisements is accepted")
				waitForRouteAdvertisements(oc, cudnName)

				g.By("Makes sure the FRR configuration is generated for each node")
				for _, nodeName := range workerNodesOrderedNames {
					frr, err := getGeneratedFrrConfigurationForNode(oc, nodeName, cudnName)
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(frr).NotTo(o.BeNil())
				}

				g.By("Deploy the test pods")
				deployName, routeName, podList, err = setupTestDeployment(oc, clientset, targetNamespace, advertisedPodsNodes)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(podList.Items)).To(o.Equal(len(advertisedPodsNodes)))

				g.By("Extract test pod UDN IPs")
				v4PodIPSet, v6PodIPSet = extractPodUdnIPs(podList, nc, targetNamespace, clientset)

				g.DeferCleanup(func() {
					runOcWithRetry(oc.AsAdmin(), "delete", "deploy", deployName)
					runOcWithRetry(oc.AsAdmin(), "delete", "pod", "--all")
					runOcWithRetry(oc.AsAdmin(), "delete", "ra", cudnName)
					runOcWithRetry(oc.AsAdmin(), "delete", "clusteruserdefinednetwork", cudnName)
					cleanup()
				})
			})

			g.AfterEach(func() {
			})

			g.It("pods should communicate with external host without being SNATed", func() {
				g.By("Checking that BGP routes to the PodNetwork are learned by other nodes")
				g.By("Checking that routes are advertised to each node")
				for _, nodeName := range workerNodesOrderedNames {
					verifyLearnedBgpRoutesForNode(oc, nodeName, cudnName)
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
			})

			g.It("External host should be able to query route advertised pods by the pod IP", func() {
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
			})
		})
	})
})

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
func getLearnedBgpRoutesByNode(oc *exutil.CLI, nodeName string) (map[string]string, map[string]string, error) {
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
	out, err = adminExecInPod(oc, frrNamespace, podName, "frr", "ip route show proto bgp")
	if err != nil {
		return nil, nil, err
	}
	v4bgpRoutes = parseRoutes(out)

	out, err = adminExecInPod(oc, frrNamespace, podName, "frr", "ip -6 route show proto bgp")
	if err != nil {
		return nil, nil, err
	}
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

func newRouteAdvertisementsManifest(name string, podNetwork, egressip bool) string {
	advertisements := []string{}
	if podNetwork {
		advertisements = append(advertisements, "PodNetwork")
	}
	if egressip {
		advertisements = append(advertisements, "EgressIP")
	}
	return fmt.Sprintf(`
apiVersion: k8s.ovn.org/v1
kind: RouteAdvertisements
metadata:
  name: %s
spec:
  advertisements: [%s]
  networkSelector:
    matchLabels:
      advertise: "true"
`, name, strings.Join(advertisements, ","))
}

// verifyLearnedBgpRoutesForNode encapsulates the verification of learned BGP routes for a node.
func verifyLearnedBgpRoutesForNode(oc *exutil.CLI, nodeName string, network string) {
	var lastErr error
	nodeSubnets, err := getNodeSubnets(oc, network)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("Checking routes for node %s in network %s", nodeName, network))
	o.Eventually(func() bool {
		bgpV4Routes, bgpV6Routes, err := getLearnedBgpRoutesByNode(oc, nodeName)
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
	}, timeOut, interval).Should(o.BeTrue(), func() string {
		return fmt.Sprintf("Route verification failed for node %s: %v", nodeName, lastErr)
	})
}

func verifyExternalRoutes(v4Routes, v6Routes map[string]string) bool {
	if _, ok := v4Routes[v4ExternalCIDR]; !ok {
		framework.Logf("Missing v4 external route %s", v4ExternalCIDR)
		return false
	}
	if _, ok := v6Routes[v6ExternalCIDR]; !ok {
		framework.Logf("Missing v6 external route %s", v6ExternalCIDR)
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
					// [TODO] enable IPv6 test once OCPBUGS-52194 is fixed
					// return false
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
		output, err := runOcWithRetry(oc.AsAdmin(), "exec", proberPod.Name, "--", "curl", "-s", url)
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
	}, timeOut, interval).Should(o.BeTrue(), func() string {
		return fmt.Sprintf("Failed to verify external response: %v", lastErr)
	})
}

// Add these helper functions after the imports

func setupPacketSniffer(oc *exutil.CLI, clientset kubernetes.Interface, snifferNamespace string, advertisedPodsNodes []string, networkPlugin string) (*v1.DaemonSet, error) {
	// Add SCC privileged
	_, err := runOcWithRetry(oc.AsAdmin(), "adm", "policy", "add-scc-to-user", "privileged",
		fmt.Sprintf("system:serviceaccount:%s:default", snifferNamespace))
	if err != nil {
		return nil, err
	}

	// Find interface for packet sniffing
	packetSnifferInterface, err := findPacketSnifferInterface(oc, networkPlugin, advertisedPodsNodes)
	if err != nil {
		return nil, err
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
	g.By("Extract test pod UDN IPs")
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
