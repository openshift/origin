package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	"k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"
	cloudnetwork "github.com/openshift/client-go/cloudnetwork/clientset/versioned"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	namespacePrefix = "egressip"
	egressIPYaml    = "egressip.yaml"
	probePodName    = "prober-pod"

	// Port 6443 should be open on most cloud platforms by default. That way, we do not necessarily have
	// to update security groups (e.g. Azure). For AWS, we choose port 30000 for the same reasons (it's open for
	// the worker node security group).
	externalTargetPort    = 6443
	externalTargetPortAWS = 30000

	// Max time that we wait for changes to EgressIP objects
	// to propagate to the CloudPrivateIPConfig objects.
	// This can take a significant amount of time on Azure.
	// BZ https://bugzilla.redhat.com/show_bug.cgi?id=2073045
	egressUpdateTimeout = 300

	agnhostImage            = "k8s.gcr.io/e2e-test-images/agnhost:2.43"
	targetHostStartupScript = `#!/bin/bash
yum install podman -y
firewall-cmd --add-port={{listenPort}}/tcp
podman run --network=host -d --rm {{agnhost}} netexec --http-port {{listenPort}} --udp-port {{listenPort}}
`

	// targetHostRequestPublicIP should always be set to false. It is only useful for debugging of the target host
	// while developing EgressIP tests.
	// If this is set to true, assign a DNAT / floating IP so that one can SSH into the host and open port 22 to
	// 0.0.0.0/0. Additionally, print the private and public SSH key to the command line.
	targetHostRequestPublicIP = false
)

