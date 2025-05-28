package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	nadapi "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"

	kubeauthorizationv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
	utilnet "k8s.io/utils/net"
	"k8s.io/utils/pointer"

	exutil "github.com/openshift/origin/test/extended/util"
)

const openDefaultPortsAnnotation = "k8s.ovn.org/open-default-ports"
const RequiredUDNNamespaceLabel = "k8s.ovn.org/primary-user-defined-network"

// NOTE: We are observing pod creation requests taking more than two minutes t
// reach the CNI for the CNI to do the necessary plumbing. This is causing tests
// to timeout since pod doesn't go into ready state.
// See https://issues.redhat.com/browse/OCPBUGS-48362 for details. We can revisit
// these values when that bug is fixed but given the Kubernetes test default for a
// pod to startup is 5mins: https://github.com/kubernetes/kubernetes/blob/60c4c2b2521fb454ce69dee737e3eb91a25e0535/test/e2e/framework/timeouts.go#L22-L23
// we are not too far from the mark or against test policy
const podReadyPollTimeout = 10 * time.Minute
const podReadyPollInterval = 6 * time.Second

// NOTE: Upstream, we use either the default of gomega which is 1sec polltimeout with 10ms pollinterval OR
// the tests have hardcoded values with 5sec being common for polltimeout and 10ms for pollinterval
// This is being changed to be 10seconds poll timeout to account for infrastructure complexity between
// OpenShift and KIND clusters. Also changing the polling interval to be 1 second so that in both
// Eventually and Consistently blocks we get at least 10 retries (10/1) in good conditions and 5 retries (10/2) in
// bad conditions since connectToServer util has a 2 second timeout.
// FIXME: Timeout increased to 30 seconds because default network controller does not receive the pod event after its annotations
// are updated. Reduce timeout back to sensible value once issue is understood.
const serverConnectPollTimeout = 30 * time.Second
const serverConnectPollInterval = 1 * time.Second

// randomNetworkMetaName return pseudo random name for network related objects (NAD,UDN,CUDN).
// CUDN is cluster-scoped object, in case tests running in parallel, having random names avoids
// conflicting with other tests.
func randomNetworkMetaName() string {
	return fmt.Sprintf("test-net-%s", rand.String(5))
}

var nadToUdnParams = map[string]string{
	"primary":   "Primary",
	"secondary": "Secondary",
	"layer2":    "Layer2",
	"layer3":    "Layer3",
}

func generateUserDefinedNetworkManifest(params *networkAttachmentConfigParams) string {
	subnets := generateSubnetsYaml(params)
	return `
apiVersion: k8s.ovn.org/v1
kind: UserDefinedNetwork
metadata:
  name: ` + params.name + `
spec:
  topology: ` + nadToUdnParams[params.topology] + `
  ` + params.topology + `: 
    role: ` + nadToUdnParams[params.role] + `
    subnets: ` + subnets + `
    ` + generateIPAMLifecycle(params) + `
`
}

func generateClusterUserDefinedNetworkManifest(params *networkAttachmentConfigParams) string {
	subnets := generateSubnetsYaml(params)
	return `
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: ` + params.name + `
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [` + params.namespace + `]
  network:
    topology: ` + nadToUdnParams[params.topology] + `
    ` + params.topology + `: 
      role: ` + nadToUdnParams[params.role] + `
      subnets: ` + subnets + `
`
}

func generateSubnetsYaml(params *networkAttachmentConfigParams) string {
	if params.topology == "layer3" {
		l3Subnets := generateLayer3Subnets(params.cidr)
		return fmt.Sprintf("[%s]", strings.Join(l3Subnets, ","))
	}
	return fmt.Sprintf("[%s]", params.cidr)
}

func generateLayer3Subnets(cidrs string) []string {
	cidrList := strings.Split(cidrs, ",")
	var subnets []string
	for _, cidr := range cidrList {
		cidrSplit := strings.Split(cidr, "/")
		switch len(cidrSplit) {
		case 2:
			subnets = append(subnets, fmt.Sprintf(`{cidr: "%s/%s"}`, cidrSplit[0], cidrSplit[1]))
		case 3:
			subnets = append(subnets, fmt.Sprintf(`{cidr: "%s/%s", hostSubnet: %q }`, cidrSplit[0], cidrSplit[1], cidrSplit[2]))
		default:
			panic(fmt.Sprintf("invalid layer3 subnet: %v", cidr))
		}
	}
	return subnets
}

func generateIPAMLifecycle(params *networkAttachmentConfigParams) string {
	if !params.allowPersistentIPs {
		return ""
	}
	return `ipam:
      lifecycle: Persistent`
}

