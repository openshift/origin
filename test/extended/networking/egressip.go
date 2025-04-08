package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"
	cloudnetwork "github.com/openshift/client-go/cloudnetwork/clientset/versioned"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	// for all tests
	namespacePrefix = "egressip"
	egressIPYaml    = "egressip.yaml"
	probePodName    = "prober-pod"

	// for tests against host networked pods
	egressIPTargetHostPortMin = 32667
	egressIPTargetHostPortMax = 32767

	// Max time that we wait for changes to EgressIP objects
	// to propagate to the CloudPrivateIPConfig objects.
	// This can take a significant amount of time on Azure.
	// BZ https://bugzilla.redhat.com/show_bug.cgi?id=2073045
	egressUpdateTimeout = 180
)

var _ = g.Describe("[sig-network][Feature:EgressIP][apigroup:operator.openshift.io]", func() {
	oc := exutil.NewCLIWithPodSecurityLevel(namespacePrefix, admissionapi.LevelPrivileged)
	portAllocator := NewPortAllocator(egressIPTargetHostPortMin, egressIPTargetHostPortMax)

	var (
		networkPlugin string

		clientset             kubernetes.Interface
		cloudNetworkClientset cloudnetwork.Interface
		tmpDirEgressIP        string

		workerNodesOrdered        []corev1.Node
		workerNodesOrderedNames   []string
		egressIPNodesOrderedNames []string
		nonEgressIPNodeName       string

		egressIPNamespace      string
		externalNamespace      string
		packetSnifferDaemonSet *v1.DaemonSet
		packetSnifferInterface string

		ingressDomain string

		cloudType configv1.PlatformType
		hasIPv4   bool
		hasIPv6   bool

		targetProtocol string
		targetHost     string
		targetPort     int
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
		tmpDirEgressIP, err = ioutil.TempDir("", "egressip-e2e")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Getting the kubernetes clientset")
		f := oc.KubeFramework()
		clientset = f.ClientSet

		g.By("Getting the cloudnetwork clientset")
		cloudNetworkClientset, err = cloudnetwork.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Determining the cloud infrastructure type")
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		cloudType = infra.Spec.PlatformSpec.Type

		g.By("Verifying that this is a supported cloud infrastructure platform")
		isSupportedPlatform := false
		supportedPlatforms := []configv1.PlatformType{
			configv1.AWSPlatformType,
			configv1.GCPPlatformType,
			configv1.AzurePlatformType,
			configv1.OpenStackPlatformType,
		}
		for _, supportedPlatform := range supportedPlatforms {
			if cloudType == supportedPlatform {
				isSupportedPlatform = true
				break
			}
		}
		if !isSupportedPlatform {
			skipper.Skipf("This cloud platform (%s) is not supported for this test", cloudType)
		}

		// A supported version of OpenShift must hold the CloudPrivateIPConfig CRD.
		// Otherwise, skip this test.
		g.By("Verifying that this is a supported version of OpenShift")
		isSupportedOcpVersion, err := exutil.DoesApiResourceExist(oc.AdminConfig(), "cloudprivateipconfigs", "cloud.network.openshift.io")
		o.Expect(err).NotTo(o.HaveOccurred())
		if !isSupportedOcpVersion {
			skipper.Skipf("This OCP version is not supported for this test (api-resource cloudprivateipconfigs not found)")
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

		g.By("Determining the cloud address families")
		hasIPv4, hasIPv6, err = GetIPAddressFamily(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Determining the target protocol, host and port")
		targetProtocol, targetHost, targetPort, err = getTargetProtocolHostPort(oc, hasIPv4, hasIPv6, cloudType, networkPlugin)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Testing against: CloudType: %s, NetworkPlugin: %s, Protocol %s, TargetHost: %s, TargetPort: %d",
			cloudType,
			networkPlugin,
			targetProtocol,
			targetHost,
			targetPort)

		g.By("Creating a project for the prober pod")
		// Create a target project and assign source and target namespace
		// to variables for later use.
		egressIPNamespace = f.Namespace.Name
		externalNamespace = oc.SetupProject()

		g.By("Selecting the EgressIP nodes and a non-EgressIP node")
		nonEgressIPNodeName = workerNodesOrderedNames[0]
		egressIPNodesOrderedNames = workerNodesOrderedNames[1:]

		g.By("Setting the ingressdomain")
		ingressDomain, err = getIngressDomain(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		if networkPluginName() == OVNKubernetesPluginName {
			g.By("Setting the EgressIP nodes as EgressIP assignable")
			for _, node := range egressIPNodesOrderedNames {
				_, err = runOcWithRetry(oc.AsAdmin(), "label", "node", node, "k8s.ovn.org/egress-assignable=")
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})

	// Do not check for errors in g.AfterEach as the other cleanup steps will fail, otherwise.
	g.AfterEach(func() {
		if networkPluginName() == OVNKubernetesPluginName {
			g.By("Deleting the EgressIP object if it exists for OVN Kubernetes")
			egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
			if _, err := os.Stat(egressIPYamlPath); err == nil {
				_, _ = runOcWithRetry(oc.AsAdmin(), "delete", "-f", tmpDirEgressIP+"/"+egressIPYaml)
			}

			g.By("Removing the EgressIP assignable annotation for OVN Kubernetes")
			for _, nodeName := range egressIPNodesOrderedNames {
				_, _ = runOcWithRetry(oc.AsAdmin(), "label", "node", nodeName, "k8s.ovn.org/egress-assignable-")
			}
		} else {
			g.By("Removing any hostsubnet EgressIPs for OpenShiftSDN")
			for _, nodeName := range egressIPNodesOrderedNames {
				_ = sdnHostsubnetFlushEgressIPs(oc, nodeName)
				_ = sdnHostsubnetFlushEgressCIDRs(oc, nodeName)
			}
		}

		g.By("Removing the temp directory")
		os.RemoveAll(tmpDirEgressIP)
	})

	g.Context("[internal-targets]", func() {
		g.JustBeforeEach(func() {
			// Host networked is needed for host networked pods.
			g.By("Adding SCC hostnetwork to the external namespace")
			_, err := runOcWithRetry(oc.AsAdmin(), "adm", "policy", "add-scc-to-user", "hostnetwork", fmt.Sprintf("system:serviceaccount:%s:default", externalNamespace))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("EgressIP pods should query hostNetwork pods with the local node's SNAT", func() {
			var targetIP string
			var targetPort int

			g.By("Selecting a single EgressIP node, and one node per source deployment")
			// Requires a minimum of 3 worker nodes in total:
			// 1 nonEgressIPNodeName + at least 2 as sources of EgressIP traffic.
			o.Expect(len(egressIPNodesOrderedNames)).Should(o.BeNumerically(">", 1))
			egressIPNodeStr := []string{egressIPNodesOrderedNames[0]}
			deploymentNodeStr := [][]string{
				{egressIPNodesOrderedNames[0]},
				{egressIPNodesOrderedNames[1]},
			}

			g.By("Creating the target DaemonSet with a single hostnetworked pod on the target node")
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
					nonEgressIPNodeName,
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
			podIPs, err := getDaemonSetPodIPs(clientset, externalNamespace, daemonSetName)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(podIPs)).Should(o.BeNumerically(">", 0))
			targetIP = podIPs[0]

			var routeNames []string
			for k, v := range deploymentNodeStr {
				g.By(fmt.Sprintf("Creating EgressIP test source deployment %d with number of pods equals number of EgressIP nodes", k))
				_, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, fmt.Sprint(k), ingressDomain, len(v), v)
				routeNames = append(routeNames, routeName)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			// For this test, get a single EgressIP per node.
			// Note: On some clouds like GCP, there is no dedicated CIDR per node and instead all EgressIPs come from a common pool.
			// Thus, this is only an artificial assignment of EgressIP to node on these cloud platforms and the EgressIP feature
			// will pick the actual node.
			g.By("Getting a map of source nodes and potential Egress IPs for these nodes")
			egressIPsPerNode := 1
			nodeEgressIPMap, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, egressIPNodeStr, cloudType, egressIPsPerNode)
			framework.Logf("%v", nodeEgressIPMap)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Choosing the EgressIPs to be assigned, one per node")
			egressIPSet := make(map[string]string)
			for nodeName, eip := range nodeEgressIPMap {
				_, ok := egressIPSet[eip[0]]
				if !ok {
					egressIPSet[eip[0]] = nodeName
				}
			}

			g.By("Creating the EgressIP object for OVN Kubernetes")
			egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
			egressIPObjectName := egressIPNamespace
			ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

			g.By("Applying the EgressIP object for OVN Kubernetes")
			_, err = runOcWithRetry(oc.AsAdmin(), "create", "-f", tmpDirEgressIP+"/"+egressIPYaml)
			o.Expect(err).NotTo(o.HaveOccurred())

			// This approach here is different from the other tests because:
			// a) No additional SNAT or similar can be injected by the cloud as we go directly from node and we know that we have
			// an endpoint on the cloud, always, thus we can directly query agnhost's /clientip.
			// b) The requests in tcpdump did not expose the request string for some reason (probably needed better filters)
			// c) It's simpler to just query for the /clientip instead of relying on the packet capture for these tests.
			for _, routeName := range routeNames {
				g.By(fmt.Sprintf("Launching a new prober pod and probing for EgressIPs at %s", routeName))
				numberOfRequestsToSend := 10
				clientIPSet, err := probeForClientIPs(oc, externalNamespace, probePodName, routeName, targetIP, targetPort, numberOfRequestsToSend)
				o.Expect(err).NotTo(o.HaveOccurred())

				// Note: my interpretation is that it's a bug if we see an egressIP here:
				// We should never see egressIPs when querying internal targets:
				// https://bugzilla.redhat.com/show_bug.cgi?id=2070929
				// However, this was still a subject of discussion. When we enable these tests after
				// we fix 2070929, decide if we want to see EgressIPs here or not and possibly remove
				// this verification.
				g.By("Making sure that EgressIPs were not part of the response")
				framework.Logf("egressIPSet is: %v", egressIPSet)
				framework.Logf("clientIPSet is: %v", clientIPSet)
				o.Expect(len(clientIPSet)).Should(o.BeNumerically(">", 0))
				o.Expect(
					// return false if any key of x is in y or vice-versa.
					func(x map[string]string, y map[string]struct{}) bool {
						for k := range x {
							if _, ok := y[k]; ok {
								return false
							}
						}
						for k := range y {
							if _, ok := x[k]; ok {
								return false
							}
						}
						return true
					}(egressIPSet, clientIPSet)).To(o.BeTrue())
			}
		})
	}) // end testing to internal targets

	g.Context("[external-targets][apigroup:user.openshift.io][apigroup:security.openshift.io]", func() {
		g.JustBeforeEach(func() {
			// SCC privileged is needed to run tcpdump on the packet sniffer containers, and at the minimum host networked is needed for
			// host networked pods.
			g.By("Adding SCC privileged to the external namespace")
			_, err := runOcWithRetry(oc.AsAdmin(), "adm", "policy", "add-scc-to-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:default", externalNamespace))
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Determining the interface that will be used for packet sniffing")
			packetSnifferInterface, err = findPacketSnifferInterface(oc, networkPlugin, egressIPNodesOrderedNames)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Using interface %s for packet captures", packetSnifferInterface)

			g.By("Spawning the packet sniffer pods on the EgressIP assignable hosts")
			packetSnifferDaemonSet, err = createPacketSnifferDaemonSet(oc, externalNamespace, egressIPNodesOrderedNames, targetProtocol, targetPort, packetSnifferInterface)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		// OVNKubernetes
		// OpenShiftSDN
		// Skipped on Azure due to https://bugzilla.redhat.com/show_bug.cgi?id=2073045
		g.It("pods should have the assigned EgressIPs and EgressIPs can be deleted and recreated [Skipped:azure][apigroup:route.openshift.io]", func() {
			g.By("Creating the EgressIP test source deployment with number of pods equals number of EgressIP nodes")
			_, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "", ingressDomain, len(egressIPNodesOrderedNames), egressIPNodesOrderedNames)
			o.Expect(err).NotTo(o.HaveOccurred())

			// For this test, get a single EgressIP per node.
			// Note: On some clouds like GCP, there is no dedicated CIDR per node and instead all EgressIPs come from a common pool.
			// Thus, this is only an artificial assignment of EgressIP to node on these cloud platforms and the EgressIP feature
			// will pick the actual node.
			g.By("Getting a map of source nodes and potential Egress IPs for these nodes")
			egressIPsPerNode := 1
			nodeEgressIPMap, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, egressIPNodesOrderedNames, cloudType, egressIPsPerNode)
			framework.Logf("%v", nodeEgressIPMap)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Choosing the EgressIPs to be assigned, one per node")
			egressIPSet := make(map[string]string)
			for nodeName, eip := range nodeEgressIPMap {
				_, ok := egressIPSet[eip[0]]
				if !ok {
					egressIPSet[eip[0]] = nodeName
				}
			}

			numberOfRequestsToSend := 10
			if targetHost == "self" {
				targetHost = routeName
			}
			// Run this twice to make sure that repeated EgressIP creation and deletion works.
			egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
			egressIPObjectName := egressIPNamespace
			for i := 0; i < 2; i++ {
				if networkPlugin == OVNKubernetesPluginName {
					g.By("Creating the EgressIP object for OVN Kubernetes")
					ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

					g.By("Applying the EgressIP object for OVN Kubernetes")
					applyEgressIPObject(oc, cloudNetworkClientset, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)
				} else {
					g.By("Adding EgressIPs to netnamespace and hostsubnet for OpenShiftSDN")
					openshiftSDNAssignEgressIPsManually(oc, cloudNetworkClientset, egressIPNamespace, egressIPSet, egressUpdateTimeout)
				}

				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
				spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)

				if networkPlugin == OVNKubernetesPluginName {
					g.By("Deleting the EgressIP object for OVN Kubernetes")
					// Use cascading foreground deletion to make sure that the EgressIP object and its dependencies are gone.
					_, err = runOcWithRetry(oc.AsAdmin(), "delete", "egressip", egressIPObjectName, "--cascade=foreground")
					o.Expect(err).NotTo(o.HaveOccurred())
				} else {
					g.By("Removing EgressIPs from netnamespace and hostsubnet for OpenShiftSDN")
					for eip, nodeName := range egressIPSet {
						err = sdnNamespaceRemoveEgressIP(oc, egressIPNamespace, eip)
						o.Expect(err).NotTo(o.HaveOccurred())
						err = sdnHostsubnetRemoveEgressIP(oc, nodeName, eip)
						o.Expect(err).NotTo(o.HaveOccurred())
					}
				}

				// Azure often fails on this step here - BZ https://bugzilla.redhat.com/show_bug.cgi?id=2073045
				g.By(fmt.Sprintf("Waiting for maximum %d seconds for the CloudPrivateIPConfig objects to vanish", egressUpdateTimeout))
				waitForCloudPrivateIPConfigsDeletion(oc, cloudNetworkClientset, egressIPSet, egressUpdateTimeout)

				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", 0, egressIPSet))
				spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, 0, packetSnifferDaemonSet, egressIPSet)
			}

			if networkPlugin == OVNKubernetesPluginName {
				g.By("Removing the egressIPYaml file to signal that no further cleanup is needed for OVN Kubernetes")
				os.Remove(egressIPYamlPath)
			}
		})

		// OVNKubernetes
		// OpenShiftSDN
		g.It("pods should keep the assigned EgressIPs when being rescheduled to another node", func() {
			g.By("Selecting a single EgressIP node, and a single start node for the pod")
			// requires a total of 3 worker nodes
			o.Expect(len(egressIPNodesOrderedNames)).Should(o.BeNumerically(">", 1))
			leftNode := egressIPNodesOrderedNames[0:1]
			rightNode := egressIPNodesOrderedNames[1:2]

			g.By(fmt.Sprintf("Creating the EgressIP test source deployment on node %s", rightNode[0]))
			deploymentName, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "", ingressDomain, len(rightNode), rightNode)
			o.Expect(err).NotTo(o.HaveOccurred())

			// Getting an EgressIP for a specific node only works on AWS. However, the important
			// thing here is that we get only a single EgressIP which will be assigned to one
			// of the 2 nodes only. On AWS, the EgressIP and the pod will end up on different nodes,
			// the pod will then always be moved to the node that the EgressIP is on. On other cloud
			// platforms, what happens depends on the involved controllers. Either, the pod and
			// EgressIPs start out on the same node, or on different nodes. The end result though
			// is that we always test both scenarios: pod and EgressIP on the same node, pod and
			// EgressIP on different nodes. And we also test that pods can be moved between nodes.
			g.By(fmt.Sprintf("Finding potential Egress IPs for node %s", leftNode[0]))
			egressIPsPerNode := 1
			nodeEgressIPMap, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, leftNode, cloudType, egressIPsPerNode)
			framework.Logf("%v", nodeEgressIPMap)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Choosing the single EgressIP to be assigned")
			egressIPSet := make(map[string]string)
			for nodeName, eip := range nodeEgressIPMap {
				_, ok := egressIPSet[eip[0]]
				if !ok {
					egressIPSet[eip[0]] = nodeName
				}
			}

			// This step is different depending on the network plugin.
			if networkPlugin == OVNKubernetesPluginName {
				g.By("Creating the EgressIP object for OVN Kubernetes")
				egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
				egressIPObjectName := egressIPNamespace
				ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

				g.By("Applying the EgressIP object for OVN Kubernetes")
				applyEgressIPObject(oc, cloudNetworkClientset, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)
			} else {
				g.By("Patching the netnamespace and hostsubnet for OpenShiftSDN")
				openshiftSDNAssignEgressIPsManually(oc, cloudNetworkClientset, egressIPNamespace, egressIPSet, egressUpdateTimeout)
			}

			numberOfRequestsToSend := 10
			if targetHost == "self" {
				targetHost = routeName
			}
			g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
			spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)

			g.By("Updating the source deployment's Affinity and moving it to the other source node")
			err = updateDeploymentAffinity(oc, egressIPNamespace, deploymentName, leftNode)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
			spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)
		})

		// OVNKubernetes
		// Skipped on OpenShiftSDN as the plugin does not support pod selectors.
		g.It("only pods matched by the pod selector should have the EgressIPs [Skipped:Network/OpenShiftSDN]", func() {
			g.By("Creating the EgressIP test source deployment with number of pods equals number of EgressIP nodes")
			deployment0Name, route0Name, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "0", ingressDomain, len(egressIPNodesOrderedNames), egressIPNodesOrderedNames)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating the second EgressIP test source deployment with number of pods equals number of EgressIP nodes")
			_, route1Name, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "1", ingressDomain, len(egressIPNodesOrderedNames), egressIPNodesOrderedNames)
			o.Expect(err).NotTo(o.HaveOccurred())

			// For this test, get a single EgressIP per node.
			// Note: On some clouds like GCP, there is no dedicated CIDR per node and instead all EgressIPs come from a common pool.
			// Thus, this is only an artificial assignment of EgressIP to node on these cloud platforms and the EgressIP feature
			// will pick the actual node.
			g.By("Getting a map of source nodes and potential Egress IPs for these nodes")
			egressIPsPerNode := 1
			nodeEgressIPMap, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, egressIPNodesOrderedNames, cloudType, egressIPsPerNode)
			framework.Logf("%v", nodeEgressIPMap)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Choosing the EgressIPs to be assigned, one per node")
			egressIPSet := make(map[string]string)
			for nodeName, eip := range nodeEgressIPMap {
				_, ok := egressIPSet[eip[0]]
				if !ok {
					egressIPSet[eip[0]] = nodeName
				}
			}

			g.By("Creating the EgressIP object for OVN Kubernetes")
			egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
			egressIPObjectName := egressIPNamespace
			ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, fmt.Sprintf("app: %s", deployment0Name), egressIPSet)

			g.By("Applying the EgressIP object for OVN Kubernetes")
			applyEgressIPObject(oc, cloudNetworkClientset, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)

			numberOfRequestsToSend := 10
			if targetHost == "self" {
				targetHost = route0Name
			}
			g.By(fmt.Sprintf("Testing first EgressIP test source deployment and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
			spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, route0Name, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)

			if targetHost == "self" {
				targetHost = route1Name
			}
			g.By(fmt.Sprintf("Testing second EgressIP test source deployment and making sure that %d requests with search string and EgressIPs %v were seen", 0, egressIPSet))
			spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, route1Name, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, 0, packetSnifferDaemonSet, egressIPSet)
		})

		// OVNKubernetes
		// Skipped on OpenShiftSDN as this plugin has no EgressIPs object
		g.It("pods should have the assigned EgressIPs and EgressIPs can be updated [Skipped:Network/OpenShiftSDN]", func() {
			g.By("Creating the EgressIP test source deployment with number of pods equals number of EgressIP nodes")
			_, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "", ingressDomain, len(egressIPNodesOrderedNames), egressIPNodesOrderedNames)
			o.Expect(err).NotTo(o.HaveOccurred())

			// For this test, get a single EgressIP per node.
			// Note: On some clouds like GCP, there is no dedicated CIDR per node and instead all EgressIPs come from a common pool.
			// Thus, this is only an artificial assignment of EgressIP to node on these cloud platforms and the EgressIP feature
			// will pick the actual node.
			g.By("Getting a map of source nodes and potential Egress IPs for these nodes")
			egressIPsPerNode := 1
			nodeEgressIPMap, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, egressIPNodesOrderedNames, cloudType, egressIPsPerNode)
			framework.Logf("%v", nodeEgressIPMap)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Choosing the EgressIPs to be assigned, one per node, for a total of 2 nodes")
			i := 0
			egressIPSetTemp := make(map[string]string)
			for nodeName, eip := range nodeEgressIPMap {
				// only do this for 2 nodes
				if i > 1 {
					break
				}
				i++

				_, ok := egressIPSetTemp[eip[0]]
				if !ok {
					egressIPSetTemp[eip[0]] = nodeName
				}
			}
			o.Expect(len(egressIPSetTemp)).Should(o.BeNumerically("==", 2))

			// Run this for each of the EgressIPs (and because we are applying, this will update the EgressIP object)
			numberOfRequestsToSend := 10
			if targetHost == "self" {
				targetHost = routeName
			}
			for eip, nodeName := range egressIPSetTemp {
				egressIPSet := map[string]string{eip: nodeName}

				g.By("Creating the EgressIP object for OVN Kubernetes")
				egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
				egressIPObjectName := egressIPNamespace
				ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

				g.By("Applying the EgressIP object for OVN Kubernetes")
				applyEgressIPObject(oc, cloudNetworkClientset, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)

				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
				spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)
			}
		})

		// OpenShiftSDN
		// Skipped on OVNKubernetes
		g.It("EgressIPs can be assigned automatically [Skipped:Network/OVNKubernetes]", func() {
			g.By("Adding EgressCIDR configuration to hostSubnets for OpenShiftSDN")
			for _, eipNodeName := range egressIPNodesOrderedNames {
				for _, node := range workerNodesOrdered {
					if node.Name == eipNodeName {
						nodeEgressIPConfigs, err := getNodeEgressIPConfiguration(&node)
						if err != nil {
							o.Expect(err).NotTo(o.HaveOccurred())
						}
						o.Expect(len(nodeEgressIPConfigs)).Should(o.BeNumerically("==", 1))
						// TODO - not ready for dualstack (?)
						egressCIDR := nodeEgressIPConfigs[0].IFAddr.IPv4
						if egressCIDR == "" {
							egressCIDR = nodeEgressIPConfigs[0].IFAddr.IPv6
						}
						err = sdnHostsubnetSetEgressCIDR(oc, node.Name, egressCIDR)
						o.Expect(err).NotTo(o.HaveOccurred())
					}
				}
			}
			g.By("Creating the EgressIP test source deployment with number of pods equals number of EgressIP nodes")
			_, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "", ingressDomain, len(egressIPNodesOrderedNames), egressIPNodesOrderedNames)
			o.Expect(err).NotTo(o.HaveOccurred())

			// For this test, get a single EgressIP per node.
			g.By("Getting a map of source nodes and potential Egress IPs for these nodes")
			egressIPsPerNode := 1
			nodeEgressIPMap, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, egressIPNodesOrderedNames, cloudType, egressIPsPerNode)
			framework.Logf("%v", nodeEgressIPMap)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Choosing the EgressIPs to be assigned, one per node")
			egressIPSet := make(map[string]string)
			for nodeName, eip := range nodeEgressIPMap {
				_, ok := egressIPSet[eip[0]]
				if !ok {
					egressIPSet[eip[0]] = nodeName
				}
			}

			g.By("Patching the netnamespace for OpenShiftSDN")
			for eip := range egressIPSet {
				err := sdnNamespaceAddEgressIP(oc, egressIPNamespace, eip)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			numberOfRequestsToSend := 10
			if targetHost == "self" {
				targetHost = routeName
			}
			g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
			spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)
		})
	}) // end testing to external targets
})