// EgressIP tests work as follows:
//   External host:
//     Spawn an additional RHEL 8 VM on the cloud platform for tests, install podman on that VM and start an agnhost
//     container.
//   EgressIP pod:
//     Inside the egressIPNamespace, we set up an agnhost netexec pod. Traffic outgoing of this pod will or will not be
//     matched by EgressIPs, this depends on the test. The pod will be instructed to dial to the external host during
//     the tests. In order to instruct the pod to dial to the external host, we must call into the agnhost netexec
//     dial endpoint. Hence, we expose the EgressIP pod via a SVC and route, the setup looks +/- as follows:
//       inside egressIPNamespace:
//         Route:egressIPNamespace/route-to-agnhost
//           --> SVC:egressIPNamespace/svc-to-agnhost
//               --> Deployment:egressIPNamespace/agnhost
//   prober pod:
//     A prober pod is created inside externalNamespace. It dials into the route, targets the /dial HTTP endpoint
//     which in turn triggers traffic generation from the EgressIP pod to the external host. Here is an example
//     requests that the prober pod sends:
//     kubeconfig exec prober-podwwfts -- \
//       curl --max-time 15 -s http://e2e-test-egressip-4cjkc-0.apps.cluster.com/dial?protocol=http&host=10.0.128.5&port=32667&request=/clientip
var _ = g.Describe("[sig-network][Feature:EgressIP][apigroup:config.openshift.io]", func() {
	oc := exutil.NewCLIWithPodSecurityLevel(namespacePrefix, admissionapi.LevelPrivileged)
	// portAllocator := NewPortAllocator(targetPodPortMin, targetPodPortMax)

	var (
		networkPlugin string

		clientset             kubernetes.Interface
		cloudNetworkClientset cloudnetwork.Interface
		tmpDirEgressIP        string

		// boundedReadySchedulableNodes are schedulable and available nodes (= worker nodes in most cases).
		boundedReadySchedulableNodes []corev1.Node
		egressIPNodesNames           []string
		// nonEgressIPNode              corev1.Node

		// egressIPNamespace is the namespace that the the EgressIP is configured for.
		egressIPNamespace string
		// proberPodNamespace is the namespace that the prober pod is in.
		proberPodNamespace string

		ingressDomain string
		cloudType     configv1.PlatformType
		cc            cloudClient
		targetVMIP    string
		targetVMPort  int
		targetVM      *vm
	)

	g.BeforeEach(func() {
		g.By("Verifying that this cluster uses a network plugin that is supported for this test")
		networkPlugin = networkPluginName()
		if networkPlugin != OVNKubernetesPluginName &&
			networkPlugin != openshiftSDNPluginName {
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
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(
			context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		cloudType = infra.Spec.PlatformSpec.Type

		g.By("Verifying that this is a supported cloud infrastructure platform")
		isSupportedPlatform := false
		supportedPlatforms := []configv1.PlatformType{
			configv1.AWSPlatformType,
			configv1.GCPPlatformType,
			configv1.AzurePlatformType,
			// configv1.OpenStackPlatformType, // TBD in a follow-up commit.
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

		// A supported version of OpenShift must hold the CloudPrivateIPConfig CRD. Otherwise, skip this test.
		g.By("Verifying that this is a supported version of OpenShift")
		isSupportedOcpVersion, err := exutil.DoesApiResourceExist(oc.AdminConfig(), "cloudprivateipconfigs",
			"cloud.network.openshift.io")
		o.Expect(err).NotTo(o.HaveOccurred())
		if !isSupportedOcpVersion {
			skipper.Skipf("API resource CloudPrivateIPConfigs not found")
		}

		g.By("Getting bounded ready schedulable (worker) nodes")
		nodes, err := e2enode.GetBoundedReadySchedulableNodes(f.ClientSet, 3)
		o.Expect(err).NotTo(o.HaveOccurred())
		boundedReadySchedulableNodes = nodes.Items
		if len(boundedReadySchedulableNodes) < 3 {
			skipper.Skipf("This test requires a minimum of 3 worker nodes. This environment has %d worker nodes.",
				len(boundedReadySchedulableNodes),
			)
		}

		g.By("Creating a project for the prober pod")
		// Create a target project and assign source and target namespace to variables for later use.
		egressIPNamespace = f.Namespace.Name
		proberPodNamespace = oc.SetupProject()

		g.By("Selecting the EgressIP nodes and a non-EgressIP node")
		// nonEgressIPNode = boundedReadySchedulableNodes[0]
		egressIPNodes := boundedReadySchedulableNodes[1:]
		egressIPNodesNames = make([]string, len(egressIPNodes))
		for i, s := range egressIPNodes {
			egressIPNodesNames[i] = s.Name
		}

		g.By("Setting the ingressdomain")
		ingressDomain, err = getIngressDomain(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		if networkPlugin == OVNKubernetesPluginName {
			g.By("Setting the EgressIP nodes as EgressIP assignable")
			for _, node := range egressIPNodesNames {
				_, err = runOcWithRetry(oc.AsAdmin(), "label", "node", node, "k8s.ovn.org/egress-assignable=")
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		g.By(fmt.Sprintf("Setting up the cloud client for platform %s", cloudType))
		cc, err = newCloudClient(oc, cloudType)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Setting up an external target")
		targetVMPort = externalTargetPort
		if cloudType == configv1.AWSPlatformType {
			framework.Logf("Platform is AWS, choosing target port %d", externalTargetPortAWS)
			targetVMPort = externalTargetPortAWS
		}
		targetVM, err = setupExternalTarget(oc, cc, f.Namespace.Name, targetVMPort)
		o.Expect(err).NotTo(o.HaveOccurred())
		targetVMIP = targetVM.privateIP.String()
		framework.Logf("VM private IP: %s, VM public IP: %s, VM port: %d", targetVMIP, targetVM.publicIP, targetVMPort)
	})

	g.AfterEach(func() {
		// We ignore any errors here on purpose as:
		// a) we want to continue with execution of the other steps
		// b) these errors are not important enough to stop testing / report a failure.
		if networkPluginName() == OVNKubernetesPluginName {
			g.By("Deleting the EgressIP object if it exists for OVN Kubernetes")
			egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
			if _, err := os.Stat(egressIPYamlPath); err == nil {
				_, _ = runOcWithRetry(oc.AsAdmin(), "delete", "-f", tmpDirEgressIP+"/"+egressIPYaml)
			}

			g.By("Removing the EgressIP assignable annotation for OVN Kubernetes")
			for _, nodeName := range egressIPNodesNames {
				_, _ = runOcWithRetry(oc.AsAdmin(), "label", "node", nodeName, "k8s.ovn.org/egress-assignable-")
			}
		} else {
			g.By("Removing any hostsubnet EgressIPs for OpenShiftSDN")
			for _, nodeName := range egressIPNodesNames {
				_ = sdnHostsubnetFlushEgressIPs(oc, nodeName)
				_ = sdnHostsubnetFlushEgressCIDRs(oc, nodeName)
			}
		}

		g.By("Removing the temp directory")
		os.RemoveAll(tmpDirEgressIP)

		g.By("Tearing down external target")
		tearDownExternalTarget(oc, cc, targetVM)

		g.By("Closing the CloudClient")
		err := cc.Close()
		if err != nil {
			framework.Logf("Error closing CloudClient, err: %q", err)
		}
	})

	g.Context("[external-targets][apigroup:user.openshift.io][apigroup:security.openshift.io]", func() {
		// OVNKubernetes
		// OpenShiftSDN
		// Skipped on Azure due to https://bugzilla.redhat.com/show_bug.cgi?id=2073045
		g.It("pods should have the assigned EgressIPs and EgressIPs can be deleted and recreated [Skipped:azure][apigroup:route.openshift.io]", func() {
			g.By("Creating the EgressIP test source deployment with number of pods equals number of EgressIP nodes")
			_, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "", ingressDomain,
				len(egressIPNodesNames), egressIPNodesNames)
			o.Expect(err).NotTo(o.HaveOccurred())

			// For this test, get a single EgressIP per node.
			g.By("Choosing the EgressIPs to be assigned, one per node")
			egressIPsPerNode := 1
			nodeEgressIPMap, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, cloudType,
				[]string{targetVMIP}, egressIPsPerNode, egressIPNodesNames...)
			framework.Logf("%v", nodeEgressIPMap)
			o.Expect(err).NotTo(o.HaveOccurred())

			egressIPSet := make(map[string]string)
			for nodeName, eip := range nodeEgressIPMap {
				_, ok := egressIPSet[eip[0]]
				if !ok {
					egressIPSet[eip[0]] = nodeName
				}
			}

			g.By("Setting number of requests to send")
			numberOfRequestsToSend := 10

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

				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests "+
					"with EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
				spawnProberSendEgressIPTrafficCheckOutput(oc, proberPodNamespace, probePodName, routeName, targetVMIP,
					targetVMPort, numberOfRequestsToSend, numberOfRequestsToSend, egressIPSet)

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

				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests "+
					"with EgressIPs %v were seen", 0, egressIPSet))
				spawnProberSendEgressIPTrafficCheckOutput(oc, proberPodNamespace, probePodName, routeName, targetVMIP,
					targetVMPort, numberOfRequestsToSend, 0, egressIPSet)
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
			// requires a total of 3 worker nodes (already verified in BeforeEach, additional verification here)
			o.Expect(len(egressIPNodesNames)).Should(o.BeNumerically(">", 1))
			leftNode := egressIPNodesNames[0:1]
			rightNode := egressIPNodesNames[1:2]

			g.By(fmt.Sprintf("Creating the EgressIP test source deployment on node %s", rightNode[0]))
			deploymentName, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "", ingressDomain, len(rightNode), rightNode)
			o.Expect(err).NotTo(o.HaveOccurred())

			// Getting an EgressIP for a specific node and assigning it to that exact node doesn't work. However,
			// the important thing here is that we get only a single EgressIP which will be assigned to one of the
			// 2 nodes only. Either, the pod and EgressIPs start out on the same node, or on different nodes. The end
			// result though is that we always test both scenarios: pod and EgressIP on the same node, pod and
			// EgressIP on different nodes. And we also test that pods can be moved between nodes.
			g.By(fmt.Sprintf("Finding potential EgressIPs for node %s", leftNode[0]))
			egressIPsPerNode := 1
			nodeEgressIPMap, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, cloudType,
				[]string{targetVMIP}, egressIPsPerNode, leftNode...)
			framework.Logf("%v", nodeEgressIPMap)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Choosing the single EgressIP to be assigned")
			egressIPSet := make(map[string]string)
			for nodeName, eip := range nodeEgressIPMap {
				if _, ok := egressIPSet[eip[0]]; !ok {
					egressIPSet[eip[0]] = nodeName
					break
				}
			}
			o.Expect(len(egressIPSet)).Should(o.BeNumerically(">", 0))

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

			g.By("Setting number of requests to send")
			numberOfRequestsToSend := 10

			g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests "+
				"with EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
			spawnProberSendEgressIPTrafficCheckOutput(oc, proberPodNamespace, probePodName, routeName, targetVMIP,
				targetVMPort, numberOfRequestsToSend, numberOfRequestsToSend, egressIPSet)

			g.By("Updating the source deployment's Affinity and moving it to the other source node")
			err = updateDeploymentAffinity(oc, egressIPNamespace, deploymentName, leftNode)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests "+
				"with EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
			spawnProberSendEgressIPTrafficCheckOutput(oc, proberPodNamespace, probePodName, routeName, targetVMIP,
				targetVMPort, numberOfRequestsToSend, numberOfRequestsToSend, egressIPSet)
		})

		// OVNKubernetes
		// Skipped on OpenShiftSDN as the plugin does not support pod selectors.
		g.It("only pods matched by the pod selector should have the EgressIPs [Skipped:Network/OpenShiftSDN]", func() {
			// requires a total of 3 worker nodes (already verified in BeforeEach, additional verification here)
			o.Expect(len(egressIPNodesNames)).Should(o.BeNumerically(">", 1))
			leftNode := egressIPNodesNames[0:1]
			rightNode := egressIPNodesNames[1:2]

			g.By("Creating the EgressIP test source deployment with number of pods equals number of EgressIP nodes")
			deployment0Name, route0Name, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "0",
				ingressDomain, len(leftNode), leftNode)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating the second EgressIP test source deployment with number of pods equals number of EgressIP nodes")
			_, route1Name, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "1", ingressDomain,
				len(rightNode), rightNode)
			o.Expect(err).NotTo(o.HaveOccurred())

			// For this test, get a single EgressIP per node.
			g.By("Getting a map of source nodes and potential Egress IPs for these nodes")
			egressIPsPerNode := 1
			nodeEgressIPMap, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, cloudType,
				[]string{targetVMIP}, egressIPsPerNode, egressIPNodesNames...)
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

			podSelector := fmt.Sprintf("app: %s", deployment0Name)
			g.By(fmt.Sprintf("Creating the EgressIP object for OVN Kubernetes with pod selector %q", podSelector))
			egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
			egressIPObjectName := egressIPNamespace
			ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, podSelector,
				egressIPSet)

			g.By("Applying the EgressIP object for OVN Kubernetes")
			applyEgressIPObject(oc, cloudNetworkClientset, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)

			g.By("Setting number of requests to send")
			numberOfRequestsToSend := 10

			g.By(fmt.Sprintf("Testing first EgressIP test source deployment. "+
				"Sending requests from prober and making sure that %d requests "+
				"with EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
			spawnProberSendEgressIPTrafficCheckOutput(oc, proberPodNamespace, probePodName, route0Name, targetVMIP,
				targetVMPort, numberOfRequestsToSend, numberOfRequestsToSend, egressIPSet)

			g.By(fmt.Sprintf("Testing second EgressIP test source deployment. "+
				"Sending requests from prober and making sure that %d requests "+
				"with EgressIPs %v were seen", 0, egressIPSet))
			spawnProberSendEgressIPTrafficCheckOutput(oc, proberPodNamespace, probePodName, route1Name, targetVMIP,
				targetVMPort, numberOfRequestsToSend, 0, egressIPSet)
		})

		// OVNKubernetes
		// Skipped on OpenShiftSDN as this plugin has no EgressIPs object
		g.It("pods should have the assigned EgressIPs and EgressIPs can be updated [Skipped:Network/OpenShiftSDN]", func() {
			g.By("Creating the EgressIP test source deployment with number of pods equals number of EgressIP nodes")
			_, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "", ingressDomain,
				len(egressIPNodesNames), egressIPNodesNames)
			o.Expect(err).NotTo(o.HaveOccurred())

			// For this test, get a single EgressIP per node.
			g.By("Getting a map of source nodes and potential EgressIPs for these nodes")
			egressIPsPerNode := 1
			nodeEgressIPMap, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, cloudType,
				[]string{targetVMIP}, egressIPsPerNode, egressIPNodesNames...)
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
				if _, ok := egressIPSetTemp[eip[0]]; !ok {
					egressIPSetTemp[eip[0]] = nodeName
					i++
				}
			}
			o.Expect(len(egressIPSetTemp)).Should(o.BeNumerically("==", 2))

			g.By("Setting number of requests to send")
			numberOfRequestsToSend := 10

			// Run this for each of the EgressIPs (and because we are applying, this will update the EgressIP object)
			for eip, nodeName := range egressIPSetTemp {
				egressIPSet := map[string]string{eip: nodeName}

				g.By("Creating the EgressIP object for OVN Kubernetes")
				egressIPYamlPath := tmpDirEgressIP + "/" + egressIPYaml
				egressIPObjectName := egressIPNamespace
				ovnKubernetesCreateEgressIPObject(oc, egressIPYamlPath, egressIPObjectName, egressIPNamespace, "", egressIPSet)

				g.By("Applying the EgressIP object for OVN Kubernetes")
				applyEgressIPObject(oc, cloudNetworkClientset, egressIPYamlPath, egressIPNamespace, egressIPSet, egressUpdateTimeout)

				g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests "+
					"with EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
				spawnProberSendEgressIPTrafficCheckOutput(oc, proberPodNamespace, probePodName, routeName, targetVMIP,
					targetVMPort, numberOfRequestsToSend, numberOfRequestsToSend, egressIPSet)
			}
		})

		// OpenShiftSDN
		// Skipped on OVNKubernetes
		g.It("EgressIPs can be assigned automatically [Skipped:Network/OVNKubernetes]", func() {
			g.By("Adding EgressCIDR configuration to hostSubnets for OpenShiftSDN")
			for _, eipNodeName := range egressIPNodesNames {
				for _, node := range boundedReadySchedulableNodes {
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
			_, routeName, err := createAgnhostDeploymentAndIngressRoute(oc, egressIPNamespace, "", ingressDomain, len(egressIPNodesNames), egressIPNodesNames)
			o.Expect(err).NotTo(o.HaveOccurred())

			// For this test, get a single EgressIP per node.
			g.By("Getting a map of source nodes and potential Egress IPs for these nodes")
			egressIPsPerNode := 1
			nodeEgressIPMap, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, cloudType,
				[]string{targetVMIP}, egressIPsPerNode, egressIPNodesNames...)
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

			g.By("Setting number of requests to send")
			numberOfRequestsToSend := 10
			g.By(fmt.Sprintf("Sending requests from prober and making sure that %d requests "+
				"with EgressIPs %v were seen", numberOfRequestsToSend, egressIPSet))
			spawnProberSendEgressIPTrafficCheckOutput(oc, proberPodNamespace, probePodName, routeName, targetVMIP,
				targetVMPort, numberOfRequestsToSend, numberOfRequestsToSend, egressIPSet)
		})
	}) // end testing to external targets
})

