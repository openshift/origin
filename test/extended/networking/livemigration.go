package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	admissionapi "k8s.io/pod-security-admission/api"

	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = Describe("[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("network-segmentation-e2e", admissionapi.LevelBaseline)
	f := oc.KubeFramework()

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

		Context("with user defined networks and persistent ips configured", func() {
			const (
				nadName           = "blue"
				bindingName       = "passt"
				udnCrReadyTimeout = 5 * time.Second
				vmName            = "myvm"
			)
			var (
				//nad              *nadv1.NetworkAttachmentDefinition
				//vm               *kubevirtv1.VirtualMachine
				//vmi              *kubevirtv1.VirtualMachineInstance
				cidrIPv4 = "100.10.0.0/24"
				cidrIPv6 = "2010:100:200::0/60"
				//expectedAddreses []string
			)

			DescribeTableSubtree("created using",
				func(createNetworkFn func(netConfig networkAttachmentConfigParams) error) {

					DescribeTable("[Suite:openshift/network/virtualization] should keep ip", func(netConfig networkAttachmentConfigParams, resourceCmd func() string, opCmd func(cli *vmClient, vmNamespace, vmName string)) {
						var err error
						netConfig.namespace = f.Namespace.Name
						// correctCIDRFamily makes use of the ginkgo framework so it needs to be in the testcase
						netConfig.cidr = correctCIDRFamily(oc, cidrIPv4, cidrIPv6)
						workerNodes, err := getWorkerNodesOrdered(cs)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(workerNodes)).To(BeNumerically(">=", 2))

						Expect(createNetworkFn(netConfig)).To(Succeed())

						virtClient := newVirtClient(oc)

						var cmd string
						if netConfig.role == "primary" {
							cmd = fmt.Sprintf(resourceCmd(), vmName, f.Namespace.Name, bindingName)
						} else {
							cmd = fmt.Sprintf(resourceCmd(), vmName, f.Namespace.Name, nadName)
						}
						virtClient.createVM(f.Namespace.Name, vmName, cmd)
						virtClient.waitForVMReadiness(f.Namespace.Name, vmName)

						expectedAddresses := addressFromStatus(virtClient, f.Namespace.Name, vmName)

						opCmd(virtClient, f.Namespace.Name, vmName)

						obtainedAddresses := addressFromStatus(virtClient, f.Namespace.Name, vmName)
						Expect(obtainedAddresses).To(Equal(expectedAddresses))

					},
						XEntry(
							"when the VM attached to a primary UDN is migrated between nodes",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "primary",
								allowPersistentIPs: true,
							},
							fedoraVMWithPrimaryUDNAttachment,
							migrateVM,
						),
						XEntry(
							"when the VMI attached to a primary UDN is migrated between nodes",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "primary",
								allowPersistentIPs: true,
							},
							fedoraVMIWithPrimaryUDNAttachment,
							migrateVM,
						),
						XEntry(
							"when the VM attached to a primary UDN is restarted",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "primary",
								allowPersistentIPs: true,
							},
							fedoraVMWithPrimaryUDNAttachment,
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
							fedoraVMWithSecondaryNetworkAttachment,
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
							fedoraVMIWithSecondaryNetworkAttachment,
							migrateVM,
						),
						XEntry(
							"when the VM attached to a secondary UDN is restarted",
							networkAttachmentConfigParams{
								name:               nadName,
								topology:           "layer2",
								role:               "secondary",
								allowPersistentIPs: true,
							},
							fedoraVMWithSecondaryNetworkAttachment,
							restartVM,
						))
				},
				Entry("NetworkAttachmentDefinitions", func(c networkAttachmentConfigParams) error {
					netConfig := newNetworkAttachmentConfig(c)
					nad := generateNAD(netConfig)
					_, err := nadClient.NetworkAttachmentDefinitions(c.namespace).Create(context.Background(), nad, metav1.CreateOptions{})
					return err
				}),
				XEntry("UserDefinedNetwork", func(c networkAttachmentConfigParams) error {
					udnManifest := generateUserDefinedNetworkManifest(&c)
					cleanup, err := createManifest(c.namespace, udnManifest)
					DeferCleanup(cleanup)
					Expect(waitForUserDefinedNetworkReady(c.namespace, c.name, udnCrReadyTimeout)).To(Succeed())
					return err
				}))
		})
	})
})

