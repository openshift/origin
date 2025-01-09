package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	testutils "k8s.io/kubernetes/test/utils"
	admissionapi "k8s.io/pod-security-admission/api"

	nadapi "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"

	"github.com/openshift/origin/test/extended/networking/kubevirt"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = Describe("[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("network-segmentation-e2e", admissionapi.LevelBaseline)
	f := oc.KubeFramework()

	InOVNKubernetesContext(func() {
		var (
			cs         clientset.Interface
			nadClient  nadclient.K8sCniCncfIoV1Interface
			virtClient *kubevirt.Client
		)

		BeforeEach(func() {
			cs = f.ClientSet

			var err error
			nadClient, err = nadclient.NewForConfig(f.ClientConfig())
			Expect(err).NotTo(HaveOccurred())
			virtClient, err = kubevirt.NewClient(oc, "/tmp/")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("with user defined networks and persistent ips configured", func() {
			const (
				nadName           = "blue"
				bindingName       = "passt"
				udnCrReadyTimeout = 5 * time.Second
				vmName            = "myvm"
			)
			var (
				cidrIPv4 = "203.203.0.0/16"
				cidrIPv6 = "2014:100:200::0/60"
			)

			DescribeTableSubtree("created using",
				func(createNetworkFn func(netConfig networkAttachmentConfigParams) networkAttachmentConfig) {

					DescribeTable("[Suite:openshift/network/virtualization] should keep ip", func(netConfig networkAttachmentConfigParams, vmResource string, opCmd func(cli *kubevirt.Client, vmNamespace, vmName string)) {
						var err error
						netConfig.namespace = f.Namespace.Name
						// correctCIDRFamily makes use of the ginkgo framework so it needs to be in the testcase
						netConfig.cidr = correctCIDRFamily(oc, cidrIPv4, cidrIPv6)
						workerNodes, err := getWorkerNodesOrdered(cs)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(workerNodes)).To(BeNumerically(">=", 2))

						isDualStack := getIPFamilyForCluster(f) == DualStack

						provisionedNetConfig := createNetworkFn(netConfig)

						for _, node := range workerNodes {
							Eventually(func() bool {
								isNetProvisioned, err := isNetworkProvisioned(oc, node.Name, provisionedNetConfig.networkName)
								return err == nil && isNetProvisioned
							}).WithPolling(time.Second).WithTimeout(udnCrReadyTimeout).Should(
								BeTrueBecause("the network must be ready before creating workloads"),
							)
						}

						httpServerPods := prepareHTTPServerPods(f, netConfig, workerNodes)
						vmCreationParams := kubevirt.CreationTemplateParams{
							VMName:                    vmName,
							VMNamespace:               f.Namespace.Name,
							FedoraContainterDiskImage: image.LocationFor("quay.io/kubevirt/fedora-with-test-tooling-container-disk:20241024_891122a6fc"),
						}
						if netConfig.role == "primary" {
							vmCreationParams.NetBindingName = bindingName
						} else {
							vmCreationParams.NetworkName = nadName
						}

						Expect(virtClient.CreateVM(vmResource, vmCreationParams)).To(Succeed())
						waitForVMReadiness(virtClient, vmCreationParams.VMNamespace, vmCreationParams.VMName)

						By("Retrieving addresses before test operation")
						initialAddresses := obtainAddresses(virtClient, netConfig, vmName)
						expectedNumberOfAddresses := 1
						if isDualStack {
							expectedNumberOfAddresses = 2
						}
						Expect(initialAddresses).To(HaveLen(expectedNumberOfAddresses))

						httpServerPodsIPs := httpServerTestPodsMultusNetworkIPs(netConfig, httpServerPods)

						By(fmt.Sprintf("Check east/west traffic before test operation using IPs: %v", httpServerPodsIPs))
						checkEastWestTraffic(virtClient, vmName, httpServerPodsIPs)

						opCmd(virtClient, f.Namespace.Name, vmName)

						By("Retrieving addresses after test operation")
						obtainedAddresses := obtainAddresses(virtClient, netConfig, vmName)
						Expect(obtainedAddresses).To(ConsistOf(initialAddresses))

						By("Check east/west after test operation")
						checkEastWestTraffic(virtClient, vmName, httpServerPodsIPs)
					},
						Entry(
							"when the VM attached to a primary UDN is migrated between nodes",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "primary",
								allowPersistentIPs: true,
							},
							kubevirt.FedoraVMWithPrimaryUDNAttachment,
							migrateVM,
						),
						Entry(
							"when the VMI attached to a primary UDN is migrated between nodes",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "primary",
								allowPersistentIPs: true,
							},
							kubevirt.FedoraVMIWithPrimaryUDNAttachment,
							migrateVM,
						),
						Entry(
							"when the VM attached to a primary UDN is restarted",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "primary",
								allowPersistentIPs: true,
							},
							kubevirt.FedoraVMWithPrimaryUDNAttachment,
							restartVM,
						),
						Entry(
							"when the VM attached to a secondary UDN is migrated between nodes",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "secondary",
								allowPersistentIPs: true,
							},
							kubevirt.FedoraVMWithSecondaryNetworkAttachment,
							migrateVM,
						),
						Entry(
							"when the VMI attached to a secondary UDN is migrated between nodes",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "secondary",
								allowPersistentIPs: true,
							},
							kubevirt.FedoraVMIWithSecondaryNetworkAttachment,
							migrateVM,
						),
						Entry(
							"when the VM attached to a secondary UDN is restarted",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "secondary",
								allowPersistentIPs: true,
							},
							kubevirt.FedoraVMWithSecondaryNetworkAttachment,
							restartVM,
						))
				},
				Entry("NetworkAttachmentDefinitions", func(c networkAttachmentConfigParams) networkAttachmentConfig {
					netConfig := newNetworkAttachmentConfig(c)
					nad := generateNAD(netConfig)
					By(fmt.Sprintf("Creating NetworkAttachmentDefinitions %s/%s", nad.Namespace, nad.Name))
					_, err := nadClient.NetworkAttachmentDefinitions(c.namespace).Create(context.Background(), nad, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())
					return netConfig
				}),
				Entry("UserDefinedNetwork", func(c networkAttachmentConfigParams) networkAttachmentConfig {
					udnManifest := generateUserDefinedNetworkManifest(&c)
					By(fmt.Sprintf("Creating UserDefinedNetwork %s/%s", c.namespace, c.name))
					Expect(applyManifest(c.namespace, udnManifest)).To(Succeed())
					Expect(waitForUserDefinedNetworkReady(c.namespace, c.name, udnCrReadyTimeout)).To(Succeed())

					nad, err := nadClient.NetworkAttachmentDefinitions(c.namespace).Get(
						context.Background(), c.name, metav1.GetOptions{},
					)
					Expect(err).NotTo(HaveOccurred())
					return networkAttachmentConfig{networkAttachmentConfigParams{networkName: networkName(nad.Spec.Config)}}
				}))
		})
	})
})

