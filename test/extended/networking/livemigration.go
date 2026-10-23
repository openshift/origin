package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	testutils "k8s.io/kubernetes/test/utils"
	admissionapi "k8s.io/pod-security-admission/api"

	nadapi "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"

	"github.com/openshift/origin/test/extended/networking/kubevirt"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	utilnet "k8s.io/utils/net"
)

const kvIPRequestsAnnot = "network.kubevirt.io/addresses"

var _ = Describe("[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines", func() {
	// disable automatic namespace creation, we need to add the required UDN label
	oc := exutil.NewCLIWithoutNamespace("network-segmentation-e2e")
	f := oc.KubeFramework()
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

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
				bindingName       = "l2bridge"
				udnCrReadyTimeout = 60 * time.Second
				// TODO(trozet): lower this timeout once https://issues.redhat.com/browse/OCPBUGS-49727 is fixed
				udnNetworkReadyTimeout = 5 * time.Minute
				vmName                 = "myvm"
			)
			var (
				cidrIPv4 = "203.203.0.0/16"
				cidrIPv6 = "2014:100:200::0/60"
			)

			DescribeTableSubtree("created using",
				func(createNetworkFn func(netConfig networkAttachmentConfigParams)) {

					DescribeTable("[Suite:openshift/network/virtualization] should keep ip", func(netConfig networkAttachmentConfigParams, vmResource string, opCmd func(cli *kubevirt.Client, vmNamespace, vmName string), wlConfig ...workloadNetworkConfig) {
						var err error
						var workloadConfig workloadNetworkConfig
						if len(wlConfig) > 0 {
							workloadConfig = wlConfig[0]
						}
						l := map[string]string{
							"e2e-framework": f.BaseName,
						}
						if netConfig.role == "primary" {
							l[RequiredUDNNamespaceLabel] = ""
						}
						ns, err := f.CreateNamespace(context.TODO(), f.BaseName, l)
						Expect(err).NotTo(HaveOccurred())
						err = udnWaitForOpenShift(oc, ns.Name)
						Expect(err).NotTo(HaveOccurred())

						f.Namespace = ns
						netConfig.namespace = f.Namespace.Name

						// IPv6 only supported for primaries yet, the
						// kubevirt bridge binding dhcp server do not
						// support it.
						if netConfig.role == "primary" {
							// correctCIDRFamily makes use of the ginkgo framework so it needs to be in the testcase
							netConfig.cidr = correctCIDRFamily(oc, cidrIPv4, cidrIPv6)
						} else {
							netConfig.cidr = cidrIPv4
						}
						workerNodes, err := getWorkerNodesOrdered(cs)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(workerNodes)).To(BeNumerically(">=", 2))

						isDualStack := false
						if strings.Contains(netConfig.cidr, ",") {
							isDualStack = true
						}

						createNetworkFn(netConfig)

						httpServerPods := prepareHTTPServerPods(f, netConfig, workerNodes)
						hasIPv4, hasIPv6, err := GetIPAddressFamily(oc)
						Expect(err).NotTo(HaveOccurred())
						// Cloud-init 23.2.1 in this guest image makes each requested IP family
						// mandatory in NetworkManager. Requesting IPv6 on an IPv4-only network
						// also tears down the IPv4 address.
						vmCreationParams := kubevirt.CreationTemplateParams{
							VMName:                    vmName,
							VMNamespace:               f.Namespace.Name,
							FedoraContainterDiskImage: image.LocationFor("quay.io/kubevirt/fedora-with-test-tooling-container-disk:v1.8.2"),
							DHCPv4:                    netConfig.role != "primary" || hasIPv4,
							DHCPv6:                    netConfig.role == "primary" && hasIPv6,
						}

						if netConfig.role == "primary" {
							vmCreationParams.NetBindingName = bindingName
						} else {
							vmCreationParams.NetworkName = nadName
						}

						var filteredPreconfiguredIPs []string
						for _, ip := range workloadConfig.preconfiguredIPs {
							if utilnet.IsIPv4String(ip) && hasIPv4 {
								filteredPreconfiguredIPs = append(filteredPreconfiguredIPs, ip)
							}
							if utilnet.IsIPv6String(ip) && hasIPv6 {
								filteredPreconfiguredIPs = append(filteredPreconfiguredIPs, ip)
							}
						}

						if len(filteredPreconfiguredIPs) > 0 {
							var err error
							vmCreationParams.PreconfiguredIP, err = formatAddressesAnnotation(filteredPreconfiguredIPs)
							Expect(err).NotTo(HaveOccurred())
						}
						if workloadConfig.preconfiguredMAC != "" {
							vmCreationParams.PreconfiguredMAC = workloadConfig.preconfiguredMAC
						}
						Expect(virtClient.CreateVM(vmResource, vmCreationParams)).To(Succeed())
						waitForVMReadiness(virtClient, vmCreationParams.VMNamespace, vmCreationParams.VMName)

						By("Retrieving addresses before test operation")
						var initialAddresses []string
						Eventually(func(g Gomega) []string {
							GinkgoHelper()

							var err error
							initialAddresses, err = obtainAddresses(virtClient, vmName)
							g.Expect(err).NotTo(HaveOccurred(), "Failed to obtain IP addresses for VM")
							return initialAddresses
						}).
							WithPolling(time.Second).
							WithTimeout(5 * time.Minute).
							ShouldNot(BeEmpty())

						expectedNumberOfAddresses := 1
						if isDualStack {
							expectedNumberOfAddresses = 2
						}
						Expect(initialAddresses).To(HaveLen(expectedNumberOfAddresses))

						if len(filteredPreconfiguredIPs) > 0 {
							By("Verifying VM received the preconfigured IP address(es)")
							for _, expectedIP := range filteredPreconfiguredIPs {
								expectedIP = strings.TrimSpace(expectedIP)
								Expect(initialAddresses).To(ContainElement(expectedIP), fmt.Sprintf("Expected IP %s not found in VM addresses %v", expectedIP, initialAddresses))
							}
						}
						if workloadConfig.preconfiguredMAC != "" {
							By("Verifying VM received the preconfigured MAC address")
							verifyVMMAC(virtClient, vmName, workloadConfig.preconfiguredMAC)
						}
						httpServerPodsIPs := httpServerTestPodsMultusNetworkIPs(netConfig, httpServerPods)

						By(fmt.Sprintf("Check east/west traffic before test operation using IPs: %v", httpServerPodsIPs))
						checkEastWestTraffic(virtClient, vmName, httpServerPodsIPs)

						opCmd(virtClient, f.Namespace.Name, vmName)

						By("Retrieving addresses after test operation")
						var obtainedAddresses []string
						Eventually(func(g Gomega) []string {
							GinkgoHelper()

							var err error
							obtainedAddresses, err = obtainAddresses(virtClient, vmName)
							g.Expect(err).NotTo(HaveOccurred(), "Failed to obtain IP addresses for VM after the migrate or restart operation")
							return obtainedAddresses
						}).
							WithPolling(time.Second).
							WithTimeout(5 * time.Minute).
							ShouldNot(BeEmpty())
						Expect(obtainedAddresses).To(ConsistOf(initialAddresses))

						if workloadConfig.preconfiguredMAC != "" {
							By("Verifying VM MAC address persisted after test operation")
							verifyVMMAC(virtClient, vmName, workloadConfig.preconfiguredMAC)
						}
						By("Check east/west after test operation")
						checkEastWestTraffic(virtClient, vmName, httpServerPodsIPs)
					},
						Entry(
							"[OCPFeatureGate:NetworkSegmentation] when the VM attached to a primary UDN is migrated between nodes",
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
							"[OCPFeatureGate:NetworkSegmentation] when the VMI attached to a primary UDN is migrated between nodes",
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
							"[OCPFeatureGate:NetworkSegmentation] when the VM attached to a primary UDN is restarted",
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
						),
						Entry(
							"[OCPFeatureGate:PreconfiguredUDNAddresses] when the VM with preconfigured IPs attached to a primary UDN is restarted",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "primary",
								allowPersistentIPs: true,
							},
							kubevirt.FedoraVMWithPreconfiguredPrimaryUDNAttachment,
							restartVM,
							workloadNetworkConfig{
								preconfiguredIPs: []string{"203.203.0.50", "2014:100:200::50"},
							},
						),
						Entry(
							"[OCPFeatureGate:PreconfiguredUDNAddresses] when the VM with preconfigured MAC attached to a primary UDN is restarted",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "primary",
								allowPersistentIPs: true,
							},
							kubevirt.FedoraVMWithPreconfiguredPrimaryUDNAttachment,
							restartVM,
							workloadNetworkConfig{
								preconfiguredMAC: "02:0A:0B:0C:0D:50",
							},
						),
						Entry(
							"[OCPFeatureGate:PreconfiguredUDNAddresses] when the VM with preconfigured IP and MAC attached to a primary UDN is migrated between nodes",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "primary",
								allowPersistentIPs: true,
							},
							kubevirt.FedoraVMWithPreconfiguredPrimaryUDNAttachment,
							migrateVM,
							workloadNetworkConfig{
								preconfiguredIPs: []string{"203.203.0.51", "2014:100:200::51"},
								preconfiguredMAC: "02:0A:0B:0C:0D:51",
							},
						),
						Entry(
							"[OCPFeatureGate:PreconfiguredUDNAddresses] when the VM with preconfigured IP address is created when the address is already taken",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "primary",
								allowPersistentIPs: true,
							},
							kubevirt.FedoraVMWithPreconfiguredPrimaryUDNAttachment,
							duplicateVM,
							workloadNetworkConfig{
								preconfiguredIPs: []string{"203.203.0.100", "2014:100:200::100"},
							},
						),
						Entry(
							"[OCPFeatureGate:PreconfiguredUDNAddresses] when the VM with preconfigured MAC address is created when the address is already taken",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "primary",
								allowPersistentIPs: true,
							},
							kubevirt.FedoraVMWithPreconfiguredPrimaryUDNAttachment,
							duplicateVM,
							workloadNetworkConfig{
								preconfiguredMAC: "02:00:00:22:22:22",
							},
						),
					)
				},
				Entry("NetworkAttachmentDefinitions", func(c networkAttachmentConfigParams) {
					netConfig := newNetworkAttachmentConfig(c)
					nad := generateNAD(netConfig)
					By(fmt.Sprintf("Creating NetworkAttachmentDefinitions %s/%s", nad.Namespace, nad.Name))
					_, err := nadClient.NetworkAttachmentDefinitions(c.namespace).Create(context.Background(), nad, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())
				}),
				Entry("[OCPFeatureGate:NetworkSegmentation] UserDefinedNetwork", func(c networkAttachmentConfigParams) {
					udnManifest := generateUserDefinedNetworkManifest(&c)
					By(fmt.Sprintf("Creating UserDefinedNetwork %s/%s", c.namespace, c.name))
					Expect(applyManifest(c.namespace, udnManifest)).To(Succeed())
					Eventually(userDefinedNetworkReadyFunc(oc.AdminDynamicClient(), c.namespace, c.name), udnCrReadyTimeout, time.Second).Should(Succeed())

					_, err := nadClient.NetworkAttachmentDefinitions(c.namespace).Get(
						context.Background(), c.name, metav1.GetOptions{},
					)
					Expect(err).NotTo(HaveOccurred())
				}))
		})
	})
})

