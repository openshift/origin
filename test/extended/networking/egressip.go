package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
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
		if networkPluginName() != OVNKubernetesPluginName {
			skipper.Skipf("This cluster does not use OVN Kubernetes")
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
		targetProtocol, targetHost, targetPort, err = getTargetProtocolHostPort(oc, hasIPv4, hasIPv6, cloudType)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Testing against: CloudType: %s, Protocol %s, TargetHost: %s, TargetPort: %d",
			cloudType,
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

		g.By("Setting the EgressIP nodes as EgressIP assignable")
		for _, node := range egressIPNodesOrderedNames {
			_, err = runOcWithRetry(oc.AsAdmin(), "label", "node", node, "k8s.ovn.org/egress-assignable=")
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	// Do not check for errors in g.AfterEach as the other cleanup steps will fail, otherwise.
	g.AfterEach(func() {
		g.By("Deleting the EgressIP object if it exists")
		egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
		if _, err := os.Stat(egressIPYamlPath); err == nil {
			_, _ = runOcWithRetry(oc.AsAdmin(), "delete", "-f", tmpDirEgressIP+"/"+egressIPYaml)
		}

		g.By("Removing the EgressIP assignable annotation")
		for _, nodeName := range egressIPNodesOrderedNames {
			removeEgressIPAssignableLabelIfSafe(oc, nodeName)
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

			g.By("Creating the EgressIP object")
			egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
			egressIPObjectName := egressIPNamespace
			createEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

			g.By("Applying the EgressIP object")
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
			packetSnifferInterface, err = findPacketSnifferInterface(oc, egressIPNodesOrderedNames)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Using interface %s for packet captures", packetSnifferInterface)

			g.By("Spawning the packet sniffer pods on the EgressIP assignable hosts")
			packetSnifferDaemonSet, err = createPacketSnifferDaemonSet(oc, externalNamespace, egressIPNodesOrderedNames, targetProtocol, targetPort, packetSnifferInterface)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

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
				g.By("Creating the EgressIP object")
				createEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

				g.By("Applying the EgressIP object")
				applyEgressIPObject(oc, cloudNetworkClientset, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)

				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
				spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)

				g.By("Deleting the EgressIP object")
				// Use cascading foreground deletion to make sure that the EgressIP object and its dependencies are gone.
				_, err = runOcWithRetry(oc.AsAdmin(), "delete", "egressip", egressIPObjectName, "--cascade=foreground")
				o.Expect(err).NotTo(o.HaveOccurred())

				// Azure often fails on this step here - BZ https://bugzilla.redhat.com/show_bug.cgi?id=2073045
				g.By(fmt.Sprintf("Waiting for maximum %d seconds for the CloudPrivateIPConfig objects to vanish", egressUpdateTimeout))
				waitForCloudPrivateIPConfigsDeletion(oc, cloudNetworkClientset, egressIPSet, egressUpdateTimeout)

				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", 0, egressIPSet))
				spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, 0, packetSnifferDaemonSet, egressIPSet)
			}

			g.By("Removing the egressIPYaml file to signal that no further cleanup is needed")
			os.Remove(egressIPYamlPath)
		})

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
			g.By("Creating the EgressIP object")
			egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
			egressIPObjectName := egressIPNamespace
			createEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

			g.By("Applying the EgressIP object")
			applyEgressIPObject(oc, cloudNetworkClientset, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)

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

		g.It("only pods matched by the pod selector should have the EgressIPs", func() {
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

			g.By("Creating the EgressIP object")
			egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
			egressIPObjectName := egressIPNamespace
			createEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, fmt.Sprintf("app: %s", deployment0Name), egressIPSet)

			g.By("Applying the EgressIP object")
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

		g.It("pods should have the assigned EgressIPs and EgressIPs can be updated", func() {
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

				g.By("Creating the EgressIP object")
				egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
				egressIPObjectName := egressIPNamespace
				createEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

				g.By("Applying the EgressIP object")
				applyEgressIPObject(oc, cloudNetworkClientset, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)

				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests with search string and EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
				spawnProberSendEgressIPTrafficCheckLogs(oc, externalNamespace, probePodName, routeName, targetProtocol, targetHost, targetPort, numberOfRequestsToSend, numberOfRequestsToSend, packetSnifferDaemonSet, egressIPSet)
			}
		})
	}) // end testing to external targets
})