//
// Functions to reduce code duplication below - those could also go into egressip_helpers.go, but they feel more appropriate here as they call
// the various testing framework matchers such as o.Expect, etc. These functions also have no return value.
// Consider these to be lego pieces that the various different test scenarios above
// use and that can serve as readymade drop-in replacements for larger chunks of code.
//

// spawnProberSendEgressIPTrafficCheckLogs is a wrapper function to reduce code duplication when probing for EgressIPs.
// Unfortunately, it can take a bit of time for EgressIPs to become active, so spawnProberSendEgressIPTrafficCheckLogs adds a 15 second retry
// mechanism which eventually must observe an EgressIP in the logs before running the actual test.
// It launches a new prober pod and sends <iterations> of requests with a unique search string. It then makes sure that <expectedHits> number
// of hits were seen.
func spawnProberSendEgressIPTrafficCheckLogs(
	oc *exutil.CLI, externalNamespace, probePodName, routeName, targetProtocol, targetHost string, targetPort, iterations, expectedHits int, packetSnifferDaemonSet *v1.DaemonSet, egressIPSet map[string]string) {

	framework.Logf("Launching a new prober pod")
	proberPod := createProberPod(oc, externalNamespace, probePodName)
	defer func() {
		framework.Logf("Destroying the prober pod")
		err := destroyProberPod(oc, proberPod)
		o.Expect(err).NotTo(o.HaveOccurred())
	}()

	// Unfortunately, even after we created the EgressIP object and the CloudPrivateIPConfig, it can take some time before everything is applied correctly.
	// Retry this test every 30 seconds for up to 2 minutes to give the cluster time to converge - eventually, this test should pass.
	o.Eventually(func() bool {
		framework.Logf("Verifying that the expected number of EgressIP outbound requests can be seen in the packet sniffer logs")
		result, err := sendEgressIPProbesAndCheckPacketSnifferLogs(oc, proberPod, routeName, targetProtocol, targetHost, targetPort, iterations, expectedHits, packetSnifferDaemonSet, egressIPSet, 10)
		return err == nil && result
	}, 120*time.Second, 30*time.Second).Should(o.BeTrue())
}