var _ = Describe("[sig-network][Feature:Layer2LiveMigration][OCPFeatureGate:NetworkSegmentation][Suite:openshift/network/virtualization] primary UDN smoke test", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("network-segmentation-e2e", admissionapi.LevelBaseline)
	f := oc.KubeFramework()

	const (
		nadName  = "blue"
		cidrIPv4 = "203.203.0.0/16"
		cidrIPv6 = "2014:100:200::0/60"
	)

	InOVNKubernetesContext(func() {
		var (
			cs        clientset.Interface
			nadClient nadclient.K8sCniCncfIoV1Interface
		)

		BeforeEach(func() {
			cs = f.ClientSet

			var err error
			nadClient, err = nadclient.NewForConfig(f.ClientConfig())
			Expect(err).NotTo(HaveOccurred())
		})

		It("assert the primary UDN feature works as expected", func() {
			netConfig := networkAttachmentConfigParams{
				name:               nadName,
				topology:           "layer2",
				role:               "primary",
				allowPersistentIPs: true,
				namespace:          f.Namespace.Name,
				cidr:               correctCIDRFamily(oc, cidrIPv4, cidrIPv6),
			}

			nad := generateNAD(newNetworkAttachmentConfig(netConfig))
			By(fmt.Sprintf("Creating NetworkAttachmentDefinitions %s/%s", nad.Namespace, nad.Name))
			_, err := nadClient.NetworkAttachmentDefinitions(f.Namespace.Name).Create(
				context.Background(),
				nad,
				metav1.CreateOptions{},
			)
			Expect(err).NotTo(HaveOccurred())

			workerNodes, err := getWorkerNodesOrdered(cs)
			Expect(err).NotTo(HaveOccurred())

			httpServerPods := prepareHTTPServerPods(f, netConfig, workerNodes)
			Expect(httpServerPods).NotTo(BeEmpty())
			Expect(podNetworkStatus(httpServerPods[0])).To(
				HaveLen(2),
				"the pod network status must feature both the cluster default network and the primary UDN attachment",
			)
		})
	})
})

type VirtualMachineInstanceConditionType string

