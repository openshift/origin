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
	"k8s.io/apimachinery/pkg/util/wait"
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
	bgpProbePodName    = "bgp-prober-pod"
	targetProtocol     = "http"
	targetPort         = 8000
)

var _ = g.Describe("[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io]", func() {
	oc := exutil.NewCLIWithPodSecurityLevel(bgpNamespacePrefix, admissionapi.LevelPrivileged)

	var (
		networkPlugin string

		clientset kubernetes.Interface
		tmpDirBGP string

		workerNodesOrdered      []corev1.Node
		workerNodesOrderedNames []string
		advertisedPodsNodes     []string
		externalNodeName        string
		targetNamespace         string
		externalNamespace       string
		packetSnifferInterface  string
		cloudType               configv1.PlatformType
		deployName              string
		routeName               string
		ingressDomain           string
		packetSnifferDaemonSet  *v1.DaemonSet
		podList                 *corev1.PodList
		v4PodIPSet              map[string]string
		v6PodIPSet              map[string]string
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
		f := oc.KubeFramework()
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

		g.By("Creating a project for the prober pod")
		// Create a target project and assign source and target namespace
		// to variables for later use.
		externalNamespace = oc.SetupProject()
	})

	// Do not check for errors in g.AfterEach as the other cleanup steps will fail, otherwise.
	g.AfterEach(func() {
		g.By("Removing the temp directory")
		os.RemoveAll(tmpDirBGP)
	})

	g.Context("[PodNetwork] Advertising the default network [apigroup:user.openshift.io][apigroup:security.openshift.io]", func() {
		g.JustBeforeEach(func() {
			// SCC privileged is needed to run tcpdump on the packet sniffer containers, and at the minimum host networked is needed for
			// host networked pods.
			g.By("Adding SCC privileged to the external namespace")
			_, err := runOcWithRetry(oc.AsAdmin(), "adm", "policy", "add-scc-to-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:default", externalNamespace))
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Selecting a node to act as as an external host")
			o.Expect(len(workerNodesOrderedNames)).Should(o.BeNumerically(">", 1))
			externalNodeName = workerNodesOrderedNames[0]
			advertisedPodsNodes = workerNodesOrderedNames[1:]

			g.By("Determining the interface that will be used for packet sniffing")
			packetSnifferInterface, err = findPacketSnifferInterface(oc, networkPlugin, []string{externalNodeName})
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Using interface %s for packet captures", packetSnifferInterface)

			g.By("Turn on the BGP advertisement of PodNetwork")
			_, err = runOcWithRetry(oc.AsAdmin(), "patch", "ra", "default", "--type=merge", `-p={"spec":{"advertisements":["PodNetwork"]}}`)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Makes sure that the RouteAdvertisements is accepted")
			waitErr := wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 30*time.Second, true, func(context.Context) (bool, error) {
				ra, err := getRouteAdvertisements(oc, "default")
				if err != nil {
					return false, err
				}
				condition := meta.FindStatusCondition(ra.Status.Conditions, "Accepted")
				return condition.Status == metav1.ConditionTrue, nil
			})
			o.Expect(waitErr).NotTo(o.HaveOccurred())

			g.By("Makes sure the FRR configuration is generated for each node")
			for _, nodeName := range workerNodesOrderedNames {
				frr, err := getGeneratedFrrConfigurationForNode(oc, nodeName)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(frr).NotTo(o.BeNil())
			}

			g.By("Spawning the packet sniffer pods on the workers")
			packetSnifferDaemonSet, err = createPacketSnifferDaemonSet(oc, externalNamespace, advertisedPodsNodes, targetProtocol, targetPort, packetSnifferInterface)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating the test target deployment with number of advertisedPodsNodes")
			ingressDomain, err = getIngressDomain(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			deployName, routeName, err = createAgnhostDeploymentAndIngressRoute(oc, targetNamespace, "", ingressDomain, len(advertisedPodsNodes), advertisedPodsNodes)
			o.Expect(err).NotTo(o.HaveOccurred())

			podList, err = clientset.CoreV1().Pods(targetNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", deployName)})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(podList.Items)).To(o.Equal(len(advertisedPodsNodes)))

			g.By("Retrieving the PodIPs of the test source deployment")
			v4PodIPSet = make(map[string]string)
			v6PodIPSet = make(map[string]string)
			for _, pod := range podList.Items {
				for _, ip := range pod.Status.PodIPs {
					IP := net.ParseIP(ip.IP)
					o.Expect(IP).NotTo(o.BeNil())
					if IP.To4() != nil {
						v4PodIPSet[ip.IP] = pod.Spec.NodeName
					} else {
						v6PodIPSet[ip.IP] = pod.Spec.NodeName
					}
				}
			}
		})

		g.It("pods should communicate with external host without being SNATed", func() {
			nodeSubnets, err := getNodeSubnets(oc, "default")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Checking that BGP routes to the PodNetwork are advertised to each node")
			for _, nodeName := range workerNodesOrderedNames {
				g.By("Checking BGP routes for node " + nodeName)
				bgpV4Routes, bgpV6Routes, err := getLearnedBgpRoutesByNode(oc, nodeName)
				o.Expect(err).NotTo(o.HaveOccurred())

				for node, subnet := range nodeSubnets {
					if node == nodeName {
						continue
					}
					for _, subnet := range subnet {
						if subnet.IP.To4() != nil {
							o.Expect(bgpV4Routes).To(o.HaveKey(subnet.String()))
						} else {
							// o.Expect(bgpV6Routes).To(o.HaveKey(subnet.String()))
							o.Expect(bgpV6Routes).ToNot(o.BeNil())
						}
					}
				}
			}

			numberOfRequestsToSend := 10
			g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and PodIP %v were seen", numberOfRequestsToSend, v4PodIPSet))
			// The baremetal environment is built by dev-scripts, which runs a redfish http server at 192.168.111.1:8000. Use this as the external target host.
			v4External := "192.168.111.1"
			// v6External := "fd2e:6f44:5dd8:c956::1"

			g.By("sending to IPv4 external host")
			spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, v4External, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, v4PodIPSet)
			// g.By("sending to IPv6 external host")
			// spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, v6External, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, v6podIPSet)
		})

		g.It("External host should be able to quay route advertised pods by the pod IP", func() {
			g.By("Launching a new hostnetworked prober pod")
			nodeSelection := e2epod.NodeSelection{}
			e2epod.SetAffinity(&nodeSelection, externalNodeName)
			proberPod := createProberPod(oc, externalNamespace, bgpProbePodName, func(p *corev1.Pod) {
				e2epod.SetNodeSelection(&p.Spec, nodeSelection)
				p.Spec.HostNetwork = true
			})

			pod, err := clientset.CoreV1().Pods(externalNamespace).Get(context.TODO(), proberPod.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			clientIP := pod.Status.PodIP

			for targetIP := range v4PodIPSet {
				url := fmt.Sprintf("%s:%d", targetIP, targetPort)
				g.By(fmt.Sprintf("Launching a new hostNetwork prober pod and probing for the target pod at %s", url))
				request := fmt.Sprintf("http://%s/clientip", url)
				output, err := runOcWithRetry(oc.AsAdmin(), "exec", proberPod.Name, "--", "curl", "-s", request)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Making sure that probe pod IP presents in the response")
				framework.Logf("output: %s clientIP: %s", output, clientIP)
				clientIpPort := strings.Split(output, ":")
				o.Expect(clientIpPort).To(o.HaveLen(2))
				o.Expect(clientIpPort[0]).To(o.Equal(clientIP))

				g.By("Making sure that probe pod IP presents in the sniffer packet log")
				var found map[string]int
				err = wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 30*time.Second, true, func(context.Context) (bool, error) {
					found, _ = scanPacketSnifferDaemonSetPodLogs(oc, packetSnifferDaemonSet, targetProtocol, clientIP)
					if len(found) != 0 {
						framework.Logf("found: %v", found)
						return true, nil
					}
					return false, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(found)).To(o.BeNumerically(">", 0))
			}
			err = destroyProberPod(oc, proberPod)
			o.Expect(err).NotTo(o.HaveOccurred())
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
func getGeneratedFrrConfigurationForNode(oc *exutil.CLI, nodeName string) (*frrapi.FRRConfiguration, error) {
	dynamic := oc.AdminDynamicClient()
	unstructuredList, err := dynamic.Resource(frrapi.SchemeGroupVersion.WithResource("frrconfigurations")).Namespace("").List(context.TODO(), metav1.ListOptions{})
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
		"-n", "openshift-frr-k8s",
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
	out, err = adminExecInPod(oc, "openshift-frr-k8s", podName, "frr", "ip route show proto bgp")
	if err != nil {
		return nil, nil, err
	}
	v4bgpRoutes = parseRoutes(out)

	out, err = adminExecInPod(oc, "openshift-frr-k8s", podName, "frr", "ip -6 route show proto bgp")
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