var _ = Describe("[sig-network][Feature:Layer2LiveMigration][OCPFeatureGate:NetworkSegmentation][apigroup:route.openshift.io][Suite:openshift/network/virtualization] primary UDN migration egress", func() {
	oc := exutil.NewCLIWithoutNamespace("udn-migration-egress")
	f := oc.KubeFramework()
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	InOVNKubernetesContext(func() {
		It("should consistently egress through each VM's own node while another VM migrates", func() {
			ctx := context.Background()
			hasIPv4, hasIPv6, err := GetIPAddressFamily(oc)
			Expect(err).NotTo(HaveOccurred(), "discover the cluster's IP families")
			var families []int
			if hasIPv4 {
				families = append(families, 4)
			}
			if hasIPv6 {
				families = append(families, 6)
			}
			nodes, err := e2enode.GetBoundedReadySchedulableNodes(ctx, f.ClientSet, 2)
			Expect(err).NotTo(HaveOccurred(), "select nodes for live migration")
			if len(nodes.Items) < 2 {
				Skip(fmt.Sprintf("live migration requires two ready, schedulable nodes; found %d", len(nodes.Items)))
			}
			nodeNames := []string{nodes.Items[0].Name, nodes.Items[1].Name}

			By("Creating a primary Layer 2 UDN with persistent IPs")
			oc.CreateNamespaceUDN()
			namespace := oc.Namespace()
			Expect(udnWaitForOpenShift(oc, namespace)).To(Succeed(), "wait for UDN namespace %s", namespace)
			netConfig := networkAttachmentConfigParams{
				name: "blue", namespace: namespace, topology: "layer2", role: "primary", allowPersistentIPs: true,
				// Reuse this file's existing migration-test subnets; each test has its own isolated UDN.
				cidr: correctCIDRFamily(oc, "203.203.0.0/16", "2014:100:200::0/60"),
			}
			Expect(applyManifest(namespace, generateUserDefinedNetworkManifest(&netConfig))).To(Succeed(), "create UDN %s/%s", namespace, netConfig.name)
			Eventually(userDefinedNetworkReadyFunc(oc.AdminDynamicClient(), namespace, netConfig.name)).
				WithTimeout(time.Minute).WithPolling(time.Second).Should(Succeed(), "wait for UDN %s/%s", namespace, netConfig.name)

			By("Creating the test route and source-IP sniffers")
			helperNS, err := f.CreateNamespace(ctx, f.BaseName+"-captures", nil)
			Expect(err).NotTo(HaveOccurred(), "create capture namespace")
			ingressDomain, err := getIngressDomain(oc)
			Expect(err).NotTo(HaveOccurred(), "discover the ingress domain")
			var hosts []string
			for _, node := range nodes.Items {
				hosts = append(hosts, node.Labels[corev1.LabelHostname])
			}
			deployment, routeHost, err := createAgnhostDeploymentAndIngressRoute(oc, helperNS.Name, "", ingressDomain, 1, hosts)
			Expect(err).NotTo(HaveOccurred(), "create the agnhost test route")
			Expect(waitForDeploymentComplete(oc, helperNS.Name, deployment)).To(Succeed(), "wait for the route backend")
			sniffer, err := startVMEgressCaptures(ctx, oc, helperNS.Name, nodes.Items)
			Expect(err).NotTo(HaveOccurred(), "start packet sniffers on %v", nodeNames)

			virtClient, err := kubevirt.NewClient(oc, GinkgoT().TempDir())
			Expect(err).NotTo(HaveOccurred(), "initialize the VM client")
			params := kubevirt.CreationTemplateParams{
				VMName: "migrating", VMNamespace: namespace, NetBindingName: "l2bridge",
				FedoraContainterDiskImage: image.LocationFor("quay.io/kubevirt/fedora-with-test-tooling-container-disk:v1.8.2"),
				DHCPv4:                    hasIPv4, DHCPv6: hasIPv6, NodeNames: nodeNames,
			}
			Expect(virtClient.CreateVM(kubevirt.FedoraVMWithPrimaryUDNAttachment, params)).To(Succeed(), "create migrating VM")
			waitForVMReadiness(virtClient, namespace, params.VMName)
			sourceNode, err := virtClient.GetJSONPath("vmi", params.VMName, "{.status.nodeName}")
			Expect(err).NotTo(HaveOccurred(), "read migrating VM's initial node")
			params.VMName, params.NodeNames = "stationary", []string{sourceNode}
			Expect(virtClient.CreateVM(kubevirt.FedoraVMWithPrimaryUDNAttachment, params)).To(Succeed(), "create stationary VM on %s", sourceNode)
			waitForVMReadiness(virtClient, namespace, params.VMName)

			checkEgress := func(vm, nodeName string) error {
				node, err := f.ClientSet.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("read VM %s's node %s: %w", vm, nodeName, err)
				}
				for _, family := range families {
					if err := checkVMEgress(ctx, oc, virtClient, vm, node, routeHost, family, sniffer); err != nil {
						return err
					}
				}
				return nil
			}

			By("Checking both VMs' external connectivity and egress node before migration")
			Eventually(func() error {
				if err := checkEgress("migrating", sourceNode); err != nil {
					return err
				}
				return checkEgress("stationary", sourceNode)
			}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Succeed(), "wait for both VMs to reach the route with their node's source IP")

			By("Checking the stationary VM's egress while the other VM migrates")
			Expect(virtClient.CreateVMIM("migrating")).To(Succeed(), "start VM migration")
			Eventually(func() string {
				Expect(checkEgress("stationary", sourceNode)).To(Succeed(), "stationary VM must keep its node-local egress during migration")
				completed, err := virtClient.GetJSONPath("vmim", "migrating", "{.status.migrationState.completed}")
				Expect(err).NotTo(HaveOccurred(), "read migration completion status")
				failed, err := virtClient.GetJSONPath("vmim", "migrating", "{.status.migrationState.failed}")
				Expect(err).NotTo(HaveOccurred(), "read migration failure status")
				Expect(failed).NotTo(Equal("true"), "VM migration must not fail")
				return completed
			}).WithTimeout(5*time.Minute).WithPolling(time.Second).Should(Equal("true"), "wait for VM migration")
			targetNode, err := virtClient.GetJSONPath("vmi", "migrating", "{.status.nodeName}")
			Expect(err).NotTo(HaveOccurred(), "read migrated VM's node")
			Expect(targetNode).NotTo(Equal(sourceNode), "VM must migrate to the other node")

			By("Consistently checking both VMs' external connectivity and node-local egress after migration")
			Consistently(func() error {
				if err := checkEgress("stationary", sourceNode); err != nil {
					return err
				}
				return checkEgress("migrating", targetNode)
			}).WithTimeout(30*time.Second).WithPolling(time.Second).Should(Succeed(), "both VMs must retain node-local egress after migration")
		})
	})
})

