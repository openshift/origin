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

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

var _ = g.Describe("[sig-network][Feature:BGP][apigroup:operator.openshift.io]", func() {
	oc := exutil.NewCLIWithPodSecurityLevel(bgpNamespacePrefix, admissionapi.LevelPrivileged)
	portAllocator := NewPortAllocator(egressIPTargetHostPortMin, egressIPTargetHostPortMax)

	var (
		networkPlugin string

		clientset kubernetes.Interface
		tmpDirBGP string

		workerNodesOrdered      []corev1.Node
		workerNodesOrderedNames []string
		adervisedPodsNodes      []string
		externalNodeName        string

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

	g.Context("[external-targets][apigroup:user.openshift.io][apigroup:security.openshift.io]", func() {
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

			nodeSelection := e2epod.NodeSelection{}
			e2epod.SetAffinity(&nodeSelection, externalNodeName)
			for _, targetIP := range podIPs {
				url := fmt.Sprintf("%s:%d", targetIP, targetPort)
				g.By(fmt.Sprintf("Launching a new hostNetwork prober pod and probing for the target pod at %s", url))
				framework.Logf("Launching a new hostnetworked prober pod")
				proberPod := createProberPod(oc, externalNamespace, bgpProbePodName, func(p *corev1.Pod) {
					e2epod.SetNodeSelection(&p.Spec, nodeSelection)
					p.Spec.HostNetwork = true
				})

				pod, err := clientset.CoreV1().Pods(externalNamespace).Get(context.TODO(), proberPod.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				clientIP := pod.Status.PodIP

				g.By("Making sure that probe pod IP presents in the response")
				request := fmt.Sprintf("http://%s/clientip", url)
				output, err := runOcWithRetry(oc.AsAdmin(), "exec", proberPod.Name, "--", "curl", "-s", request)
				o.Expect(err).NotTo(o.HaveOccurred())
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

				err = destroyProberPod(oc, proberPod)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
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