// ovnKubernetesCreateEgressIPObject creates the file containing the EgressIP YAML definition which can
// then be applied.
func ovnKubernetesCreateEgressIPObject(oc *exutil.CLI, egressIPYamlPath, egressIPObjectName, egressIPNamespace, podSelector string, egressIPSet map[string]string) string {
	framework.Logf("Marshalling the desired EgressIPs into a string")
	var egressIPs []string
	for eip := range egressIPSet {
		egressIPs = append(egressIPs, eip)
	}
	egressIPsString, err := json.Marshal(egressIPs)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Logf("Creating the EgressIP object and writing it to disk")
	var egressIPConfig string
	if podSelector == "" {
		egressIPConfig = fmt.Sprintf(
			egressIPYamlTemplateNamespaceSelector, // template yaml
			egressIPObjectName,                    // name of EgressIP
			egressIPsString,                       // compact yaml of egressIPs
			fmt.Sprintf("kubernetes.io/metadata.name: %s", egressIPNamespace), // namespace selector
		)
	} else {
		egressIPConfig = fmt.Sprintf(
			egressIPYamlTemplatePodAndNamespaceSelector, // template yaml
			egressIPNamespace, // name of EgressIP
			egressIPsString,   // compact yaml of egressIPs
			podSelector,       // pod selector
			fmt.Sprintf("kubernetes.io/metadata.name: %s", egressIPNamespace), // namespace selector
		)
	}
	err = ioutil.WriteFile(egressIPYamlPath, []byte(egressIPConfig), 0644)
	o.Expect(err).NotTo(o.HaveOccurred())

	return egressIPYamlPath
}