type vmClient struct {
	oc *exutil.CLI
}

func newVirtClient(oc *exutil.CLI) *vmClient {
	return &vmClient{oc: oc}
}

func (v *vmClient) apply(description, resource string) {
	GinkgoHelper()
	By(fmt.Sprintf("Create %s", description))
	retries := 0
	Eventually(func() error {
		if retries > 0 {
			// retry due to unknown issue where kubevirt webhook gets stuck reading the request body
			// https://github.com/ovn-org/ovn-kubernetes/issues/3902#issuecomment-1750257559
			By(fmt.Sprintf("Retrying %s creation", description))
		}
		err := v.oc.Run("apply").Args("-f", "-").InputString(resource).Execute()
		retries++
		return err
	}).WithPolling(time.Second).WithTimeout(time.Minute).Should(Succeed())
}

func (v *vmClient) createVM(namespace, vmiName, createVMCmd string) {
	GinkgoHelper()
	v.apply(fmt.Sprintf("virtual machine %s/%s", namespace, vmiName), createVMCmd)
}

func (v *vmClient) createVMIM(namespace, vmiName string) {
	GinkgoHelper()
	vmim := fmt.Sprintf(`
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstanceMigration
metadata:
  namespace: %[1]s
  name: %[2]s
spec:
  vmiName: %[2]s
`, namespace, vmiName)
	v.apply(fmt.Sprintf("virtual machine instance migration %s/%s", namespace, vmiName), vmim)
}

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