//
// Functions to reduce code duplication below - those could also go into egressip_helpers.go, but they feel more appropriate here as they call
// the various testing framework matchers such as o.Expect, etc. These functions also have no return value.
// Consider these to be lego pieces that the various different test scenarios above
// use and that can serve as readymade drop-in replacements for larger chunks of code.
//

// spawnProberSendEgressIPTrafficCheckOutput is a wrapper function to reduce code duplication when probing for EgressIPs.
// It launches a new prober pod and sends <iterations> of requests. It then makes sure that <expectedHits> number
// of hits were seen. It then destroys the prober pod.
func spawnProberSendEgressIPTrafficCheckOutput(oc *exutil.CLI, externalNamespace, probePodName, routeName,
	targetHost string, targetPort, iterations, expectedHits int, egressIPSet map[string]string) {

	framework.Logf("Launching a new prober pod")
	proberPod := createProberPod(oc, externalNamespace, probePodName)

	// Unfortunately, even after we created the EgressIP object and the CloudPrivateIPConfig, it can take some time
	// before everything is applied correctly. Retry this test every 30 seconds for up to 2 minutes to give the cluster
	// time to converge - eventually, this test should pass.
	o.Eventually(func() bool {
		framework.Logf("Verifying that the expected number of outbound requests match EgressIPs")
		result, err := sendEgressIPProbesAndCheckOutput(oc, proberPod, routeName, targetHost, targetPort, iterations, expectedHits, egressIPSet)
		if err != nil {
			framework.Logf("Received error from sendEgressIPProbesAndCheckOutput, err: %q", err)
		}
		return err == nil && result
	}, 120*time.Second, 30*time.Second).Should(o.BeTrue())

	framework.Logf("Destroying the prober pod")
	err := destroyProberPod(oc, proberPod)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// ovnKubernetesCreateEgressIPObject creates the file containing the EgressIP YAML definition which can then be applied.
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

	framework.Logf(fmt.Sprintf("Waiting for CloudPrivateIPConfig creation for a maximum of %d seconds", timeout))
	var exists bool
	var isAssigned bool
	o.Eventually(func() bool {
		for eip := range egressIPSet {
			exists, isAssigned, err = cloudPrivateIPConfigExists(oc, cloudNetworkClientset, eip)
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

	framework.Logf(fmt.Sprintf("Waiting for EgressIP addresses inside status of EgressIP CR %s for a maximum of %d seconds", egressIPObjectName, timeout))
	var hasIP bool
	o.Eventually(func() bool {
		for eip := range egressIPSet {
			hasIP, err = egressIPStatusHasIP(oc, egressIPObjectName, eip)
			o.Expect(err).NotTo(o.HaveOccurred())
			if !hasIP {
				framework.Logf("EgressIP object %s does not have IP %s in its status field.", egressIPObjectName, eip)
				return false
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
			exists, _, err = cloudPrivateIPConfigExists(oc, cloudNetworkClientset, eip)
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
			exists, isAssigned, err = cloudPrivateIPConfigExists(oc, cloudNetworkClientset, eip)
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

// setupExternalTarget sets up the external target. In order to do so, it creates an additional Virtual Machine
// outside of the current OpenShift cluster using OpenShift's cloud credentials.
// If targetHostRequestPublicIP is true then we request a public DNAT, open SSH globally and print the public and
// private key to the console to make debugging easier.
func setupExternalTarget(oc *exutil.CLI, cc cloudClient, name string, listenPort int) (*vm, error) {
	g.By("Generating listenPorts for the target VM")
	listenPorts := map[string]protocolPort{applicationPort: {port: listenPort, protocol: tcp}}
	if targetHostRequestPublicIP {
		listenPorts[sshPort] = protocolPort{port: 22, protocol: tcp}
	}

	g.By("Building startup script parameters for the VM")
	parameters := map[string]string{
		"agnhost":    agnhostImage,
		"listenPort": strconv.Itoa(listenPort),
	}

	g.By("Generating the SSH key for the target VM")
	sshPublicKey, sshPrivateKey, err := generateSSHKeyPair()
	o.Expect(err).NotTo(o.HaveOccurred())
	// Only print the SSH key contents if we request a public IP address for debugging.
	if targetHostRequestPublicIP {
		framework.Logf("Private key:\n%s", sshPrivateKey)
		framework.Logf("Public key:\n%s", sshPublicKey)
	}

	g.By(fmt.Sprintf("Creating target VM %s", name))
	v := &vm{
		name:                    name,
		ports:                   listenPorts,
		sshPrivateKey:           sshPrivateKey,
		sshPublicKey:            sshPublicKey,
		startupScript:           targetHostStartupScript,
		startupScriptParameters: parameters,
	}
	err = cc.createVM(v, targetHostRequestPublicIP)
	return v, err
}

// tearDownExternalTarget cleans up all resources that were created for the external target.
func tearDownExternalTarget(oc *exutil.CLI, cc cloudClient, vm *vm) {
	if vm == nil {
		framework.Logf("No VM created, nothing to do")
		return
	}
	err := retry.OnError(
		wait.Backoff{
			Steps:    5,
			Duration: time.Minute,
			Factor:   1.0,
			Jitter:   0.1,
		},
		func(err error) bool {
			return false
		},
		func() error {
			err := cc.deleteVM(vm)
			if err != nil {
				framework.Logf("could not delete VM %s, err: %q", vm.name, err)
			}
			return err
		})
	if err != nil {
		framework.Logf("Teardown of target failed with error: %q", err)
	}
}