const VirtualMachineInstanceConditionReady VirtualMachineInstanceConditionType = "Ready"

// [{"lastProbeTime":null,"lastTransitionTime":"2024-10-16T15:56:27Z","status":"True","type":"Ready"},{"lastProbeTime":null,"lastTransitionTime":null,"status":"True","type":"LiveMigratable"},{"lastProbeTime":null,"lastTransitionTime":null,"status":"True","type":"StorageLiveMigratable"},{"lastProbeTime":"2024-10-16T15:56:44Z","lastTransitionTime":null,"status":"True","type":"AgentConnected"}]
type VirtualMachineInstanceCondition struct {
	Type   VirtualMachineInstanceConditionType `json:"type"`
	Status corev1.ConditionStatus              `json:"status"`
	// +nullable
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
	// +nullable
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	Reason             string      `json:"reason,omitempty"`
	Message            string      `json:"message,omitempty"`
}

func waitForVMReadiness(vmClient *kubevirt.Client, namespace, vmName string) {
	By(fmt.Sprintf("Waiting for readiness at virtual machine %s/%s", namespace, vmName))
	Eventually(func(g Gomega) []VirtualMachineInstanceCondition {
		GinkgoHelper()
		vmConditionsStr, err := vmClient.GetJSONPath("vmi", vmName, "{@.status.conditions}")
		g.Expect(err).To(SatisfyAny(
			WithTransform(apierrors.IsNotFound, BeTrue()),
			Not(HaveOccurred()),
		))

		g.Expect(vmConditionsStr).NotTo(BeEmpty())

		By(fmt.Sprintf("The retrieved VM state: %s", vmConditionsStr))

		var vmConditions []VirtualMachineInstanceCondition
		g.Expect(json.Unmarshal([]byte(vmConditionsStr), &vmConditions)).To(Succeed(), "unmarshall VMI conditions")
		return vmConditions
	}).WithPolling(time.Second).WithTimeout(5 * time.Minute).Should(
		ContainElement(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(VirtualMachineInstanceConditionReady),
			"Status": Equal(corev1.ConditionTrue),
		})))

}

func waitForVMIMSuccess(vmClient *kubevirt.Client, namespace, vmName string) {
	By(fmt.Sprintf("Waiting for success at virtual machine instance migration %s/%s", namespace, vmName))
	Eventually(func(g Gomega) string {
		GinkgoHelper()
		migrationCompletedStr, err := vmClient.GetJSONPath("vmim", vmName, "{@.status.migrationState.completed}")
		g.Expect(err).To(SatisfyAny(
			WithTransform(apierrors.IsNotFound, BeTrue()),
			Not(HaveOccurred()),
		))

		g.Expect(migrationCompletedStr).NotTo(BeEmpty())

		return migrationCompletedStr
	}).WithPolling(time.Second).WithTimeout(5 * time.Minute).Should(Equal("true"))
	migrationFailedStr, err := vmClient.GetJSONPath("vmim", vmName, "{@.status.migrationState.failed}")
	Expect(err).NotTo(HaveOccurred())
	Expect(migrationFailedStr).To(BeEmpty())
}

func addressFromStatus(cli *kubevirt.Client, vmName string) []string {
	GinkgoHelper()
	addressesStr, err := cli.GetJSONPath("vmi", vmName, "{@.status.interfaces[0].ipAddresses}")
	Expect(err).NotTo(HaveOccurred())
	var addresses []string
	Expect(json.Unmarshal([]byte(addressesStr), &addresses)).To(Succeed())
	return addresses
}

func addressFromGuest(cli *kubevirt.Client, vmName string) []string {
	GinkgoHelper()
	Expect(cli.Login(vmName, vmName)).To(Succeed())
	output, err := cli.Console(vmName, "ip -j a show dev eth0")
	Expect(err).NotTo(HaveOccurred())
	// [{"ifindex":2,"ifname":"eth0","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1300,"qdisc":"fq_codel","operstate":"UP","group":"default","txqlen":1000,"link_type":"ether","address":"02:ba:c3:00:00:0a","broadcast":"ff:ff:ff:ff:ff:ff","altnames":["enp1s0"],"addr_info":[{"family":"inet","local":"100.10.0.1","prefixlen":24,"broadcast":"100.10.0.255","scope":"global","dynamic":true,"noprefixroute":true,"label":"eth0","valid_life_time":86313548,"preferred_life_time":86313548},{"family":"inet6","local":"fe80::ba:c3ff:fe00:a","prefixlen":64,"scope":"link","valid_life_time":4294967295,"preferred_life_time":4294967295}]}]
	type address struct {
		IP    string `json:"local,omitempty"`
		Scope string `json:"scope,omitempty"`
	}
	type iface struct {
		Name      string    `json:"ifname,omitempty"`
		Addresses []address `json:"addr_info,omitempty"`
	}
	ifaces := []iface{}
	Expect(json.Unmarshal([]byte(output), &ifaces)).To(Succeed())
	addresses := []string{}
	Expect(ifaces).NotTo(BeEmpty())
	for _, address := range ifaces[0].Addresses {
		if address.Scope == "link" {
			continue
		}
		addresses = append(addresses, address.IP)
	}
	return addresses
}

