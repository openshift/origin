package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	admissionapi "k8s.io/pod-security-admission/api"

	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"

	"github.com/openshift/origin/test/extended/networking/kubevirt"
	exutil "github.com/openshift/origin/test/extended/util"
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
				func(createNetworkFn func(netConfig networkAttachmentConfigParams)) {

					DescribeTable("[Suite:openshift/network/virtualization] should keep ip", func(netConfig networkAttachmentConfigParams, vmResource string, opCmd func(cli *kubevirt.Client, vmNamespace, vmName string)) {
						var err error
						netConfig.namespace = f.Namespace.Name
						// correctCIDRFamily makes use of the ginkgo framework so it needs to be in the testcase
						netConfig.cidr = correctCIDRFamily(oc, cidrIPv4, cidrIPv6)
						workerNodes, err := getWorkerNodesOrdered(cs)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(workerNodes)).To(BeNumerically(">=", 2))

						isDualStack := getIPFamilyForCluster(f) == DualStack

						createNetworkFn(netConfig)

						vmCreationParams := kubevirt.CreationTemplateParams{
							VMName:                    vmName,
							VMNamespace:               f.Namespace.Name,
							FedoraContainterDiskImage: "quay.io/kubevirt/fedora-with-test-tooling-container-disk:20241024_891122a6fc",
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

						opCmd(virtClient, f.Namespace.Name, vmName)

						By("Retrieving addresses after test operation")
						obtainedAddresses := obtainAddresses(virtClient, netConfig, vmName)
						Expect(obtainedAddresses).To(ConsistOf(initialAddresses))
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
				Entry("NetworkAttachmentDefinitions", func(c networkAttachmentConfigParams) {
					netConfig := newNetworkAttachmentConfig(c)
					nad := generateNAD(netConfig)
					_, err := nadClient.NetworkAttachmentDefinitions(c.namespace).Create(context.Background(), nad, metav1.CreateOptions{})
					Expect(err).NotTo((HaveOccurred()))
				}),
				Entry("UserDefinedNetwork", func(c networkAttachmentConfigParams) {
					udnManifest := generateUserDefinedNetworkManifest(&c)
					Expect(applyManifest(c.namespace, udnManifest)).To(Succeed())
					Expect(waitForUserDefinedNetworkReady(c.namespace, c.name, udnCrReadyTimeout)).To(Succeed())
				}))
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
	Expect(err).NotTo((HaveOccurred()))
	Expect(migrationFailedStr).To(BeEmpty())
}

func addressFromStatus(cli *kubevirt.Client, vmName string) []string {
	GinkgoHelper()
	addressesStr, err := cli.GetJSONPath("vmi", vmName, "{@.status.interfaces[0].ipAddresses}")
	Expect(err).NotTo((HaveOccurred()))
	var addresses []string
	Expect(json.Unmarshal([]byte(addressesStr), &addresses)).To(Succeed())
	return addresses
}

func addressFromGuest(cli *kubevirt.Client, vmName string) []string {
	GinkgoHelper()
	Expect(cli.Login(vmName, vmName)).To(Succeed())
	output, err := cli.Console(vmName, "ip -j a show dev eth0")
	Expect(err).NotTo((HaveOccurred()))
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
	Expect(ifaces).NotTo((BeEmpty()))
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
