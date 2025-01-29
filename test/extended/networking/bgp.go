package networking

import (
	"context"
	"fmt"
	"os"
	"strconv"
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
	"k8s.io/apimachinery/pkg/labels"
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
	clientPort         = 40000
)

var _ = g.Describe("[sig-network][Feature:RouteAdvertisements][apigroup:operator.openshift.io]", func() {
	oc := exutil.NewCLIWithPodSecurityLevel(bgpNamespacePrefix, admissionapi.LevelPrivileged)
	portAllocator := NewPortAllocator(egressIPTargetHostPortMin, egressIPTargetHostPortMax)

	var (
		networkPlugin string

		clientset kubernetes.Interface
		tmpDirBGP string

		workerNodesOrdered      []corev1.Node
		workerNodesOrderedNames []string
		adervisedPodsNodes      []string
		egressIPNodes           []string
		externalNodeName        string

		egressIPNamespace      string
		externalNamespace      string
		packetSnifferInterface string

		cloudType configv1.PlatformType
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
		egressIPNamespace = f.Namespace.Name

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

	g.Context("[PodNetwork][apigroup:user.openshift.io][apigroup:security.openshift.io]", func() {
		g.JustBeforeEach(func() {
			// SCC privileged is needed to run tcpdump on the packet sniffer containers, and at the minimum host networked is needed for
			// host networked pods.
			g.By("Adding SCC privileged to the external namespace")
			_, err := runOcWithRetry(oc.AsAdmin(), "adm", "policy", "add-scc-to-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:default", externalNamespace))
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Selecting a node to act as as an external host")
			o.Expect(len(workerNodesOrderedNames)).Should(o.BeNumerically(">", 1))
			externalNodeName = workerNodesOrderedNames[0]
			adervisedPodsNodes = workerNodesOrderedNames[1:]

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
		})

		g.It("Route advertised pods should query external host with its pod IP", func() {
			var targetIP string
			var targetPort int

			g.By("Spawning the packet sniffer pods on the workers")
			packetSnifferDaemonSet, err := createPacketSnifferDaemonSet(oc, externalNamespace, []string{externalNodeName}, targetProtocol, clientPort, packetSnifferInterface)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating the target DaemonSet with a single hostnetworked pod on the node which mimics an external host")
			daemonSetName := "hostnetworked"
			// Try the entire port range to create the DaemonSet.
			for i := 0; i < egressIPTargetHostPortMax-egressIPTargetHostPortMin; i++ {
				containerPort, err := portAllocator.AllocateNextPort()
				o.Expect(err).NotTo(o.HaveOccurred())

				// use the port that we got from the port allocator for this
				// new DS / pod. Store the created daemonset for later.
				_, err = createHostNetworkedDaemonSetAndProbe(
					clientset,
					externalNamespace,
					externalNodeName,
					daemonSetName,
					containerPort,
					10, // every 10 seconds
					6,  // for 6 retries
				)

				// If this is a port conflict, then keep the port allocation and
				// simply continue (but delete the current DS first).
				// The current port is hence marked as unavailable for
				// further tries.
				if err != nil && strings.Contains(err.Error(), "Port conflict when creating pod") {
					err := deleteDaemonSet(clientset, externalNamespace, daemonSetName)
					o.Expect(err).NotTo(o.HaveOccurred())
					continue
				}
				// Any other error shoud not have occurred.
				o.Expect(err).NotTo(o.HaveOccurred())

				// Break if no error was found.
				targetPort = containerPort
				break
			}

			g.By("Getting the targetIP for the test from the DaemonSet pod")
			label := labels.SelectorFromSet(labels.Set(map[string]string{"app": daemonSetName}))
			_, err = e2epod.WaitForPodsWithLabelRunningReady(context.TODO(), clientset, externalNamespace, label, 1, framework.PodStartTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			podIPs, err := getDaemonSetPodIPs(clientset, externalNamespace, daemonSetName)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(podIPs)).Should(o.BeNumerically(">", 0))
			targetIP = podIPs[0]

			url := fmt.Sprintf("%s:%d", targetIP, targetPort)
			for _, nodeName := range adervisedPodsNodes {
				g.By(fmt.Sprintf("Launching a new prober pod and probing for the hostNetwork pod at %s", url))
				framework.Logf("Launching a new prober pod")
				nodeSelection := e2epod.NodeSelection{}
				e2epod.SetAffinity(&nodeSelection, nodeName)
				proberPod := createProberPod(oc, externalNamespace, bgpProbePodName, func(p *corev1.Pod) { e2epod.SetNodeSelection(&p.Spec, nodeSelection) })

				pod, err := clientset.CoreV1().Pods(externalNamespace).Get(context.TODO(), proberPod.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				clientIP := pod.Status.PodIP

				g.By("Making sure that probe pod IP presents in the response")
				request := fmt.Sprintf("http://%s/clientip", url)
				output, err := runOcWithRetry(oc.AsAdmin(), "exec", proberPod.Name, "--", "curl", "--local-port", strconv.Itoa(clientPort), "-s", request)
				o.Expect(err).NotTo(o.HaveOccurred())
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
				o.Expect(found).To(o.HaveKey(clientIP))

				err = destroyProberPod(oc, proberPod)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})
		g.It("External host should be able to quay route advertised pods by the pod IP", func() {
			g.By("Selecting a node to deploy the target DaemonSet")
			// Requires a minimum of 3 worker nodes in total:
			// 1 external node + at least 2 as target of advertised pods traffic.
			o.Expect(len(workerNodesOrderedNames)).Should(o.BeNumerically(">", 1))
			externalNodeName := workerNodesOrderedNames[0]
			adervisedPodsNodes := workerNodesOrderedNames[1:]

			g.By("Creating the test target deployment with route advertised pods")
			deployName, _, err := createAgnhostDeploymentAndIngressRoute(oc, externalNamespace, "", "fake-ingress-domain", len(adervisedPodsNodes), adervisedPodsNodes)
			o.Expect(err).NotTo(o.HaveOccurred())
			targetPort := 8000

			g.By("Getting the targetIP for the test from the deploy pod")
			label := labels.SelectorFromSet(labels.Set(map[string]string{"app": deployName}))
			_, err = e2epod.WaitForPodsWithLabelRunningReady(context.TODO(), clientset, externalNamespace, label, 1, framework.PodStartTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			podIPs, err := getDeploymentPodIPs(clientset, externalNamespace, deployName)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(podIPs)).Should(o.BeNumerically(">", 0))

			g.By("Spawning the packet sniffer pods on the workers")
			packetSnifferDaemonSet, err := createPacketSnifferDaemonSet(oc, externalNamespace, []string{externalNodeName}, targetProtocol, targetPort, packetSnifferInterface)
			o.Expect(err).NotTo(o.HaveOccurred())

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

			for _, targetIP := range podIPs {
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

	g.Context("[EgressIP][apigroup:user.openshift.io][apigroup:security.openshift.io]", func() {
		g.JustBeforeEach(func() {
			// SCC privileged is needed to run tcpdump on the packet sniffer containers, and at the minimum host networked is needed for
			// host networked pods.
			g.By("Adding SCC privileged to the external namespace")
			_, err := runOcWithRetry(oc.AsAdmin(), "adm", "policy", "add-scc-to-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:default", externalNamespace))
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Selecting a node to act as as an external host")
			o.Expect(len(workerNodesOrderedNames)).Should(o.BeNumerically(">", 1))
			externalNodeName = workerNodesOrderedNames[0]
			egressIPNodes = workerNodesOrderedNames[1:]

			g.By("Determining the interface that will be used for packet sniffing")
			packetSnifferInterface, err = findPacketSnifferInterface(oc, networkPlugin, []string{externalNodeName})
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Using interface %s for packet captures", packetSnifferInterface)

			g.By("Setting the EgressIP nodes as EgressIP assignable")
			for _, node := range egressIPNodes {
				_, err = runOcWithRetry(oc.AsAdmin(), "label", "node", node, "k8s.ovn.org/egress-assignable=")
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("Turn on the BGP advertisement of EgressIPs")
			_, err = runOcWithRetry(oc.AsAdmin(), "patch", "ra", "default", "--type=merge", `-p={"spec":{"advertisements":["EgressIP","PodNetwork"]}}`)
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
		})

		g.AfterEach(func() {
			g.By("Deleting the EgressIP object if it exists for OVN Kubernetes")
			egressIPYamlPath := tmpDirBGP + "/" + egressIPYaml
			if _, err := os.Stat(egressIPYamlPath); err == nil {
				_, _ = runOcWithRetry(oc.AsAdmin(), "delete", "-f", tmpDirBGP+"/"+egressIPYaml)
			}

			g.By("Removing the EgressIP assignable annotation for OVN Kubernetes")
			for _, nodeName := range egressIPNodes {
				_, _ = runOcWithRetry(oc.AsAdmin(), "label", "node", nodeName, "k8s.ovn.org/egress-assignable-")
			}

			g.By("Turn off the BGP advertisement of EgressIPs")
			_, err := runOcWithRetry(oc.AsAdmin(), "patch", "ra", "default", "--type=merge", `-p={"spec":{"advertisements":["PodNetwork"]}}`)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("pods should have the assigned EgressIPs and EgressIPs can be deleted and recreated [apigroup:route.openshift.io]", func() {
			g.By("Spawning the packet sniffer pods on the workers")
			targetPort := 8000
			packetSnifferDaemonSet, err := createPacketSnifferDaemonSet(oc, externalNamespace, egressIPNodes, targetProtocol, targetPort, packetSnifferInterface)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating the EgressIP test source deployment with number of pods equals number of EgressIP nodes")
			ingressDomain, err := getIngressDomain(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			_, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "", ingressDomain, len(egressIPNodes), egressIPNodes)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Choosing the EgressIPs to be assigned, one per node")
			egressIPSet := make(map[string]string)
			// With BGP, we can assign arbitrary EgressIPs to nodes.
			for i := 0; i < len(egressIPNodes); i++ {
				eip := fmt.Sprintf("1.2.3.%d", i+1)
				egressIPSet[eip] = egressIPNodes[i]
			}

			numberOfRequestsToSend := 10
			// The baremetal environment is built by dev-scripts, which runs a redfish http server at 192.168.111.1:8000. Use this as the external target host.
			targetHost := "192.168.111.1"
			egressIPYamlPath := tmpDirBGP + "/" + egressIPYaml
			egressIPObjectName := egressIPNamespace
			// Run this twice to make sure that repeated EgressIP creation and deletion works.
			for i := 0; i < 2; i++ {
				g.By("Creating the EgressIP object")
				ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

				g.By("Applying the EgressIP object")
				applyEgressIPObject(oc, nil, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)

				g.By("Makes sure the EgressIP is advertised by FRR")
				for eip, nodeName := range egressIPSet {
					o.Eventually(func() bool {
						frr, err := getGeneratedFrrConfigurationForNode(oc, nodeName)
						if err != nil {
							return false
						}
						for _, prefix := range frr.Spec.BGP.Routers[0].Prefixes {
							if prefix == fmt.Sprintf("%s/32", eip) {
								return true
							}
						}
						return false
					}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).Should(o.BeTrue())
				}

				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
				spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)

				g.By("Deleting the EgressIP object")
				// Use cascading foreground deletion to make sure that the EgressIP object and its dependencies are gone.
				_, err = runOcWithRetry(oc.AsAdmin(), "delete", "egressip", egressIPObjectName, "--cascade=foreground")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Makes sure the EgressIP is not advertised by FRR")
				for eip, nodeName := range egressIPSet {
					o.Eventually(func() bool {
						frr, err := getGeneratedFrrConfigurationForNode(oc, nodeName)
						if err != nil {
							return true
						}
						for _, prefix := range frr.Spec.BGP.Routers[0].Prefixes {
							if prefix == fmt.Sprintf("%s/32", eip) {
								return true
							}
						}
						return false
					}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).Should(o.BeFalse())
				}

				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", 0, egressIPSet))
				spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, 0, packetSnifferDaemonSet, egressIPSet)
			}
		})

		g.It("pods should keep the assigned EgressIPs when being rescheduled to another node", func() {
			g.By("Selecting a single EgressIP node, and a single start node for the pod")
			// requires a total of 3 worker nodes
			o.Expect(len(egressIPNodes)).Should(o.BeNumerically(">", 1))
			leftNode := egressIPNodes[0:1]
			rightNode := egressIPNodes[1:2]

			g.By("Spawning the packet sniffer pods on the workers")
			targetPort := 8000
			packetSnifferDaemonSet, err := createPacketSnifferDaemonSet(oc, externalNamespace, egressIPNodes, targetProtocol, targetPort, packetSnifferInterface)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Creating the EgressIP test source deployment on node %s", rightNode[0]))
			ingressDomain, err := getIngressDomain(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			deploymentName, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "", ingressDomain, len(rightNode), rightNode)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Choosing the EgressIPs to be assigned, one per node")
			egressIPSet := make(map[string]string)
			// With BGP, we can assign arbitrary EgressIPs to nodes.
			for i := 0; i < len(egressIPNodes); i++ {
				eip := fmt.Sprintf("1.2.3.%d", i+1)
				egressIPSet[eip] = egressIPNodes[i]
			}

			g.By("Creating the EgressIP object for OVN Kubernetes")
			egressIPYamlPath := tmpDirBGP + "/" + egressIPYaml
			egressIPObjectName := egressIPNamespace
			ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

			g.By("Applying the EgressIP object for OVN Kubernetes")
			applyEgressIPObject(oc, nil, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)

			g.By("Makes sure the EgressIP is advertised by FRR")
			for eip, nodeName := range egressIPSet {
				o.Eventually(func() bool {
					frr, err := getGeneratedFrrConfigurationForNode(oc, nodeName)
					if err != nil {
						return false
					}
					for _, prefix := range frr.Spec.BGP.Routers[0].Prefixes {
						if prefix == fmt.Sprintf("%s/32", eip) {
							return true
						}
					}
					return false
				}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).Should(o.BeTrue())
			}

			numberOfRequestsToSend := 10
			targetHost := "192.168.111.1"
			g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
			spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)

			g.By("Updating the source deployment's Affinity and moving it to the other source node")
			err = updateDeploymentAffinity(oc, egressIPNamespace, deploymentName, leftNode)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
			spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)
		})
	})
})

// getDeploymentSetPodIPs returns the IPs of all pods in the Deployment.
func getDeploymentPodIPs(clientset kubernetes.Interface, namespace, deployName string) ([]string, error) {
	var dp *v1.Deployment
	var podIPs []string
	// Get the DS
	dp, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deployName, metav1.GetOptions{})
	if err != nil {
		return []string{}, err
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(
		context.TODO(),
		metav1.ListOptions{LabelSelector: labels.Set(dp.Spec.Selector.MatchLabels).String()})
	if err != nil {
		return []string{}, err
	}
	for _, pod := range pods.Items {
		podIPs = append(podIPs, pod.Status.PodIP)
	}

	return podIPs, nil
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