func createManifest(namespace, manifest string) (func(), error) {
	tmpDir, err := os.MkdirTemp("", "udn-test")
	if err != nil {
		return nil, err
	}
	cleanup := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			framework.Logf("Unable to remove udn test yaml files from disk %s: %v", tmpDir, err)
		}
	}

	path := filepath.Join(tmpDir, "test-ovn-k-udn-"+rand.String(5)+".yaml")
	if err := os.WriteFile(path, []byte(manifest), 0644); err != nil {
		framework.Failf("Unable to write udn yaml to disk: %v", err)
	}

	_, err = e2ekubectl.RunKubectl(namespace, "create", "-f", path)
	if err != nil {
		return cleanup, err
	}
	return cleanup, nil
}

func applyManifest(namespace, manifest string) error {
	_, err := e2ekubectl.RunKubectlInput(namespace, manifest, "apply", "-f", "-")
	return err
}

var clusterUDNGVR = schema.GroupVersionResource{
	Group:    "k8s.ovn.org",
	Version:  "v1",
	Resource: "clusteruserdefinednetworks",
}

var udnGVR = schema.GroupVersionResource{
	Group:    "k8s.ovn.org",
	Version:  "v1",
	Resource: "userdefinednetworks",
}

// getConditions extracts metav1 conditions from .status.conditions of an unstructured object
func getConditions(uns *unstructured.Unstructured) ([]metav1.Condition, error) {
	var conditions []metav1.Condition
	conditionsRaw, found, err := unstructured.NestedFieldNoCopy(uns.Object, "status", "conditions")
	if err != nil {
		return nil, fmt.Errorf("failed getting conditions in %s: %v", uns.GetName(), err)
	}
	if !found {
		return nil, fmt.Errorf("conditions not found in %v", uns)
	}

	conditionsJSON, err := json.Marshal(conditionsRaw)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(conditionsJSON, &conditions); err != nil {
		return nil, err
	}

	return conditions, nil
}

// userDefinedNetworkReadyFunc returns a function that checks for the NetworkCreated/NetworkReady condition in the provided udn
func userDefinedNetworkReadyFunc(client dynamic.Interface, namespace, name string) func() error {
	return func() error {
		udn, err := client.Resource(udnGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{}, "status")
		if err != nil {
			return err
		}
		conditions, err := getConditions(udn)
		if err != nil {
			return err
		}
		if len(conditions) == 0 {
			return fmt.Errorf("no conditions found in: %v", udn)
		}
		for _, udnCondition := range conditions {
			if (udnCondition.Type == "NetworkCreated" || udnCondition.Type == "NetworkReady") && udnCondition.Status == metav1.ConditionTrue {
				return nil
			}

		}
		return fmt.Errorf("no NetworkCreated/NetworkReady condition found in: %v", udn)
	}
}

// userDefinedNetworkReadyFunc returns a function that checks for the NetworkCreated/NetworkReady condition in the provided cluster udn
func clusterUserDefinedNetworkReadyFunc(client dynamic.Interface, name string) func() error {
	return func() error {
		cUDN, err := client.Resource(clusterUDNGVR).Get(context.Background(), name, metav1.GetOptions{}, "status")
		if err != nil {
			return err
		}
		conditions, err := getConditions(cUDN)
		if err != nil {
			return err
		}
		if len(conditions) == 0 {
			return fmt.Errorf("no conditions found in: %v", cUDN)
		}
		for _, cUDNCondition := range conditions {
			if (cUDNCondition.Type == "NetworkCreated" || cUDNCondition.Type == "NetworkReady") && cUDNCondition.Status == metav1.ConditionTrue {
				return nil
			}
		}
		return fmt.Errorf("no NetworkCreated/NetworkReady condition found in: %v", cUDN)
	}
}

func newPrimaryUserDefinedNetworkManifest(oc *exutil.CLI, name string) string {
	return `
apiVersion: k8s.ovn.org/v1
kind: UserDefinedNetwork
metadata:
  name: ` + name + `
spec:
  topology: Layer3
  layer3:
    role: Primary
    subnets: ` + generateCIDRforUDN(oc)
}

func generateCIDRforUDN(oc *exutil.CLI) string {
	hasIPv4, hasIPv6, err := GetIPAddressFamily(oc)
	Expect(err).NotTo(HaveOccurred())
	cidr := `
    - cidr: 10.20.100.0/16
`
	if hasIPv6 && hasIPv4 {
		cidr = `
    - cidr: 10.20.100.0/16
    - cidr: 2014:100:200::0/60
`
	} else if hasIPv6 {
		cidr = `
    - cidr: 2014:100:200::0/60
`
	}
	return cidr
}