func (v *vmClient) getJSONPath(resource, name, jsonPath string) (string, error) {
	output, err := v.oc.Run("get").Args(resource, name, "-o", fmt.Sprintf(`jsonpath=%q`, jsonPath)).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimPrefix(output, `"`), `"`), nil
}
func (v *vmClient) waitForVMReadiness(namespace, vmName string) {
	By(fmt.Sprintf("Waiting for readiness at virtual machine %s/%s", namespace, vmName))
	Eventually(func(g Gomega) []VirtualMachineInstanceCondition {
		GinkgoHelper()
		vmConditionsStr, err := v.getJSONPath("vmi", vmName, "{@.status.conditions}")
		g.Expect(err).To(SatisfyAny(
			WithTransform(errors.IsNotFound, BeTrue()),
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

func (v *vmClient) waitForVMIMSuccess(namespace, vmName string) {
	GinkgoHelper()
	By(fmt.Sprintf("Waiting for success at virtual machine instance migration %s/%s", namespace, vmName))
	Eventually(func(g Gomega) string {
		GinkgoHelper()
		migrationCompletedStr, err := v.getJSONPath("vmim", vmName, "{@.status.migrationState.completed}")
		g.Expect(err).To(SatisfyAny(
			WithTransform(errors.IsNotFound, BeTrue()),
			Not(HaveOccurred()),
		))

		g.Expect(migrationCompletedStr).NotTo(BeEmpty())

		return migrationCompletedStr
	}).WithPolling(time.Second).WithTimeout(5 * time.Minute).Should(Equal("true"))
	migrationFailedStr, err := v.getJSONPath("vmim", vmName, "{@.status.migrationState.failed}")
	Expect(err).To(Not(HaveOccurred()))
	Expect(migrationFailedStr).To(BeEmpty())
}

func addressFromStatus(cli *vmClient, vmNamespace, vmName string) []string {
	addressesStr, err := cli.getJSONPath("vmi", vmName, "{@.status.interfaces[0].ipAddresses}")
	Expect(err).To(Not(HaveOccurred()))
	var addresses []string
	Expect(json.Unmarshal([]byte(addressesStr), &addresses)).To(Succeed())
	return addresses
}

func restartVM(cli *vmClient, vmNamespace, vmName string) {
	By("TODO, call the 'reboot` subresource")
}

func migrateVM(cli *vmClient, vmNamespace, vmName string) {
	cli.createVMIM(vmNamespace, vmName)
	cli.waitForVMIMSuccess(vmNamespace, vmName)
}

func fedoraVMWithSecondaryNetworkAttachment() string {
	return `
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  running: true
  template:
    spec:
      domain:
        devices:
          disks:
            - name: containerdisk
              disk:
                bus: virtio
            - name: cloudinitdisk
              disk:
                bus: virtio
          interfaces:
          - name: underlay
            bridge: {}
        machine:
          type: ""
        resources:
          requests:
            memory: 2048M
      networks:
      - name: underlay
        multus:
          networkName: %[2]s/%[3]s
      terminationGracePeriodSeconds: 0
      volumes:
        - name: containerdisk
          containerDisk:
            image: quay.io/kubevirt/fedora-with-test-tooling-container-disk:devel
        - name: cloudinitdisk
          cloudInitNoCloud:
            userData: |-
              #cloud-config
              password: fedora
              chpasswd: { expire: False }
`
}

func fedoraVMWithPrimaryUDNAttachment() string {
	return `
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  running: true
  template:
    spec:
      domain:
        devices:
          disks:
            - name: containerdisk
              disk:
                bus: virtio
            - name: cloudinitdisk
              disk:
                bus: virtio
          interfaces:
          - name: overlay
            binding:
              name: %[3]s
        machine:
          type: ""
        resources:
          requests:
            memory: 2048M
      networks:
      - name: overlay
        pod: {}
      terminationGracePeriodSeconds: 0
      volumes:
        - name: containerdisk
          containerDisk:
            image: quay.io/kubevirt/fedora-with-test-tooling-container-disk:devel
        - name: cloudinitdisk
          cloudInitNoCloud:
            userData: |-
              #cloud-config
              password: fedora
              chpasswd: { expire: False }
`
}

func fedoraVMIWithSecondaryNetworkAttachment() string {
	return `
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstance
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  domain:
    devices:
      disks:
      - disk:
          bus: virtio
        name: containerdisk
      - disk:
          bus: virtio
        name: cloudinitdisk
      interfaces:
      - bridge: {}
        name: overlay
      rng: {}
    resources:
      requests:
        memory: 2048M
  networks:
  - multus:
      networkName: %[2]s/%[3]s
    name: overlay
  terminationGracePeriodSeconds: 0
  volumes:
  - containerDisk:
      image: quay.io/kubevirt/fedora-with-test-tooling-container-disk:devel
    name: containerdisk
  - cloudInitNoCloud:
      userData: |-
        #cloud-config
        password: fedora
        chpasswd: { expire: False }
    name: cloudinitdisk
`
}

func fedoraVMIWithPrimaryUDNAttachment() string {
	return `
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstance
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  domain:
    devices:
      disks:
      - disk:
          bus: virtio
        name: containerdisk
      - disk:
          bus: virtio
        name: cloudinitdisk
      interfaces:
      - name: overlay
        binding:
          name: %[3]s
      rng: {}
    resources:
      requests:
        memory: 2048M
  networks:
  - pod: {}
    name: overlay
  terminationGracePeriodSeconds: 0
  volumes:
  - containerDisk:
      image: quay.io/kubevirt/fedora-with-test-tooling-container-disk:devel
    name: containerdisk
  - cloudInitNoCloud:
      userData: |-
        #cloud-config
        password: fedora
        chpasswd: { expire: False }
    name: cloudinitdisk
`
}
