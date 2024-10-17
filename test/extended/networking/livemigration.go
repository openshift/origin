package networking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
			virtClient *vmClient
		)

		BeforeEach(func() {
			cs = f.ClientSet

			var err error
			nadClient, err = nadclient.NewForConfig(f.ClientConfig())
			Expect(err).NotTo(HaveOccurred())
			virtClient = newVirtClient(oc, "/tmp/")
		})

		Context("with user defined networks and persistent ips configured", func() {
			const (
				nadName           = "blue"
				bindingName       = "passt"
				udnCrReadyTimeout = 5 * time.Second
				vmName            = "myvm"
			)
			var (
				cidrIPv4 = "100.10.0.0/24"
				cidrIPv6 = "2010:100:200::0/60"
			)

			DescribeTableSubtree("created using",
				func(createNetworkFn func(netConfig networkAttachmentConfigParams)) {

					DescribeTable("[Suite:openshift/network/virtualization] should keep ip", func(netConfig networkAttachmentConfigParams, resourceCmd func() string, opCmd func(cli *vmClient, vmNamespace, vmName string)) {
						var err error
						netConfig.namespace = f.Namespace.Name
						// correctCIDRFamily makes use of the ginkgo framework so it needs to be in the testcase
						netConfig.cidr = correctCIDRFamily(oc, cidrIPv4, cidrIPv6)
						workerNodes, err := getWorkerNodesOrdered(cs)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(workerNodes)).To(BeNumerically(">=", 2))

						isDualStack := getIPFamilyForCluster(f) == DualStack

						createNetworkFn(netConfig)

						var cmd string
						if netConfig.role == "primary" {
							cmd = fmt.Sprintf(resourceCmd(), vmName, f.Namespace.Name, bindingName)
						} else {
							cmd = fmt.Sprintf(resourceCmd(), vmName, f.Namespace.Name, nadName)
						}
						virtClient.createVM(f.Namespace.Name, vmName, cmd)
						virtClient.waitForVMReadiness(f.Namespace.Name, vmName)

						By("Retrieving addresses before test operation")
						expectedAddresses := []string{}
						if netConfig.role == "primary" {
							expectedAddresses = addressFromGuest(virtClient, f.Namespace.Name, vmName)
						} else {
							expectedAddresses = addressFromStatus(virtClient, f.Namespace.Name, vmName)
						}
						expectedNumberOfAddreses := 1
						if isDualStack {
							expectedNumberOfAddreses = 2
						}
						Expect(expectedAddresses).To(HaveLen(expectedNumberOfAddreses))

						opCmd(virtClient, f.Namespace.Name, vmName)

						By("Retrieving addresses after test operation")
						obtainedAddresses := []string{}
						if netConfig.role == "primary" {
							obtainedAddresses = addressFromGuest(virtClient, f.Namespace.Name, vmName)
						} else {
							obtainedAddresses = addressFromStatus(virtClient, f.Namespace.Name, vmName)
						}
						Expect(obtainedAddresses).To(ConsistOf(expectedAddresses))

					},
						Entry(
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
						Entry(
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
						Entry(
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
						Entry(
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
				Entry("NetworkAttachmentDefinitions", func(c networkAttachmentConfigParams) {
					netConfig := newNetworkAttachmentConfig(c)
					nad := generateNAD(netConfig)
					_, err := nadClient.NetworkAttachmentDefinitions(c.namespace).Create(context.Background(), nad, metav1.CreateOptions{})
					Expect(err).To(Not(HaveOccurred()))
				}),
				Entry("UserDefinedNetwork", func(c networkAttachmentConfigParams) {
					udnManifest := generateUserDefinedNetworkManifest(&c)
					Expect(applyManifest(c.namespace, udnManifest)).To(Succeed())
					Expect(waitForUserDefinedNetworkReady(c.namespace, c.name, udnCrReadyTimeout)).To(Succeed())
				}))
		})
	})
})

type vmClient struct {
	oc      *exutil.CLI
	virtCtl string
}

func newVirtClient(oc *exutil.CLI, tmpDir string) *vmClient {
	GinkgoHelper()
	virtCtl, err := ensureVirtctl("v1.3.1", tmpDir)
	Expect(err).To(Not(HaveOccurred()))
	return &vmClient{
		oc:      oc,
		virtCtl: virtCtl,
	}
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
		err := applyManifest(v.oc.Namespace(), resource)
		retries++
		return err
	}).WithPolling(time.Second).WithTimeout(time.Minute).Should(Succeed())
}