func obtainAddresses(virtClient *kubevirt.Client, netConfig networkAttachmentConfigParams, vmName string) []string {
	if netConfig.role == "primary" {
		return addressFromGuest(virtClient, vmName)
	}
	return addressFromStatus(virtClient, vmName)
}

func restartVM(cli *kubevirt.Client, vmNamespace, vmName string) {
	GinkgoHelper()
	By(fmt.Sprintf("Restarting vmi %s/%s", vmNamespace, vmName))
	Expect(cli.Restart(vmName)).To(Succeed())
	waitForVMReadiness(cli, vmNamespace, vmName)
}

func migrateVM(cli *kubevirt.Client, vmNamespace, vmName string) {
	GinkgoHelper()
	By(fmt.Sprintf("Migrating vmi %s/%s", vmNamespace, vmName))
	Expect(cli.CreateVMIM(vmName)).To(Succeed())
	waitForVMIMSuccess(cli, vmNamespace, vmName)
}

func waitForPodsCondition(fr *framework.Framework, pods []*corev1.Pod, conditionFn func(g Gomega, pod *corev1.Pod)) {
	for _, pod := range pods {
		Eventually(func(g Gomega) {
			var err error
			pod, err = fr.ClientSet.CoreV1().Pods(fr.Namespace.Name).Get(context.TODO(), pod.Name, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			conditionFn(g, pod)
		}).
			WithTimeout(time.Minute).
			WithPolling(time.Second).
			Should(Succeed())
	}
}

func generateAgnhostPod(name, namespace, nodeName string, args ...string) *corev1.Pod {
	agnHostPod := e2epod.NewAgnhostPod(namespace, name, nil, nil, nil, args...)
	agnHostPod.Spec.NodeName = nodeName
	return agnHostPod
}

func createHTTPServerPods(fr *framework.Framework, annotations map[string]string, selectedNodes []corev1.Node) []*corev1.Pod {
	var pods []*corev1.Pod
	for _, selectedNode := range selectedNodes {
		pod := generateAgnhostPod(
			"testpod-"+selectedNode.Name,
			fr.Namespace.Name,
			selectedNode.Name,
			"netexec", "--http-port", "8000")
		pod.Annotations = annotations
		pods = append(pods, e2epod.NewPodClient(fr).CreateSync(context.TODO(), pod))
	}
	return pods
}

func updatePods(fr *framework.Framework, pods []*corev1.Pod) []*corev1.Pod {
	for i, pod := range pods {
		var err error
		pod, err = fr.ClientSet.CoreV1().Pods(fr.Namespace.Name).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		pods[i] = pod
	}
	return pods
}

func podNetworkStatus(pod *v1.Pod, predicates ...func(nadapi.NetworkStatus) bool) ([]nadapi.NetworkStatus, error) {
	podNetStatus, found := pod.Annotations[nadapi.NetworkStatusAnnot]
	if !found {
		return nil, fmt.Errorf("the pod must feature the `networks-status` annotation")
	}

	var netStatus []nadapi.NetworkStatus
	if err := json.Unmarshal([]byte(podNetStatus), &netStatus); err != nil {
		return nil, err
	}

	if len(predicates) == 0 {
		return netStatus, nil
	}
	var netStatusMeetingPredicates []nadapi.NetworkStatus
	for i := range netStatus {
		for _, predicate := range predicates {
			if predicate(netStatus[i]) {
				netStatusMeetingPredicates = append(netStatusMeetingPredicates, netStatus[i])
				continue
			}
		}
	}
	return netStatusMeetingPredicates, nil
}
func checkPodRunningReady() func(Gomega, *corev1.Pod) {
	return func(g Gomega, pod *corev1.Pod) {
		ok, err := testutils.PodRunningReady(pod)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(ok).To(BeTrue())
	}
}

func checkPodHasIPsAtNetwork(netName string, expectedNumberOfAddresses int) func(Gomega, *corev1.Pod) {
	return func(g Gomega, pod *corev1.Pod) {
		GinkgoHelper()
		By(fmt.Sprintf("Checking pod annotations: %+v", pod.Annotations))
		netStatus, err := podNetworkStatus(pod, func(status nadapi.NetworkStatus) bool {
			return status.Name == netName
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(netStatus).To(HaveLen(1))
		g.Expect(netStatus[0].IPs).To(HaveLen(expectedNumberOfAddresses))
	}
}

func prepareHTTPServerPods(fr *framework.Framework, netConfig networkAttachmentConfigParams, selectedNodes []corev1.Node) []*corev1.Pod {
	By("Preparing HTTP server pods")
	httpServerPodsAnnotations := map[string]string{}
	if netConfig.role != "primary" {
		httpServerPodsAnnotations["k8s.v1.cni.cncf.io/networks"] = fmt.Sprintf(`[{"namespace": %q, "name": %q}]`, netConfig.namespace, netConfig.name)
	}
	var httpServerPodCondition func(Gomega, *corev1.Pod)
	expectedNumberOfAddresses := len(strings.Split(netConfig.cidr, ","))
	if netConfig.role != "primary" {
		httpServerPodCondition = checkPodHasIPsAtNetwork(fmt.Sprintf("%s/%s", netConfig.namespace, netConfig.name), expectedNumberOfAddresses)
	} else {
		httpServerPodCondition = checkPodRunningReady()
	}

	httpServerTestPods := createHTTPServerPods(fr, httpServerPodsAnnotations, selectedNodes)
	waitForPodsCondition(fr, httpServerTestPods, httpServerPodCondition)
	return updatePods(fr, httpServerTestPods)
}
func httpServerTestPodsMultusNetworkIPs(netConfig networkAttachmentConfigParams, httpServerTestPods []*corev1.Pod) map[string][]string {
	GinkgoHelper()
	ips := map[string][]string{}
	for _, pod := range httpServerTestPods {
		var ovnPodAnnotation *PodAnnotation
		Eventually(func() (*PodAnnotation, error) {
			var err error
			ovnPodAnnotation, err = unmarshalPodAnnotation(pod.Annotations, fmt.Sprintf("%s/%s", netConfig.namespace, netConfig.name))
			return ovnPodAnnotation, err
		}).
			WithTimeout(5 * time.Second).
			WithPolling(200 * time.Millisecond).
			ShouldNot(BeNil())
		for _, ipnet := range ovnPodAnnotation.IPs {
			ips[pod.Name] = append(ips[pod.Name], ipnet.IP.String())
		}
	}
	return ips

}

func checkEastWestTraffic(virtClient *kubevirt.Client, vmiName string, podIPsByName map[string][]string) {
	GinkgoHelper()
	Expect(virtClient.Login(vmiName, vmiName)).To(Succeed())
	polling := 15 * time.Second
	timeout := time.Minute
	for podName, podIPs := range podIPsByName {
		for _, podIP := range podIPs {
			output := ""
			Eventually(func() error {
				var err error
				output, err = virtClient.Console(vmiName, fmt.Sprintf("curl http://%s", net.JoinHostPort(podIP, "8000")))
				return err
			}).
				WithPolling(polling).
				WithTimeout(timeout).
				Should(Succeed(), func() string { return podName + ": " + output })
		}
	}
}

func isNetworkProvisioned(oc *exutil.CLI, nodeName string, networkName string) (bool, error) {
	ovnkubePodInfo, err := ovnkubePod(oc, nodeName)
	if err != nil {
		return false, err
	}

	lsName := logicalSwitchName(networkName)
	out, err := adminExecInPod(
		oc,
		"openshift-ovn-kubernetes",
		ovnkubePodInfo.podName,
		ovnkubePodInfo.containerName,
		fmt.Sprintf("ovn-nbctl list logical-switch %s", lsName),
	)
	if err != nil {
		return false, fmt.Errorf("failed to find a logical switch for network %q: %w", networkName, err)
	}

	return strings.Contains(out, lsName), nil
}

func logicalSwitchName(networkName string) string {
	netName := strings.ReplaceAll(networkName, "-", ".")
	netName = strings.ReplaceAll(netName, "/", ".")
	return fmt.Sprintf("%s_ovn_layer2_switch", netName)
}

func networkName(netSpecConfig string) string {
	GinkgoHelper()
	type netConfig struct {
		Name string `json:"name,omitempty"`
	}
	var nc netConfig
	Expect(json.Unmarshal([]byte(netSpecConfig), &nc)).To(Succeed())
	return nc.Name
}