var _ = Describe("[sig-network][Feature:Layer2LiveMigration][OCPFeatureGate:NetworkSegmentation][Suite:openshift/network/virtualization] primary UDN smoke test", func() {
	// disable automatic namespace creation, we need to add the required UDN label
	oc := exutil.NewCLIWithoutNamespace("network-segmentation-e2e")
	f := oc.KubeFramework()
	f.NamespacePodSecurityLevel = admissionapi.LevelBaseline

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

			ns, err := f.CreateNamespace(context.TODO(), f.BaseName, map[string]string{
				"e2e-framework":           f.BaseName,
				RequiredUDNNamespaceLabel: "",
			})
			err = udnWaitForOpenShift(oc, ns.Name)
			Expect(err).NotTo(HaveOccurred())
			f.Namespace = ns
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

var _ = Describe("[sig-network][Feature:Layer2LiveMigration][Suite:openshift/network/virtualization] Kubevirt Virtual Machines", func() {
	It("Placeholder test for GA", func() {
		Expect(1).To(Equal(1)) // we just need a test to run to ensure the platform comes up correctly
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

func addressFromStatus(cli *kubevirt.Client, vmName string) ([]string, error) {
	var addresses []string
	addressesStr, err := cli.GetJSONPath("vmi", vmName, "{@.status.interfaces[0].ipAddresses}")
	if err != nil {
		return nil, fmt.Errorf("failed to extract the IP addresses from VM %q: %w", vmName, err)
	}

	if addressesStr == "" {
		return nil, nil
	}

	if err := json.Unmarshal([]byte(addressesStr), &addresses); err != nil {
		return nil, fmt.Errorf("failed to unmarshal addresses %q: %w", addressesStr, err)
	}

	// Filter out IPv6 link-local addresses using net.IP.IsLinkLocalUnicast
	var filteredAddresses []string
	for _, addr := range addresses {
		ip := net.ParseIP(addr)
		if ip != nil && !ip.IsLinkLocalUnicast() {
			filteredAddresses = append(filteredAddresses, addr)
		}
	}

	return filteredAddresses, nil
}

func obtainAddresses(virtClient *kubevirt.Client, vmName string) ([]string, error) {
	return addressFromStatus(virtClient, vmName)
}

func obtainMAC(virtClient *kubevirt.Client, vmName string) (string, error) {
	macStr, err := virtClient.GetJSONPath("vmi", vmName, "{@.status.interfaces[0].mac}")
	if err != nil {
		return "", fmt.Errorf("failed to extract the MAC address from VM %q: %w", vmName, err)
	}
	return strings.ToUpper(macStr), nil
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

func verifyVMMAC(virtClient *kubevirt.Client, vmName, expectedMAC string) {
	GinkgoHelper()
	var actualMAC string
	Eventually(func(g Gomega) string {
		GinkgoHelper()

		var err error
		actualMAC, err = obtainMAC(virtClient, vmName)
		g.Expect(err).NotTo(HaveOccurred(), "Failed to obtain MAC address for VM")
		return actualMAC
	}).
		WithPolling(time.Second).
		WithTimeout(5 * time.Minute).
		Should(Equal(expectedMAC))
}

func duplicateVM(cli *kubevirt.Client, vmNamespace, vmName string) {
	GinkgoHelper()
	duplicateVMName := vmName + "-duplicate"
	By(fmt.Sprintf("Duplicating VM %s/%s to %s/%s", vmNamespace, vmName, vmNamespace, duplicateVMName))

	vmiSpecJSON, err := cli.GetJSONPath("vmi", vmName, "{.spec}")
	Expect(err).NotTo(HaveOccurred())
	var vmiSpec map[string]interface{}
	Expect(json.Unmarshal([]byte(vmiSpecJSON), &vmiSpec)).To(Succeed())

	originalVMIRawAnnotations, err := cli.GetJSONPath("vmi", vmName, "{.metadata.annotations}")
	Expect(err).NotTo(HaveOccurred())

	originalVMIAnnotations := map[string]string{}
	Expect(json.Unmarshal([]byte(originalVMIRawAnnotations), &originalVMIAnnotations)).To(Succeed())

	var vmiCreationOptions []kubevirt.Option
	var vmiExpectations []func()
	if requestedIPs, hasIPRequests := originalVMIAnnotations[kvIPRequestsAnnot]; hasIPRequests {
		vmiCreationOptions = append(
			vmiCreationOptions,
			kubevirt.WithAnnotations(ipRequests(requestedIPs)),
		)
		vmiExpectations = append(vmiExpectations, func() {
			waitForVMPodEventWithMessage(
				cli,
				vmNamespace,
				duplicateVMName,
				"IP is already allocated",
				2*time.Minute,
			)
		})
	}

	mac, err := cli.GetJSONPath("vmi", vmName, "{.spec.domain.devices.interfaces[0].macAddress}")
	Expect(err).NotTo(HaveOccurred())
	if len(mac) > 0 {
		vmiExpectations = append(vmiExpectations, func() {
			waitForVMPodEventWithMessage(
				cli,
				vmNamespace,
				duplicateVMName,
				"MAC address already in use",
				2*time.Minute,
			)
		})
	}

	Expect(cli.CreateVMIFromSpec(vmNamespace, duplicateVMName, vmiSpec, vmiCreationOptions...)).To(Succeed())
	for _, expectation := range vmiExpectations {
		expectation()
	}
}

func waitForVMPodEventWithMessage(vmClient *kubevirt.Client, vmNamespace, vmName, expectedEventMessage string, timeout time.Duration) {
	GinkgoHelper()
	By(fmt.Sprintf("Waiting for event containing %q on VM %s/%s virt-launcher pod", expectedEventMessage, vmNamespace, vmName))

	Eventually(func(g Gomega) []string {
		const vmLabelKey = "vm.kubevirt.io/name"
		podNames, err := vmClient.GetPodsByLabel(vmLabelKey, vmName)
		g.Expect(err).NotTo(HaveOccurred(), "Failed to get pods by label %s=%s", vmLabelKey, vmName)
		g.Expect(podNames).To(HaveLen(1), "Expected exactly one virt-launcher pod for VM %s/%s, but found %d pods: %v", vmNamespace, vmName, len(podNames), podNames)

		virtLauncherPodName := podNames[0]
		eventMessages, err := vmClient.GetEventsForPod(virtLauncherPodName)
		g.Expect(err).NotTo(HaveOccurred(), "Failed to get events for pod %s", virtLauncherPodName)

		return eventMessages
	}).WithPolling(time.Second).WithTimeout(timeout).Should(
		ContainElement(ContainSubstring(expectedEventMessage)),
		fmt.Sprintf("Expected to find an event containing %q", expectedEventMessage))
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

// formatAddressesAnnotation converts slice of IPs to the required JSON format for kubevirt addresses annotation
func formatAddressesAnnotation(preconfiguredIPs []string) (string, error) {
	const primaryUDNNetworkName = "overlay"
	if len(preconfiguredIPs) == 0 {
		return "", nil
	}

	ips := make([]string, len(preconfiguredIPs))
	for i, ip := range preconfiguredIPs {
		ips[i] = strings.TrimSpace(ip)
	}

	staticIPs, err := json.Marshal(map[string][]string{
		primaryUDNNetworkName: ips,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal static IPs: %w", err)
	}

	return string(staticIPs), nil
}

func ipRequests(ips string) map[string]string {
	return map[string]string{kvIPRequestsAnnot: ips}
}

func startVMEgressCaptures(ctx context.Context, oc *exutil.CLI, namespace string, nodes []corev1.Node) (*appsv1.DaemonSet, error) {
	if err := udnWaitForOpenShift(oc, namespace); err != nil {
		return nil, fmt.Errorf("wait for capture namespace %s: %w", namespace, err)
	}
	if _, err := oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "privileged", "-z", "default", "-n", namespace).Output(); err != nil {
		return nil, fmt.Errorf("allow captures in %s: %w", namespace, err)
	}
	var hosts []string
	for _, node := range nodes {
		hosts = append(hosts, node.Labels[corev1.LabelHostname])
	}
	// Include local ingress traffic as well as physical egress. Verification below
	// uses the post-SNAT node IP, so duplicate or pre-SNAT observations do not count.
	ds, err := createPacketSnifferDaemonSet(oc.AsAdmin(), namespace, hosts, "http", 80, "any")
	if err != nil {
		return nil, fmt.Errorf("create packet sniffers: %w", err)
	}
	var lastErr error
	err = wait.PollUntilContextTimeout(ctx, time.Second, time.Minute, true, func(context.Context) (bool, error) {
		logs, err := getDaemonSetLogs(oc.KubeFramework().ClientSet, ds)
		if err != nil {
			lastErr = err
			return false, nil
		}
		if len(logs) != len(nodes) {
			return false, nil
		}
		for _, node := range nodes {
			if !strings.Contains(logs[node.Name], "listening on") {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("wait for packet sniffers to listen: %w (last log error: %v)", err, lastErr)
	}
	return ds, nil
}

// Originate the request in the VM guest to exercise its cached gateway MAC,
// then check its hosting node's SNAT address in the shared packet captures.
func checkVMEgress(ctx context.Context, oc *exutil.CLI, client *kubevirt.Client, vm string, node *corev1.Node, routeHost string, family int, sniffer *appsv1.DaemonSet) error {
	ipFamily := corev1.IPv4Protocol
	if family == 6 {
		ipFamily = corev1.IPv6Protocol
	}
	addresses := e2enode.GetAddressesByTypeAndFamily(node, corev1.NodeInternalIP, ipFamily)
	if len(addresses) == 0 {
		return fmt.Errorf("VM %s's node %s has no IPv%d internal IP", vm, node.Name, family)
	}
	expectedIP := net.ParseIP(addresses[0]).String()
	if err := client.Login(vm, vm); err != nil {
		return fmt.Errorf("log in to VM %s: %w", vm, err)
	}
	marker := uuid.NewString()
	command := fmt.Sprintf("curl -%d --http1.1 --noproxy '*' --fail --silent --show-error --output /dev/null --connect-timeout 2 --max-time 3 'http://%s/?ovn-egress=%s'", family, routeHost, marker)
	if _, err := client.Console(vm, command); err != nil {
		return fmt.Errorf("VM %s IPv%d request to route %s: %w", vm, family, routeHost, err)
	}
	var lastErr error
	err := wait.PollUntilContextTimeout(ctx, time.Second, 10*time.Second, true, func(context.Context) (bool, error) {
		observed, err := scanPacketSnifferDaemonSetPodLogs(oc, sniffer, "http", marker)
		if err != nil {
			lastErr = err
			return false, nil
		}
		return observed[expectedIP] > 0, nil
	})
	if err != nil {
		return fmt.Errorf("verify VM %s IPv%d source IP is %s on node %s: %w (last scan error: %v)", vm, family, expectedIP, node.Name, err, lastErr)
	}
	return nil
}