func newUserDefinedNetworkManifest(name string) string {
	return `
apiVersion: k8s.ovn.org/v1
kind: UserDefinedNetwork
metadata:
  name: ` + name + `
spec:
  topology: "Layer2"
  layer2:
    role: Secondary
    subnets: ["10.100.0.0/16"]
`
}

func assertNetAttachDefManifest(nadClient nadclient.K8sCniCncfIoV1Interface, namespace, udnName, udnUID string) {
	nad, err := nadClient.NetworkAttachmentDefinitions(namespace).Get(context.Background(), udnName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	ExpectWithOffset(1, nad.Name).To(Equal(udnName))
	ExpectWithOffset(1, nad.Namespace).To(Equal(namespace))
	ExpectWithOffset(1, nad.OwnerReferences).To(Equal([]metav1.OwnerReference{{
		APIVersion:         "k8s.ovn.org/v1",
		Kind:               "UserDefinedNetwork",
		Name:               "test-net",
		UID:                types.UID(udnUID),
		BlockOwnerDeletion: pointer.Bool(true),
		Controller:         pointer.Bool(true),
	}}))

	jsonTemplate := `{
		"cniVersion":"1.0.0",
		"type": "ovn-k8s-cni-overlay",
		"name": "%s",
		"netAttachDefName": "%s",
		"topology": "layer2",
		"role": "secondary",
		"subnets": "10.100.0.0/16"
	}`

	// REMOVEME(trozet): after network name has been updated to use underscores in OVNK
	expectedLegacyNetworkName := namespace + "." + udnName
	expectedNetworkName := namespace + "_" + udnName
	expectedNadName := namespace + "/" + udnName

	nadJSONLegacy := fmt.Sprintf(jsonTemplate, expectedLegacyNetworkName, expectedNadName)
	nadJSON := fmt.Sprintf(jsonTemplate, expectedNetworkName, expectedNadName)

	ExpectWithOffset(1, nad.Spec.Config).To(SatisfyAny(
		MatchJSON(nadJSONLegacy),
		MatchJSON(nadJSON),
	))
}

func validateUDNStatusReportsConsumers(client dynamic.Interface, udnNamesapce, udnName, expectedPodName string) error {
	udn, err := client.Resource(udnGVR).Namespace(udnNamesapce).Get(context.Background(), udnName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	conditions, err := getConditions(udn)
	if err != nil {
		return err
	}
	conditions = normalizeConditions(conditions)
	expectedMsg := fmt.Sprintf("failed to delete NetworkAttachmentDefinition [%[1]s/%[2]s]: network in use by the following pods: [%[1]s/%[3]s]",
		udnNamesapce, udnName, expectedPodName)
	networkReadyCondition := metav1.Condition{
		Type:    "NetworkReady",
		Status:  metav1.ConditionFalse,
		Reason:  "SyncError",
		Message: expectedMsg,
	}
	networkCreatedCondition := metav1.Condition{
		Type:    "NetworkCreated",
		Status:  metav1.ConditionFalse,
		Reason:  "SyncError",
		Message: expectedMsg,
	}
	for _, udnCondition := range conditions {
		if udnCondition == networkReadyCondition || udnCondition == networkCreatedCondition {
			return nil
		}
	}
	return fmt.Errorf("failed to find NetworkCreated/NetworkReady condition in %v", conditions)
}

func normalizeConditions(conditions []metav1.Condition) []metav1.Condition {
	for i := range conditions {
		t := metav1.NewTime(time.Time{})
		conditions[i].LastTransitionTime = t
	}
	return conditions
}

func assertClusterNADManifest(nadClient nadclient.K8sCniCncfIoV1Interface, namespace, udnName, udnUID string) {
	nad, err := nadClient.NetworkAttachmentDefinitions(namespace).Get(context.Background(), udnName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	ExpectWithOffset(1, nad.Name).To(Equal(udnName))
	ExpectWithOffset(1, nad.Namespace).To(Equal(namespace))
	ExpectWithOffset(1, nad.OwnerReferences).To(Equal([]metav1.OwnerReference{{
		APIVersion:         "k8s.ovn.org/v1",
		Kind:               "ClusterUserDefinedNetwork",
		Name:               udnName,
		UID:                types.UID(udnUID),
		BlockOwnerDeletion: pointer.Bool(true),
		Controller:         pointer.Bool(true),
	}}))
	ExpectWithOffset(1, nad.Labels).To(Equal(map[string]string{"k8s.ovn.org/user-defined-network": ""}))
	ExpectWithOffset(1, nad.Finalizers).To(Equal([]string{"k8s.ovn.org/user-defined-network-protection"}))

	// REMOVEME(trozet): after network name has been updated to use underscores in OVNK
	expectedLegacyNetworkName := "cluster.udn." + udnName

	expectedNetworkName := "cluster_udn_" + udnName
	expectedNadName := namespace + "/" + udnName

	jsonTemplate := `{
		"cniVersion":"1.0.0",
		"type": "ovn-k8s-cni-overlay",
		"name": "%s",
		"netAttachDefName": "%s",
		"topology": "layer2",
		"role": "secondary",
		"subnets": "10.100.0.0/16"
	}`

	nadJSONLegacy := fmt.Sprintf(jsonTemplate, expectedLegacyNetworkName, expectedNadName)
	nadJSON := fmt.Sprintf(jsonTemplate, expectedNetworkName, expectedNadName)

	ExpectWithOffset(1, nad.Spec.Config).To(SatisfyAny(
		MatchJSON(nadJSONLegacy),
		MatchJSON(nadJSON),
	))
}

func validateClusterUDNStatusReportsActiveNamespacesFunc(client dynamic.Interface, cUDNName string, expectedActiveNsNames ...string) func() error {
	return func() error {
		cUDN, err := client.Resource(clusterUDNGVR).Get(context.Background(), cUDNName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		conditions, err := getConditions(cUDN)
		if err != nil {
			return err
		}
		if len(conditions) == 0 {
			return fmt.Errorf("expected at least one condition in %v", cUDN)
		}

		c := conditions[0]
		if c.Type != "NetworkCreated" && c.Type != "NetworkReady" {
			return fmt.Errorf("expected NetworkCreated/NetworkReady type in %v", c)
		}
		if c.Status != metav1.ConditionTrue {
			return fmt.Errorf("expected True status in %v", c)
		}
		if c.Reason != "NetworkAttachmentDefinitionCreated" && c.Reason != "NetworkAttachmentDefinitionReady" {
			return fmt.Errorf("expected NetworkAttachmentDefinitionCreated/NetworkAttachmentDefinitionReady reason in %v", c)
		}
		if !strings.Contains(c.Message, "NetworkAttachmentDefinition has been created in following namespaces:") {
			return fmt.Errorf("expected \"NetworkAttachmentDefinition has been created in following namespaces:\" in %s", c.Message)
		}

		for _, ns := range expectedActiveNsNames {
			if !strings.Contains(c.Message, ns) {
				return fmt.Errorf("expected to find %q namespace in %s", ns, c.Message)
			}
		}
		return nil
	}
}

func validateClusterUDNStatusReportConsumers(client dynamic.Interface, cUDNName, udnNamespace, expectedPodName string) error {
	cUDN, err := client.Resource(clusterUDNGVR).Get(context.Background(), cUDNName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	conditions, err := getConditions(cUDN)
	if err != nil {
		return err
	}
	conditions = normalizeConditions(conditions)
	expectedMsg := fmt.Sprintf("failed to delete NetworkAttachmentDefinition [%[1]s/%[2]s]: network in use by the following pods: [%[1]s/%[3]s]",
		udnNamespace, cUDNName, expectedPodName)
	networkCreatedCondition := metav1.Condition{
		Type:    "NetworkCreated",
		Status:  metav1.ConditionFalse,
		Reason:  "NetworkAttachmentDefinitionSyncError",
		Message: expectedMsg,
	}
	networkReadyCondition := metav1.Condition{
		Type:    "NetworkReady",
		Status:  metav1.ConditionFalse,
		Reason:  "NetworkAttachmentDefinitionSyncError",
		Message: expectedMsg,
	}
	for _, clusterUDNCondition := range conditions {
		if clusterUDNCondition == networkCreatedCondition || clusterUDNCondition == networkReadyCondition {
			return nil
		}
	}
	return fmt.Errorf("failed to find NetworkCreated/NetworkReady condition in %v", conditions)
}

func newClusterUDNManifest(name string, targetNamespaces ...string) string {
	targetNs := strings.Join(targetNamespaces, ",")
	return `
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: ` + name + `
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [ ` + targetNs + ` ]
  network:
    topology: Layer2
    layer2:
      role: Secondary
      subnets: ["10.100.0.0/16"]
`
}

func newPrimaryClusterUDNManifest(oc *exutil.CLI, name string, targetNamespaces ...string) string {
	targetNs := strings.Join(targetNamespaces, ",")
	return `
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: ` + name + `
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [ ` + targetNs + ` ]
  network:
    topology: Layer3
    layer3:
      role: Primary
      subnets: ` + generateCIDRforClusterUDN(oc)
}

func generateCIDRforClusterUDN(oc *exutil.CLI) string {
	hasIPv4, hasIPv6, err := GetIPAddressFamily(oc)
	Expect(err).NotTo(HaveOccurred())
	cidr := `[{cidr: "203.203.0.0/16"}]`
	if hasIPv6 && hasIPv4 {
		cidr = `[{cidr: "203.203.0.0/16"},{cidr: "2014:100:200::0/60"}]`
	} else if hasIPv6 {
		cidr = `[{cidr: "2014:100:200::0/60"}]`
	}
	return cidr
}

func setRuntimeDefaultPSA(pod *v1.Pod) {
	dontEscape := false
	noRoot := true
	pod.Spec.SecurityContext = &v1.PodSecurityContext{
		RunAsNonRoot: &noRoot,
		SeccompProfile: &v1.SeccompProfile{
			Type: v1.SeccompProfileTypeRuntimeDefault,
		},
	}
	pod.Spec.Containers[0].SecurityContext = &v1.SecurityContext{
		AllowPrivilegeEscalation: &dontEscape,
		Capabilities: &v1.Capabilities{
			Drop: []v1.Capability{"ALL"},
		},
	}
}

type podOption func(*podConfiguration)

func podConfig(podName string, opts ...podOption) *podConfiguration {
	pod := &podConfiguration{
		name: podName,
	}
	for _, opt := range opts {
		opt(pod)
	}
	return pod
}

func withCommand(cmdGenerationFn func() []string) podOption {
	return func(pod *podConfiguration) {
		pod.containerCmd = cmdGenerationFn()
	}
}

func withLabels(labels map[string]string) podOption {
	return func(pod *podConfiguration) {
		pod.labels = labels
	}
}

func withNetworkAttachment(networks []nadapi.NetworkSelectionElement) podOption {
	return func(pod *podConfiguration) {
		pod.attachments = networks
	}
}

// podIPsForUserDefinedPrimaryNetwork returns the v4 or v6 IPs for a pod on the UDN
func podIPsForUserDefinedPrimaryNetwork(k8sClient clientset.Interface, podNamespace string, podName string, attachmentName string, index int) (string, error) {
	pod, err := k8sClient.CoreV1().Pods(podNamespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	netStatus, err := userDefinedNetworkStatus(pod, attachmentName)
	if err != nil {
		return "", err
	}

	if len(netStatus.IPs) == 0 {
		return "", fmt.Errorf("attachment for network %q without IPs", attachmentName)
	}
	if len(netStatus.IPs) > 2 {
		return "", fmt.Errorf("attachment for network %q with more than two IPs", attachmentName)
	}
	return netStatus.IPs[index].IP.String(), nil
}

func podIPsForDefaultNetwork(k8sClient clientset.Interface, podNamespace string, podName string) (string, string, error) {
	pod, err := k8sClient.CoreV1().Pods(podNamespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	ipv4, ipv6 := getPodAddresses(pod)
	return ipv4, ipv6, nil
}

func userDefinedNetworkStatus(pod *v1.Pod, networkName string) (PodAnnotation, error) {
	netStatus, err := unmarshalPodAnnotation(pod.Annotations, networkName)
	if err != nil {
		return PodAnnotation{}, fmt.Errorf("failed to unmarshall annotations for pod %q: %v", pod.Name, err)
	}

	return *netStatus, nil
}

func runUDNPod(cs clientset.Interface, namespace string, podConfig podConfiguration, podSpecTweak func(*v1.Pod)) *v1.Pod {
	By(fmt.Sprintf("instantiating the UDN pod %s", podConfig.name))
	podSpec := generatePodSpec(podConfig)
	if podSpecTweak != nil {
		podSpecTweak(podSpec)
	}
	serverPod, err := cs.CoreV1().Pods(podConfig.namespace).Create(
		context.Background(),
		podSpec,
		metav1.CreateOptions{},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(serverPod).NotTo(BeNil())

	By(fmt.Sprintf("asserting the UDN pod %s reaches the `Ready` state", podConfig.name))
	var updatedPod *v1.Pod
	Eventually(func() v1.PodPhase {
		updatedPod, err = cs.CoreV1().Pods(namespace).Get(context.Background(), serverPod.GetName(), metav1.GetOptions{})
		if err != nil {
			return v1.PodFailed
		}
		return updatedPod.Status.Phase
	}, podReadyPollTimeout, podReadyPollInterval).Should(Equal(v1.PodRunning))
	return updatedPod
}

type networkAttachmentConfigParams struct {
	cidr               string
	excludeCIDRs       []string
	namespace          string
	name               string
	topology           string
	networkName        string
	vlanID             int
	allowPersistentIPs bool
	role               string
}

type networkAttachmentConfig struct {
	networkAttachmentConfigParams
}

func newNetworkAttachmentConfig(params networkAttachmentConfigParams) networkAttachmentConfig {
	networkAttachmentConfig := networkAttachmentConfig{
		networkAttachmentConfigParams: params,
	}
	if networkAttachmentConfig.networkName == "" {
		networkAttachmentConfig.networkName = uniqueNadName(networkAttachmentConfig.name)
	}
	return networkAttachmentConfig
}

func uniqueNadName(originalNetName string) string {
	const randomStringLength = 5
	return fmt.Sprintf("%s_%s", rand.String(randomStringLength), originalNetName)
}

func generateNAD(config networkAttachmentConfig) *nadapi.NetworkAttachmentDefinition {
	nadSpec := fmt.Sprintf(
		`
{
        "cniVersion": "0.3.0",
        "name": %q,
        "type": "ovn-k8s-cni-overlay",
        "topology":%q,
        "subnets": %q,
        "excludeSubnets": %q,
        "mtu": 1300,
        "netAttachDefName": %q,
        "vlanID": %d,
        "allowPersistentIPs": %t,
        "role": %q
}
`,
		config.networkName,
		config.topology,
		config.cidr,
		strings.Join(config.excludeCIDRs, ","),
		namespacedName(config.namespace, config.name),
		config.vlanID,
		config.allowPersistentIPs,
		config.role,
	)
	return generateNetAttachDef(config.namespace, config.name, nadSpec)
}

func generateNetAttachDef(namespace, nadName, nadSpec string) *nadapi.NetworkAttachmentDefinition {
	return &nadapi.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nadName,
			Namespace: namespace,
		},
		Spec: nadapi.NetworkAttachmentDefinitionSpec{Config: nadSpec},
	}
}

func namespacedName(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

type podConfiguration struct {
	attachments            []nadapi.NetworkSelectionElement
	containerCmd           []string
	name                   string
	namespace              string
	nodeSelector           map[string]string
	isPrivileged           bool
	labels                 map[string]string
	requiresExtraNamespace bool
}

func generatePodSpec(config podConfiguration) *v1.Pod {
	podSpec := frameworkpod.NewAgnhostPod(config.namespace, config.name, nil, nil, nil, config.containerCmd...)
	if len(config.attachments) > 0 {
		podSpec.Annotations = networkSelectionElements(config.attachments...)
	}
	podSpec.Spec.NodeSelector = config.nodeSelector
	podSpec.Labels = config.labels
	if config.isPrivileged {
		privileged := true
		podSpec.Spec.Containers[0].SecurityContext.Privileged = &privileged
	}
	return podSpec
}

func networkSelectionElements(elements ...nadapi.NetworkSelectionElement) map[string]string {
	marshalledElements, err := json.Marshal(elements)
	if err != nil {
		panic(fmt.Errorf("programmer error: you've provided wrong input to the test data: %v", err))
	}
	return map[string]string{
		nadapi.NetworkAttachmentAnnot: string(marshalledElements),
	}
}

func httpServerContainerCmd(port uint16) []string {
	return []string{"netexec", "--http-port", fmt.Sprintf("%d", port)}
}

// takes the CLI, potential ipv4 and ipv6 cidrs and returns the correct cidr family for the cluster under test
func correctCIDRFamily(oc *exutil.CLI, ipv4CIDR, ipv6CIDR string) string {
	hasIPv4, hasIPv6, err := GetIPAddressFamily(oc)
	Expect(err).NotTo(HaveOccurred())
	// dual stack cluster
	if hasIPv6 && hasIPv4 {
		return strings.Join([]string{ipv4CIDR, ipv6CIDR}, ",")
	}
	// single stack ipv6 cluster
	if hasIPv6 {
		return ipv6CIDR
	}
	// single stack ipv4 cluster
	return ipv4CIDR
}

func getNetCIDRSubnet(netCIDR string) (string, error) {
	subStrings := strings.Split(netCIDR, "/")
	if len(subStrings) == 3 {
		return subStrings[0] + "/" + subStrings[1], nil
	} else if len(subStrings) == 2 {
		return netCIDR, nil
	}
	return "", fmt.Errorf("invalid network cidr: %q", netCIDR)
}

func inRange(cidr string, ip string) error {
	_, cidrRange, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	if cidrRange.Contains(net.ParseIP(ip)) {
		return nil
	}

	return fmt.Errorf("ip [%s] is NOT in range %s", ip, cidr)
}

func connectToServer(clientPodConfig podConfiguration, serverIP string, port int) error {
	_, err := connectToServerWithPath(clientPodConfig.namespace, clientPodConfig.name, serverIP, "" /* no path */, port)
	return err
}

func connectToServerWithPath(podNamespace, podName, serverIP, path string, port int) (string, error) {
	return e2ekubectl.RunKubectl(
		podNamespace,
		"exec",
		podName,
		"--",
		"curl",
		"--connect-timeout",
		"2",
		net.JoinHostPort(serverIP, fmt.Sprintf("%d", port))+path,
	)
}

// Returns pod's ipv4 and ipv6 addresses IN ORDER
func getPodAddresses(pod *v1.Pod) (string, string) {
	var ipv4Res, ipv6Res string
	for _, a := range pod.Status.PodIPs {
		if utilnet.IsIPv4String(a.IP) {
			ipv4Res = a.IP
		}
		if utilnet.IsIPv6String(a.IP) {
			ipv6Res = a.IP
		}
	}
	return ipv4Res, ipv6Res
}

// PodAnnotation describes the assigned network details for a single pod network. (The
// actual annotation may include the equivalent of multiple PodAnnotations.)
type PodAnnotation struct {
	// IPs are the pod's assigned IP addresses/prefixes
	IPs []*net.IPNet
	// MAC is the pod's assigned MAC address
	MAC net.HardwareAddr
	// Gateways are the pod's gateway IP addresses; note that there may be
	// fewer Gateways than IPs.
	Gateways []net.IP
	// Routes are additional routes to add to the pod's network namespace
	Routes []PodRoute
	// Primary reveals if this network is the primary network of the pod or not
	Primary bool
}

// PodRoute describes any routes to be added to the pod's network namespace
type PodRoute struct {
	// Dest is the route destination
	Dest *net.IPNet
	// NextHop is the IP address of the next hop for traffic destined for Dest
	NextHop net.IP
}

type annotationNotSetError struct {
	msg string
}

func (anse annotationNotSetError) Error() string {
	return anse.msg
}

// newAnnotationNotSetError returns an error for an annotation that is not set
func newAnnotationNotSetError(format string, args ...interface{}) error {
	return annotationNotSetError{msg: fmt.Sprintf(format, args...)}
}

type podAnnotation struct {
	IPs      []string   `json:"ip_addresses"`
	MAC      string     `json:"mac_address"`
	Gateways []string   `json:"gateway_ips,omitempty"`
	Routes   []podRoute `json:"routes,omitempty"`

	IP      string `json:"ip_address,omitempty"`
	Gateway string `json:"gateway_ip,omitempty"`
	Primary bool   `json:"primary"`
}

type podRoute struct {
	Dest    string `json:"dest"`
	NextHop string `json:"nextHop"`
}

// UnmarshalPodAnnotation returns the default network info from pod.Annotations
func unmarshalPodAnnotation(annotations map[string]string, networkName string) (*PodAnnotation, error) {
	const podNetworkAnnotation = "k8s.ovn.org/pod-networks"
	ovnAnnotation, ok := annotations[podNetworkAnnotation]
	if !ok {
		return nil, newAnnotationNotSetError("could not find OVN pod annotation in %v", annotations)
	}

	podNetworks := make(map[string]podAnnotation)
	if err := json.Unmarshal([]byte(ovnAnnotation), &podNetworks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ovn pod annotation %q: %v",
			ovnAnnotation, err)
	}
	tempA := podNetworks[networkName]
	a := &tempA

	podAnnotation := &PodAnnotation{Primary: a.Primary}
	var err error

	podAnnotation.MAC, err = net.ParseMAC(a.MAC)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pod MAC %q: %v", a.MAC, err)
	}

	if len(a.IPs) == 0 {
		if a.IP == "" {
			return nil, fmt.Errorf("bad annotation data (neither ip_address nor ip_addresses is set)")
		}
		a.IPs = append(a.IPs, a.IP)
	} else if a.IP != "" && a.IP != a.IPs[0] {
		return nil, fmt.Errorf("bad annotation data (ip_address and ip_addresses conflict)")
	}
	for _, ipstr := range a.IPs {
		ip, ipnet, err := net.ParseCIDR(ipstr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pod IP %q: %v", ipstr, err)
		}
		ipnet.IP = ip
		podAnnotation.IPs = append(podAnnotation.IPs, ipnet)
	}

	if len(a.Gateways) == 0 {
		if a.Gateway != "" {
			a.Gateways = append(a.Gateways, a.Gateway)
		}
	} else if a.Gateway != "" && a.Gateway != a.Gateways[0] {
		return nil, fmt.Errorf("bad annotation data (gateway_ip and gateway_ips conflict)")
	}
	for _, gwstr := range a.Gateways {
		gw := net.ParseIP(gwstr)
		if gw == nil {
			return nil, fmt.Errorf("failed to parse pod gateway %q", gwstr)
		}
		podAnnotation.Gateways = append(podAnnotation.Gateways, gw)
	}

	for _, r := range a.Routes {
		route := PodRoute{}
		_, route.Dest, err = net.ParseCIDR(r.Dest)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pod route dest %q: %v", r.Dest, err)
		}
		if route.Dest.IP.IsUnspecified() {
			return nil, fmt.Errorf("bad podNetwork data: default route %v should be specified as gateway", route)
		}
		if r.NextHop != "" {
			route.NextHop = net.ParseIP(r.NextHop)
			if route.NextHop == nil {
				return nil, fmt.Errorf("failed to parse pod route next hop %q", r.NextHop)
			} else if utilnet.IsIPv6(route.NextHop) != utilnet.IsIPv6CIDR(route.Dest) {
				return nil, fmt.Errorf("pod route %s has next hop %s of different family", r.Dest, r.NextHop)
			}
		}
		podAnnotation.Routes = append(podAnnotation.Routes, route)
	}

	return podAnnotation, nil
}

// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// NextIP returns IP incremented by 1, if IP is invalid, return nil
// copied from https://github.com/containernetworking/plugins/blob/acf8ddc8e1128e6f68a34f7fe91122afeb1fa93d/pkg/ip/cidr.go#L23
func NextIP(ip net.IP) net.IP {
	normalizedIP := normalizeIP(ip)
	if normalizedIP == nil {
		return nil
	}

	i := ipToInt(normalizedIP)
	return intToIP(i.Add(i, big.NewInt(1)), len(normalizedIP) == net.IPv6len)
}

// copied from https://github.com/containernetworking/plugins/blob/acf8ddc8e1128e6f68a34f7fe91122afeb1fa93d/pkg/ip/cidr.go#L60
func ipToInt(ip net.IP) *big.Int {
	return big.NewInt(0).SetBytes(ip)
}

// copied from https://github.com/containernetworking/plugins/blob/acf8ddc8e1128e6f68a34f7fe91122afeb1fa93d/pkg/ip/cidr.go#L64
func intToIP(i *big.Int, isIPv6 bool) net.IP {
	intBytes := i.Bytes()

	if len(intBytes) == net.IPv4len || len(intBytes) == net.IPv6len {
		return intBytes
	}

	if isIPv6 {
		return append(make([]byte, net.IPv6len-len(intBytes)), intBytes...)
	}

	return append(make([]byte, net.IPv4len-len(intBytes)), intBytes...)
}

// normalizeIP will normalize IP by family,
// IPv4 : 4-byte form
// IPv6 : 16-byte form
// others : nil
// copied from https://github.com/containernetworking/plugins/blob/acf8ddc8e1128e6f68a34f7fe91122afeb1fa93d/pkg/ip/cidr.go#L82
func normalizeIP(ip net.IP) net.IP {
	if ipTo4 := ip.To4(); ipTo4 != nil {
		return ipTo4
	}
	return ip.To16()
}

// Network masks off the host portion of the IP, if IPNet is invalid,
// return nil
// copied from https://github.com/containernetworking/plugins/blob/acf8ddc8e1128e6f68a34f7fe91122afeb1fa93d/pkg/ip/cidr.go#L89C1-L105C2
func Network(ipn *net.IPNet) *net.IPNet {
	if ipn == nil {
		return nil
	}

	maskedIP := ipn.IP.Mask(ipn.Mask)
	if maskedIP == nil {
		return nil
	}

	return &net.IPNet{
		IP:   maskedIP,
		Mask: ipn.Mask,
	}
}

func udnWaitForOpenShift(oc *exutil.CLI, namespace string) error {
	serviceAccountName := "default"
	framework.Logf("Waiting for ServiceAccount %q to be provisioned...", serviceAccountName)
	err := exutil.WaitForServiceAccount(oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace), serviceAccountName)
	if err != nil {
		return err
	}

	framework.Logf("Waiting on permissions in namespace %q ...", namespace)
	err = exutil.WaitForSelfSAR(1*time.Second, 60*time.Second, oc.AdminKubeClient(), kubeauthorizationv1.SelfSubjectAccessReviewSpec{
		ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
			Namespace: namespace,
			Verb:      "create",
			Group:     "",
			Resource:  "pods",
		},
	})
	if err != nil {
		return err
	}

	framework.Logf("Waiting on SCC annotations in namespace %q ...", namespace)
	err = exutil.WaitForNamespaceSCCAnnotations(oc.AdminKubeClient().CoreV1(), namespace)
	if err != nil {
		return err
	}
	return nil
}