// applyEgressIPObject is a wrapper that applies the EgressIP object in file <egressIPYamlPath> with name <egressIPObjectName>
// The propagation from a created EgressIP object to CloudPrivateIPConfig can take quite some time on Azure, hence also add a
// check that waits for the CloudPrivateIPConfigs to be created.
func applyEgressIPObject(oc *exutil.CLI, cloudNetworkClientset cloudnetwork.Interface, egressIPYamlPath, egressIPObjectName string, egressIPSet map[string]string, timeout int) {
	framework.Logf("Applying the EgressIP object %s", egressIPObjectName)
	_, err := runOcWithRetry(oc.AsAdmin(), "apply", "-f", egressIPYamlPath)
	o.Expect(err).NotTo(o.HaveOccurred())

	if cloudNetworkClientset != nil {
		framework.Logf(fmt.Sprintf("Waiting for CloudPrivateIPConfig creation for a maximum of %d seconds", timeout))
		var exists bool
		var isAssigned bool
		o.Eventually(func() bool {
			for eip := range egressIPSet {
				exists, isAssigned, err = cloudPrivateIpConfigExists(oc, cloudNetworkClientset, eip)
				o.Expect(err).NotTo(o.HaveOccurred())
				if !exists {
					framework.Logf("CloudPrivateIPConfig for %s not found.", eip)
					return false
				}
				if !isAssigned {
					framework.Logf("CloudPrivateIPConfig for %s not assigned.", eip)
					return false
				}
			}
			framework.Logf("CloudPrivateIPConfigs for %v found.", egressIPSet)
			return true
		}, time.Duration(timeout)*time.Second, 5*time.Second).Should(o.BeTrue())
	}

	framework.Logf(fmt.Sprintf("Waiting for EgressIP addresses inside status of EgressIP CR %s for a maximum of %d seconds", egressIPObjectName, timeout))
	var hasIP bool
	var nodeName string
	o.Eventually(func() bool {
		for eip := range egressIPSet {
			hasIP, nodeName, err = egressIPStatusHasIP(oc, egressIPObjectName, eip)
			o.Expect(err).NotTo(o.HaveOccurred())
			if !hasIP {
				framework.Logf("EgressIP object %s does not have IP %s in its status field.", egressIPObjectName, eip)
				return false
			} else {
				egressIPSet[eip] = nodeName
			}
		}
		framework.Logf("Egress IP object %s does have all IPs for %v.", egressIPObjectName, egressIPSet)
		return true
	}, time.Duration(timeout)*time.Second, 5*time.Second).Should(o.BeTrue())
}

