package networking

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
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

	consolev1client "github.com/openshift/client-go/console/clientset/versioned"

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
				cidrIPv4 = "203.203.0.0/16"
				cidrIPv6 = "2014:100:200::0/60"
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

						httpServerPods := prepareHTTPServerPods(f, netConfig, workerNodes)

						tmpl, err := template.New(f.Namespace.Name).Parse(resourceCmd())
						Expect(err).To(Not(HaveOccurred()))
						params := struct {
							VMName                    string
							VMNamespace               string
							FedoraContainterDiskImage string
							NetBindingName            string
							NetworkName               string
						}{
							VMName:                    vmName,
							VMNamespace:               f.Namespace.Name,
							FedoraContainterDiskImage: image.LocationFor("quay.io/kubevirt/fedora-with-test-tooling-container-disk:20241024_891122a6fc"),
						}
						if netConfig.role == "primary" {
							params.NetBindingName = bindingName
						} else {
							params.NetworkName = nadName
						}

						var cmd bytes.Buffer
						Expect(tmpl.Execute(&cmd, params)).To(Succeed())
						Expect(virtClient.createVM(f.Namespace.Name, vmName, cmd.String())).To(Succeed())
						virtClient.waitForVMReadiness(f.Namespace.Name, vmName)

						By("Retrieving addresses before test operation")
						expectedAddresses := []string{}
						if netConfig.role == "primary" {
							expectedAddresses = addressFromGuest(virtClient, vmName)
						} else {
							expectedAddresses = addressFromStatus(virtClient, vmName)
						}
						expectedNumberOfAddresses := 1
						if isDualStack {
							expectedNumberOfAddresses = 2
						}
						Expect(expectedAddresses).To(HaveLen(expectedNumberOfAddresses))

						httpServerPodsIPs := httpServerTestPodsMultusNetworkIPs(netConfig, httpServerPods)

						By("Check east/west traffic before test operation")
						checkEastWestTraffic(virtClient, vmName, httpServerPodsIPs)

						opCmd(virtClient, f.Namespace.Name, vmName)

						By("Retrieving addresses after test operation")
						obtainedAddresses := []string{}
						if netConfig.role == "primary" {
							obtainedAddresses = addressFromGuest(virtClient, vmName)
						} else {
							obtainedAddresses = addressFromStatus(virtClient, vmName)
						}
						Expect(obtainedAddresses).To(ConsistOf(expectedAddresses))

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
					By(fmt.Sprintf("Creating NetworkAttachmentDefinitions %s/%s", nad.Namespace, nad.Name))
					_, err := nadClient.NetworkAttachmentDefinitions(c.namespace).Create(context.Background(), nad, metav1.CreateOptions{})
					Expect(err).To(Not(HaveOccurred()))
				}),
				Entry("UserDefinedNetwork", func(c networkAttachmentConfigParams) {
					udnManifest := generateUserDefinedNetworkManifest(&c)
					By(fmt.Sprintf("Creating UserDefinedNetwork %s/%s", c.namespace, c.name))
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
	virtCtl, err := ensureVirtctl(oc, tmpDir)
	Expect(err).To(Not(HaveOccurred()))
	return &vmClient{
		oc:      oc,
		virtCtl: virtCtl,
	}
}

func (v *vmClient) apply(description, resource string) error {
	GinkgoHelper()
	By(fmt.Sprintf("Create %s", description))
	return applyManifest(v.oc.Namespace(), resource)
}

func (v *vmClient) virtctl(args []string) (string, error) {
	GinkgoHelper()
	output, err := exec.Command(v.virtCtl, args...).CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (v *vmClient) createVM(namespace, vmiName, createVMCmd string) error {
	GinkgoHelper()
	return v.apply(fmt.Sprintf("virtual machine %s/%s", namespace, vmiName), createVMCmd)
}

func (v *vmClient) createVMIM(namespace, vmiName string) error {
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
	return v.apply(fmt.Sprintf("virtual machine instance migration %s/%s", namespace, vmiName), vmim)
}

func (v *vmClient) restart(vmName string) error {
	By(fmt.Sprintf("Restarting vmi %s/%s with virtctl restart", v.oc.Namespace(), vmName))
	GinkgoHelper()
	_, err := v.virtctl([]string{"restart", "-n", v.oc.Namespace(), vmName})
	if err != nil {
		return err
	}
	return nil
}

func (v *vmClient) console(vmName, command string) (string, error) {
	GinkgoHelper()
	By(fmt.Sprintf("Calling command '%s' on vmi %s/%s with virtctl console", command, v.oc.Namespace(), vmName))
	output, err := kubevirt.RunCommand(v.virtCtl, v.oc.Namespace(), vmName, command, 5*time.Second)
	if err != nil {
		return "", err
	}
	return output, nil
}

func (v *vmClient) login(vmName, hostname string) error {
	GinkgoHelper()
	By(fmt.Sprintf("Loging to vmi %s/%s", v.oc.Namespace(), vmName))
	return kubevirt.LoginToFedoraWithHostname(v.virtCtl, v.oc.Namespace(), vmName, "fedora", "fedora", hostname)
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

func addressFromStatus(cli *vmClient, vmName string) []string {
	addressesStr, err := cli.getJSONPath("vmi", vmName, "{@.status.interfaces[0].ipAddresses}")
	Expect(err).To(Not(HaveOccurred()))
	var addresses []string
	Expect(json.Unmarshal([]byte(addressesStr), &addresses)).To(Succeed())
	return addresses
}

func addressFromGuest(cli *vmClient, vmName string) []string {
	Expect(cli.login(vmName, vmName)).To(Succeed())
	output, err := cli.console(vmName, "ip -j a show dev eth0")
	Expect(err).To(Not(HaveOccurred()))
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
	Expect(ifaces).To(Not(BeEmpty()))
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
	Expect(cli.restart(vmName)).To(Succeed())
	cli.waitForVMReadiness(vmNamespace, vmName)
}

func migrateVM(cli *vmClient, vmNamespace, vmName string) {
	GinkgoHelper()
	Expect(cli.createVMIM(vmNamespace, vmName)).To(Succeed())
	cli.waitForVMIMSuccess(vmNamespace, vmName)
}

func fedoraVMWithSecondaryNetworkAttachment() string {
	return `
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: {{ .VMName }}
  namespace: {{ .VMNamespace }}
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
          networkName: {{ .VMNamespace }}/{{ .NetworkName }}
      terminationGracePeriodSeconds: 0
      volumes:
        - name: containerdisk
          containerDisk:
            image: {{ .FedoraContainterDiskImage }} 
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
  name: {{ .VMName }}
  namespace: {{ .VMNamespace }}
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
              name: {{ .NetBindingName }}
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
            image: {{ .FedoraContainterDiskImage }}
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
  name: {{ .VMName }}
  namespace: {{ .VMNamespace }}
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
      networkName: {{ .VMNamespace }}/{{ .NetworkName }}
    name: overlay
  terminationGracePeriodSeconds: 0
  volumes:
  - containerDisk:
      image: {{ .FedoraContainterDiskImage }}
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
  name: {{ .VMName }}
  namespace: {{ .VMNamespace }}
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
          name: {{ .NetBindingName }}
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
      image: {{ .FedoraContainterDiskImage }}
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

func ensureVirtctl(oc *exutil.CLI, dir string) (string, error) {
	url, err := discoverVirtctlURL(oc)
	if err != nil {
		return "", err
	}
	filepath := filepath.Join(dir, "virtctl")
	_, err = os.Stat(filepath)
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

func discoverVirtctlURL(oc *exutil.CLI) (string, error) {
	consoleClient, err := consolev1client.NewForConfig(oc.AsAdmin().UserConfig())
	if err != nil {
		return "", err
	}
	virtctlCliDownload, err := consoleClient.ConsoleV1().ConsoleCLIDownloads().Get(context.Background(), "virtctl-clidownloads-kubevirt-hyperconverged", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for _, virtctlLink := range virtctlCliDownload.Spec.Links {
		if strings.Contains(virtctlLink.Text, "x86_64") {
			return virtctlLink.Href, nil
		}
	}
	return "", fmt.Errorf("missing virtctl for x86_64 arch")
}

func downloadFile(url string, filepath string) error {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(gzipReader)
	for true {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeReg {
			out, err := os.Create(filepath)
			if err != nil {
				return err
			}
			defer out.Close()
			if _, err := io.Copy(out, tarReader); err != nil {
				return err
			}
		}
	}
	return nil
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

func checkEastWestTraffic(virtClient *vmClient, vmiName string, podIPsByName map[string][]string) {
	GinkgoHelper()
	Expect(virtClient.login(vmiName, vmiName)).To(Succeed())
	polling := 15 * time.Second
	timeout := time.Minute
	for podName, podIPs := range podIPsByName {
		for _, podIP := range podIPs {
			output := ""
			Eventually(func() error {
				var err error
				output, err = virtClient.console(vmiName, fmt.Sprintf("curl http://%s", net.JoinHostPort(podIP, "8000")))
				return err
			}).
				WithPolling(polling).
				WithTimeout(timeout).
				Should(Succeed(), func() string { return podName + ": " + output })
		}
	}
}