var _ = g.Describe("[sig-network][Feature:EgressIPSecondaryHost][apigroup:operator.openshift.io]", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("egressip-secondary", admissionapi.LevelPrivileged)

	const (
		secondaryNICName              = "enp3s0"
		secondaryEgressIP             = "192.168.221.100"
		secondaryTargetIP             = "192.168.221.101"
		secondaryMacvlanNetName       = "eip-test-net"
		secondaryMacvlanContainerName = "eip-test"
		secondaryAgnhostImage         = "registry.k8s.io/e2e-test-images/agnhost:2.59"
	)

	var (
		clientset          kubernetes.Interface
		tmpDirEgressIP     string
		egressNode         string
		egressIPNamespace  string
		snifferNamespace   string
		macvlanTargetNode  string
		workerNodesOrdered []corev1.Node
		workerNodeNames    []string
	)

	g.BeforeEach(func() {
		g.By("Verifying that this cluster uses OVN-Kubernetes")
		if networkPluginName() != OVNKubernetesPluginName {
			skipper.Skipf("This cluster does not use OVN Kubernetes")
		}

		g.By("Verifying that the platform is BareMetal")
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infra.Spec.PlatformSpec.Type != configv1.BareMetalPlatformType {
			skipper.Skipf("This test requires BareMetal platform (got %s)", infra.Spec.PlatformSpec.Type)
		}

		g.By("Creating a temp directory")
		tmpDirEgressIP, err = os.MkdirTemp("", "egressip-secondary-e2e")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Getting the kubernetes clientset")
		f := oc.KubeFramework()
		clientset = f.ClientSet

		g.By("Selecting worker nodes")
		var nodeErr error
		workerNodesOrdered, nodeErr = getWorkerNodesOrdered(clientset)
		o.Expect(nodeErr).NotTo(o.HaveOccurred())
		if len(workerNodesOrdered) < 2 {
			skipper.Skipf("This test requires at least 2 worker nodes (got %d)", len(workerNodesOrdered))
		}
		workerNodeNames = make([]string, len(workerNodesOrdered))
		for i, n := range workerNodesOrdered {
			workerNodeNames[i] = n.Name
		}
		egressNode = workerNodesOrdered[1].Name
		framework.Logf("Selected egressNode=%s for labeling, all workers=%v", egressNode, workerNodeNames)

		g.By("Verifying that the secondary interface exists on all worker nodes")
		for _, w := range workerNodesOrdered {
			_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, w.Name, f.Namespace.Name, "ip", "link", "show", secondaryNICName)
			if err != nil {
				skipper.Skipf("Secondary interface %s not found on node %s: %v", secondaryNICName, w.Name, err)
			}
		}

		g.By("Setting up namespaces")
		egressIPNamespace = f.Namespace.Name
		snifferNamespace = oc.SetupProject()
	})

	g.AfterEach(func() {
		if egressNode != "" {
			g.By("Removing the EgressIP assignable annotation from " + egressNode)
			removeEgressIPAssignableLabelIfSafe(oc, egressNode)
		}

		g.By("Removing the temp directory")
		os.RemoveAll(tmpDirEgressIP)
	})

	g.It("should prevent pod IP leakage during reconciliation when new pod added to existing EgressIP namespace on secondary host interface", func() {
		const numStressPods = 20

		g.By("Labeling egress node " + egressNode + " as EgressIP assignable")
		_, err := runOcWithRetry(oc.AsAdmin(), "label", "node", egressNode, "k8s.ovn.org/egress-assignable=")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("Creating %d stress pods to slow down EgressIP reconciliation", numStressPods))
		for i := 0; i < numStressPods; i++ {
			podName := fmt.Sprintf("stress-pod-%d", i)
			frameworkpod.CreateExecPodOrFail(context.TODO(), clientset, egressIPNamespace, podName, nil)
		}

		g.By("Creating the EgressIP object")
		egressIPSet := map[string]string{secondaryEgressIP: ""}
		egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
		egressIPObjectName := egressIPNamespace
		createEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

		g.By("Applying the EgressIP object and waiting for assignment")
		applyEgressIPObject(oc, nil, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)
		defer func() {
			g.By("Deleting the EgressIP object")
			if _, err := os.Stat(egressIPYamlPath); err == nil {
				_, _ = runOcWithRetry(oc.AsAdmin(), "delete", "-f", egressIPYamlPath)
			}
		}()
		assignedEgressNode := egressIPSet[secondaryEgressIP]
		framework.Logf("EgressIP %s was assigned to node %s (labeled node was %s)", secondaryEgressIP, assignedEgressNode, egressNode)

		g.By("Enabling IP forwarding on all worker nodes")
		type sysctlState struct {
			origIPForward string
			origIfForward string
		}
		forwardingStates := make(map[string]sysctlState)
		for _, nodeName := range workerNodeNames {
			origIPFwd, fwdErr := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "default",
				"sysctl", "-n", "net.ipv4.ip_forward")
			o.Expect(fwdErr).NotTo(o.HaveOccurred())
			origIfFwd, fwdErr := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "default",
				"sysctl", "-n", "net.ipv4.conf."+secondaryNICName+".forwarding")
			o.Expect(fwdErr).NotTo(o.HaveOccurred())
			forwardingStates[nodeName] = sysctlState{
				origIPForward: strings.TrimSpace(origIPFwd),
				origIfForward: strings.TrimSpace(origIfFwd),
			}
			_, fwdErr = exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "default",
				"sysctl", "-w", "net.ipv4.ip_forward=1")
			o.Expect(fwdErr).NotTo(o.HaveOccurred())
			_, fwdErr = exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "default",
				"sysctl", "-w", "net.ipv4.conf."+secondaryNICName+".forwarding=1")
			o.Expect(fwdErr).NotTo(o.HaveOccurred())
			framework.Logf("Enabled IP forwarding on %s (was ip_forward=%s, %s.forwarding=%s)",
				nodeName, forwardingStates[nodeName].origIPForward, secondaryNICName, forwardingStates[nodeName].origIfForward)
		}
		defer func() {
			g.By("Restoring IP forwarding settings on all worker nodes")
			for nodeName, state := range forwardingStates {
				_, _ = exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "default",
					"sysctl", "-w", "net.ipv4.ip_forward="+state.origIPForward)
				_, _ = exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "default",
					"sysctl", "-w", "net.ipv4.conf."+secondaryNICName+".forwarding="+state.origIfForward)
			}
		}()

		g.By("Selecting a master node for the macvlan target")
		masterNodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(masterNodes.Items).NotTo(o.BeEmpty(), "no master nodes found")
		macvlanTargetNode = masterNodes.Items[0].Name
		framework.Logf("macvlanTargetNode=%s (master node), assignedEgressNode=%s", macvlanTargetNode, assignedEgressNode)

		g.By("Verifying that the secondary interface exists on macvlan target node " + macvlanTargetNode)
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, macvlanTargetNode, "default",
			"ip", "link", "show", secondaryNICName)
		o.Expect(err).NotTo(o.HaveOccurred(), "secondary interface %s not found on master node %s", secondaryNICName, macvlanTargetNode)

		g.By("Deploying macvlan agnhost container as HTTP target on " + macvlanTargetNode)
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, macvlanTargetNode, "default",
			"podman", "network", "create", "--driver", "macvlan", "--ipam-driver=none",
			"-o", "parent="+secondaryNICName, secondaryMacvlanNetName)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create macvlan podman network")
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, macvlanTargetNode, "default",
			"podman", "run", "-d", "--privileged", "--name", secondaryMacvlanContainerName,
			"--network", secondaryMacvlanNetName, secondaryAgnhostImage,
			"netexec", fmt.Sprintf("--http-port=%d", serverPort))
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to run macvlan agnhost container")
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, macvlanTargetNode, "default",
			"podman", "exec", secondaryMacvlanContainerName,
			"ip", "address", "add", "dev", "eth0", secondaryTargetIP+"/24")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to assign IP to macvlan container")
		defer func() {
			g.By("Cleaning up macvlan container and network on " + macvlanTargetNode)
			_, _ = exutil.DebugNodeRetryWithOptionsAndChroot(oc, macvlanTargetNode, "default",
				"podman", "rm", "-f", secondaryMacvlanContainerName)
			_, _ = exutil.DebugNodeRetryWithOptionsAndChroot(oc, macvlanTargetNode, "default",
				"podman", "network", "rm", "-f", secondaryMacvlanNetName)
		}()

		g.By("Verifying macvlan target is reachable from the egress node via the secondary interface")
		targetURL := "http://" + net.JoinHostPort(secondaryTargetIP, strconv.Itoa(serverPort)) + "/"
		o.Eventually(func() bool {
			curlOut, curlErr := exutil.DebugNodeRetryWithOptionsAndChroot(oc, assignedEgressNode, "default",
				"curl", "-s", "--max-time", "2", "--connect-timeout", "2", "--interface", secondaryNICName, targetURL)
			if curlErr != nil {
				framework.Logf("Macvlan target not yet reachable from %s: %v", assignedEgressNode, curlErr)
				return false
			}
			framework.Logf("Macvlan target reachable from %s: %s", assignedEgressNode, strings.TrimSpace(curlOut))
			return true
		}, 30*time.Second, 5*time.Second).Should(o.BeTrue(), "Macvlan target %s not reachable from egress node %s via %s", targetURL, assignedEgressNode, secondaryNICName)

		g.By("Setting up packet sniffer on all worker nodes")
		packetSnifferDaemonSet, err := setupPacketSnifferOnInterface(oc, clientset, snifferNamespace, workerNodeNames, secondaryNICName)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Generating a unique search string for traffic identification")
		searchUUID := uuid.New().String()

		g.By("Creating the trigger pod on the assigned egress node " + assignedEgressNode)
		triggerPodName := "trigger-pod"
		triggerPod, err := createSecondaryEgressIPTriggerPod(clientset, egressIPNamespace, triggerPodName, secondaryTargetIP, serverPort, searchUUID, []string{assignedEgressNode})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = frameworkpod.WaitForPodRunningInNamespace(context.TODO(), clientset, triggerPod)
		o.Expect(err).NotTo(o.HaveOccurred())
		triggerPod, err = clientset.CoreV1().Pods(egressIPNamespace).Get(context.Background(), triggerPodName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		triggerPodIP := triggerPod.Status.PodIP
		framework.Logf("Trigger pod %s scheduled on node %s with IP %s", triggerPodName, triggerPod.Spec.NodeName, triggerPodIP)
		o.Expect(triggerPod.Spec.NodeName).To(o.Equal(assignedEgressNode),
			"Trigger pod must run on the assigned egress node to avoid cross-node routing dependencies")

		defer func() {
			g.By("Capturing namespace inspect for debugging before cleanup")
			inspectDir := os.Getenv("ARTIFACT_DIR")
			if inspectDir == "" {
				inspectDir = tmpDirEgressIP
			}
			for _, ns := range []string{egressIPNamespace, snifferNamespace} {
				destDir := inspectDir + "/namespace-inspect-" + ns
				if inspectErr := oc.AsAdmin().WithoutNamespace().Run("adm").Args("inspect", "ns/"+ns, "--dest-dir="+destDir).Execute(); inspectErr != nil {
					framework.Logf("Failed to inspect namespace %s: %v", ns, inspectErr)
				} else {
					framework.Logf("Namespace inspect for %s stored in %s", ns, destDir)
				}
			}
		}()

		g.By("Dumping EgressIP datapath state on the egress node for debugging")
		for _, debugCmd := range []struct{ desc, cmd string }{
			{"ip rule list", "ip rule list"},
			{"ip route show table 1004", "ip route show table 1004"},
			{"nft list table ip nat 2>/dev/null || iptables-save -t nat", "nft list table ip nat 2>/dev/null || iptables-save -t nat"},
		} {
			output, cmdErr := exutil.DebugNodeRetryWithOptionsAndChroot(oc, assignedEgressNode, "default",
				"bash", "-c", debugCmd.cmd)
			if cmdErr != nil {
				framework.Logf("Failed to run '%s' on %s: %v", debugCmd.desc, assignedEgressNode, cmdErr)
			} else {
				framework.Logf("%s on %s:\n%s", debugCmd.desc, assignedEgressNode, output)
			}
		}

		g.By("Waiting for traffic to accumulate in the packet sniffer")
		time.Sleep(15 * time.Second)

		g.By("Scanning packet sniffer logs for the trigger pod's traffic")
		var found map[string]int
		o.Eventually(func() bool {
			var scanErr error
			found, scanErr = scanPacketSnifferDaemonSetPodLogs(oc, packetSnifferDaemonSet, "http", searchUUID)
			if scanErr != nil {
				framework.Logf("Error scanning sniffer logs: %v", scanErr)
				return false
			}
			totalHits := 0
			for _, count := range found {
				totalHits += count
			}
			if totalHits == 0 {
				framework.Logf("No 'Parsed' lines with UUID found yet, checking EgressIP status...")
				eipStatus, statusErr := runOcWithRetry(oc.AsAdmin(), "get", "egressip", egressIPNamespace, "-o", "jsonpath={.status.items[*].node}")
				if statusErr == nil {
					framework.Logf("Current EgressIP assignment: %s", eipStatus)
				}
			}
			return totalHits > 0
		}, 300*time.Second, 5*time.Second).Should(o.BeTrue(), "No traffic captured in sniffer logs")

		g.By("Analyzing captured source IPs for pod IP leakage")
		framework.Logf("Captured %d distinct source IPs from sniffer logs", len(found))
		framework.Logf("Expected EgressIP: %s, Trigger pod IP: %s", secondaryEgressIP, triggerPodIP)
		egressIPCount := 0
		leakCount := 0
		for ip, count := range found {
			if _, isEgressIP := egressIPSet[ip]; isEgressIP {
				egressIPCount += count
			} else if ip == triggerPodIP {
				leakCount += count
				framework.Logf("LEAK DETECTED: Trigger pod IP %s seen %d times as source", ip, count)
			}
		}
		framework.Logf("Results: %d packets with EgressIP, %d packets leaked", egressIPCount, leakCount)
		o.Expect(leakCount).To(o.Equal(0),
			fmt.Sprintf("Found %d packets with non-EgressIP source IPs - indicates pod IP leakage during reconciliation", leakCount))
		o.Expect(egressIPCount).To(o.BeNumerically(">", 0),
			"Should see traffic with EgressIP as source")
	})
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

// createEgressIPObject creates the file containing the EgressIP YAML definition which can
// then be applied.
func createEgressIPObject(oc *exutil.CLI, egressIPYamlPath, egressIPObjectName, egressIPNamespace, podSelector string, egressIPSet map[string]string) string {
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
		framework.Logf("Waiting for CloudPrivateIPConfig creation for a maximum of %d seconds", timeout)
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

	framework.Logf("Waiting for EgressIP addresses inside status of EgressIP CR %s for a maximum of %d seconds", egressIPObjectName, timeout)
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