// waitForCloudPrivateIPConfigsDeletion will wait for all cloudprivateipconfig objects for the given IPs
// to vanish.
func waitForCloudPrivateIPConfigsDeletion(oc *exutil.CLI, cloudNetworkClientset cloudnetwork.Interface, egressIPSet map[string]string, timeout int) {
	var exists bool
	var err error

	o.Eventually(func() bool {
		for eip := range egressIPSet {
			exists, _, err = cloudPrivateIpConfigExists(oc, cloudNetworkClientset, eip)
			o.Expect(err).NotTo(o.HaveOccurred())
			if exists {
				framework.Logf("CloudPrivateIPConfig for %s found.", eip)
				return false
			}
		}
		framework.Logf("CloudPrivateIPConfigs for %v not found.", egressIPSet)
		return true
	}, time.Duration(timeout)*time.Second, 5*time.Second).Should(o.BeTrue())
}

// openshiftSDNAssignEgressIPsManually adds EgressIPs to hostsubnet and netnamespace.
func openshiftSDNAssignEgressIPsManually(oc *exutil.CLI, cloudNetworkClientset cloudnetwork.Interface, egressIPNamespace string, egressIPSet map[string]string, timeout int) {
	var err error
	for eip, nodeName := range egressIPSet {
		framework.Logf("Adding EgressIP %s to hostnamespace %s", eip, egressIPNamespace)
		err = sdnNamespaceAddEgressIP(oc, egressIPNamespace, eip)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Adding EgressIP %s to hostsubnet %s", eip, nodeName)
		err = sdnHostsubnetAddEgressIP(oc, nodeName, eip)
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	framework.Logf(fmt.Sprintf("Waiting for CloudPrivateIPConfig creation for a maximum of %d seconds", timeout))
	var exists bool
	var isAssigned bool
	o.Eventually(func() bool {
		for eip := range egressIPSet {
			exists, isAssigned, err = cloudPrivateIpConfigExists(oc, cloudNetworkClientset, eip)
			o.Expect(err).NotTo(o.HaveOccurred())
			if !exists {
				framework.Logf("CloudPrivateIPConfig for %s not found.", eip)
				return false
			}
			if !isAssigned {
				framework.Logf("CloudPrivateIPConfig for %s not assigned.", eip)
				return false
			}
		}
		framework.Logf("CloudPrivateIPConfigs for %v found.", egressIPSet)
		return true
	}, time.Duration(timeout)*time.Second, 5*time.Second).Should(o.BeTrue())
}