func (v *vmClient) virtctl(args []string) string {
	GinkgoHelper()
	output, err := exec.Command(v.virtCtl, args...).CombinedOutput()
	Expect(err).To(Not(HaveOccurred()), output)
	return string(output)
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

func (v *vmClient) restart(vmName string) {
	By(fmt.Sprintf("Restarting vmi %s/%s with virtctl restart", v.oc.Namespace(), vmName))
	GinkgoHelper()
	v.virtctl([]string{"restart", "-n", v.oc.Namespace(), vmName})
}

func (v *vmClient) console(vmName, command string) string {
	GinkgoHelper()
	By(fmt.Sprintf("Calling command '%s' on vmi %s/%s with virtctl console", command, v.oc.Namespace(), vmName))
	output, err := kubevirt.RunCommand(v.virtCtl, v.oc.Namespace(), vmName, command, 5*time.Second)
	Expect(err).To(Not(HaveOccurred()), output)
	return output
}

func (v *vmClient) login(vmName, hostname string) {
	GinkgoHelper()
	By(fmt.Sprintf("Loging to vmi %s/%s", v.oc.Namespace(), vmName))
	Expect(kubevirt.LoginToFedoraWithHostname(v.virtCtl, v.oc.Namespace(), vmName, "fedora", "fedora", hostname)).To(Succeed())
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

func (v *vmClient) waitForVMIMSuccess(namespace, vmName string) {
	GinkgoHelper()
	By(fmt.Sprintf("Waiting for success at virtual machine instance migration %s/%s", namespace, vmName))
	Eventually(func(g Gomega) string {
		GinkgoHelper()
		migrationCompletedStr, err := v.getJSONPath("vmim", vmName, "{@.status.migrationState.completed}")
		g.Expect(err).To(SatisfyAny(
			WithTransform(apierrors.IsNotFound, BeTrue()),
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

func addressFromGuest(cli *vmClient, vmNamespace, vmName string) []string {
	cli.login(vmName, vmName)
	output := cli.console(vmName, "ip -j a show dev eth0")
	// [{"ifindex":2,"ifname":"eth0","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1300,"qdisc":"fq_codel","operstate":"UP","group":"default","txqlen":1000,"link_type":"ether","address":"02:ba:c3:00:00:0a","broadcast":"ff:ff:ff:ff:ff:ff","altnames":["enp1s0"],"addr_info":[{"family":"inet","local":"100.10.0.1","prefixlen":24,"broadcast":"100.10.0.255","scope":"global","dynamic":true,"noprefixroute":true,"label":"eth0","valid_life_time":86313548,"preferred_life_time":86313548},{"family":"inet6","local":"fe80::ba:c3ff:fe00:a","prefixlen":64,"scope":"link","valid_life_time":4294967295,"preferred_life_time":4294967295}]}]
	type Address struct {
		Family    string `json:"family,omitempty"`
		IP        string `json:"local,omitempty"`
		PrefixLen uint   `json:"prefixlen,omitempty"`
		Scope     string `json:"scope,omitempty"`
	}
	type Iface struct {
		Name      string    `json:"ifname,omitempty"`
		Addresses []Address `json:"addr_info,omitempty"`
	}
	ifaces := []Iface{}
	Expect(json.Unmarshal([]byte(output), &ifaces)).To(Succeed())
	addresses := []string{}
	for _, address := range ifaces[0].Addresses {
		if address.Scope == "link" {
			continue
		}
		addresses = append(addresses, address.IP)
	}
	return addresses
}

func restartVM(cli *vmClient, vmNamespace, vmName string) {
	By(fmt.Sprintf("Restarting vmi %s/%s", vmNamespace, vmName))
	cli.restart(vmName)
	cli.waitForVMReadiness(vmNamespace, vmName)
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
            networkData: |
              version: 2                                                              
              ethernets:                                                              
                eth0:                                                                 
                  dhcp4: true                                                         
                  dhcp6: true                                                         
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
            networkData: |
              version: 2                                                              
              ethernets:                                                              
                eth0:                                                                 
                  dhcp4: true                                                         
                  dhcp6: true                                                         
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
      networkData: |
        version: 2                                                              
        ethernets:                                                              
          eth0:                                                                 
            dhcp4: true                                                         
            dhcp6: true                                                         
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
      networkData: |
        version: 2                                                              
        ethernets:                                                              
          eth0:                                                                 
            dhcp4: true                                                         
            dhcp6: true                                                         
      userData: |-
        #cloud-config
        password: fedora
        chpasswd: { expire: False }
    name: cloudinitdisk
`
}

func ensureVirtctl(version, dir string) (string, error) {
	url := fmt.Sprintf("https://github.com/kubevirt/kubevirt/releases/download/%[1]s/virtctl-%[1]s-linux-amd64", version)
	filepath := filepath.Join(dir, "virtctl")
	_, err := os.Stat(filepath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := downloadFile(url, filepath); err != nil {
				return "", err
			}
			if err := os.Chmod(filepath, 0755); err != nil {
				log.Fatal(err)
			}
			return filepath, nil
		}
		return "", err
	}
	return filepath, err
}

func downloadFile(url string, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}
