package networking

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	projectv1 "github.com/openshift/api/project/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	networkclient "github.com/openshift/client-go/network/clientset/versioned/typed/network/v1"
	"github.com/openshift/library-go/pkg/network/networkutils"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2edeployment "k8s.io/kubernetes/test/e2e/framework/deployment"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	"k8s.io/kubernetes/test/e2e/framework/pod"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	utilnet "k8s.io/utils/net"
)

type NodeType int

type IPFamily string

const (
	// Initial pod start can be delayed O(minutes) by slow docker pulls
	// TODO: Make this 30 seconds once #4566 is resolved.
	podStartTimeout = 5 * time.Minute

	// How often to poll pods and nodes.
	poll = 5 * time.Second

	// How wide to print pod names, by default. Useful for aligning printing to
	// quickly scan through output.
	podPrintWidth = 55

	// Indicator for same or different node
	SAME_NODE      NodeType = iota
	DIFFERENT_NODE NodeType = iota

	// TODO get these defined as constandts in networkutils
	OpenshiftSDNPluginName  = "OpenShiftSDN"
	OVNKubernetesPluginName = "OVNKubernetes"

	// IP Address Families
	IPv4      IPFamily = "ipv4"
	IPv6      IPFamily = "ipv6"
	DualStack IPFamily = "dual"
	Unknown   IPFamily = "unknown"

	nmstateNamespace = "openshift-nmstate"
)

var (
	masterRoleMachineConfigLabel = map[string]string{"machineconfiguration.openshift.io/role": "master"}
	workerRoleMachineConfigLabel = map[string]string{"machineconfiguration.openshift.io/role": "worker"}
	ipsecConfigurationBaseDir    = exutil.FixturePath("testdata", "ipsec")
	nsMachineConfigFixture       = filepath.Join(ipsecConfigurationBaseDir, "nsconfig-machine-config.yaml")
	nsNodeRebootNoneFixture      = filepath.Join(ipsecConfigurationBaseDir, "nsconfig-reboot-none-policy.yaml")
)

// IsIPv6 returns true if a group of ips are ipv6.
func isIpv6(ip []string) bool {
	ipv6 := false

	for _, ip := range ip {
		netIP := net.ParseIP(ip)
		if netIP != nil && netIP.To4() == nil {
			ipv6 = true
		} else {
			ipv6 = false
		}
	}

	return ipv6
}

func expectNoError(err error, explain ...interface{}) {
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), explain...)
}

func expectError(err error, explain ...interface{}) {
	ExpectWithOffset(1, err).To(HaveOccurred(), explain...)
}

func launchWebserverService(client k8sclient.Interface, namespace, serviceName string, nodeName string) (serviceAddr string) {
	labelSelector := make(map[string]string)
	labelSelector["name"] = "web"
	createPodForService(client, namespace, serviceName, nodeName, labelSelector)

	servicePort := 8080
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
					Port:     int32(servicePort),
				},
			},
			Selector: labelSelector,
		},
	}
	serviceClient := client.CoreV1().Services(namespace)
	_, err := serviceClient.Create(context.Background(), service, metav1.CreateOptions{})
	expectNoError(err)
	expectNoError(exutil.WaitForEndpoint(client, namespace, serviceName))
	createdService, err := serviceClient.Get(context.Background(), serviceName, metav1.GetOptions{})
	expectNoError(err)
	serviceAddr = net.JoinHostPort(createdService.Spec.ClusterIP, strconv.Itoa(servicePort))
	e2e.Logf("Target service IP/port is %s", serviceAddr)
	return
}

func createPodForService(client k8sclient.Interface, namespace, serviceName string, nodeName string, labelMap map[string]string) {
	exutil.LaunchWebserverPod(client, namespace, serviceName, nodeName)
	// FIXME: make e2e.LaunchWebserverPod() set the label when creating the pod
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		podClient := client.CoreV1().Pods(namespace)
		pod, err := podClient.Get(context.Background(), serviceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.ObjectMeta.Labels == nil {
			pod.ObjectMeta.Labels = labelMap
		} else {
			for key, value := range labelMap {
				pod.ObjectMeta.Labels[key] = value
			}
		}
		_, err = podClient.Update(context.Background(), pod, metav1.UpdateOptions{})
		return err
	})
	expectNoError(err)
}

func createWebserverLBService(client k8sclient.Interface, namespace, serviceName, nodeName string,
	externalIPs []string, epSelector map[string]string) error {
	servicePort := 8080
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
					Port:     int32(servicePort),
					TargetPort: intstr.IntOrString{Type: intstr.Int,
						IntVal: 8080},
				},
			},
			ExternalIPs: externalIPs,
			Selector:    epSelector,
		},
	}
	serviceClient := client.CoreV1().Services(namespace)
	e2e.Logf("creating service %s/%s", namespace, serviceName)
	_, err := serviceClient.Create(context.Background(), service, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	e2e.Logf("service %s/%s is created", namespace, serviceName)
	if len(epSelector) > 0 {
		err = exutil.WaitForEndpoint(client, namespace, serviceName)
		if err != nil {
			return err
		}
		e2e.Logf("endpoints for service %s/%s is up", namespace, serviceName)
	}
	_, err = serviceClient.Get(context.Background(), serviceName, metav1.GetOptions{})
	return err
}

func checkConnectivityToHost(f *e2e.Framework, nodeName string, podName string, host string, timeout time.Duration) error {
	e2e.Logf("Creating an exec pod on node %v", nodeName)
	execPod := pod.CreateExecPodOrFail(context.TODO(), f.ClientSet, f.Namespace.Name, fmt.Sprintf("execpod-sourceip-%s", nodeName), func(pod *corev1.Pod) {
		pod.Spec.NodeName = nodeName
	})
	defer func() {
		e2e.Logf("Cleaning up the exec pod")
		err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.Background(), execPod.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	}()

	var stdout string
	e2e.Logf("Waiting up to %v to wget %s", timeout, host)
	cmd := fmt.Sprintf("wget -T 30 -qO- %s", host)
	var err error
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(2) {
		stdout, err = e2eoutput.RunHostCmd(execPod.Namespace, execPod.Name, cmd)
		if err != nil {
			e2e.Logf("got err: %v, retry until timeout", err)
			continue
		}
		// Need to check output because wget -q might omit the error.
		if strings.TrimSpace(stdout) == "" {
			e2e.Logf("got empty stdout, retry until timeout")
			continue
		}
		break
	}
	return err
}

var cachedNetworkPluginName *string

func openshiftSDNMode() string {
	if cachedNetworkPluginName == nil {
		// We don't use exutil.NewCLI() here because it can't be called from BeforeEach()
		out, err := exec.Command(
			"oc", "--kubeconfig="+exutil.KubeConfigPath(),
			"get", "clusternetwork", "default",
			"--template={{.pluginName}}",
		).CombinedOutput()
		pluginName := string(out)
		if err != nil {
			e2e.Logf("Could not check network plugin name: %v. Assuming the OpenshiftSDN plugin is not being used", err)
			pluginName = ""
		}
		cachedNetworkPluginName = &pluginName
	}
	return *cachedNetworkPluginName
}

func platformType(configClient configv1client.Interface) (configv1.PlatformType, error) {
	infrastructure, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return infrastructure.Status.PlatformStatus.Type, nil
}

func networkPluginName() string {
	if cachedNetworkPluginName == nil {
		// We don't use exutil.NewCLI() here because it can't be called from BeforeEach()
		out, err := exec.Command(
			"oc", "--kubeconfig="+exutil.KubeConfigPath(),
			"get", "network", "cluster",
			"--template={{.spec.networkType}}",
		).CombinedOutput()
		pluginName := string(out)
		if err != nil {
			e2e.Logf("Could not check network plugin name: %v. Assuming a non-OpenShift plugin", err)
			pluginName = ""
		}
		cachedNetworkPluginName = &pluginName
	}
	return *cachedNetworkPluginName
}

func pluginIsolatesNamespaces() bool {
	if os.Getenv("NETWORKING_E2E_ISOLATION") == "true" {
		return true
	}
	// Assume that only the OpenShift SDN "multitenant" plugin isolates by default
	return openshiftSDNMode() == networkutils.MultiTenantPluginName
}

func pluginImplementsNetworkPolicy() bool {
	switch {
	case os.Getenv("NETWORKING_E2E_NETWORKPOLICY") == "true":
		return true
	case networkPluginName() == OpenshiftSDNPluginName && openshiftSDNMode() == networkutils.NetworkPolicyPluginName:
		return true
	case networkPluginName() == OVNKubernetesPluginName:
		return true
	default:
		// If we can't detect the plugin, we assume it doesn't support
		// NetworkPolicy, so the tests will work under kubenet
		return false
	}
}

func makeNamespaceGlobal(oc *exutil.CLI, ns *corev1.Namespace) {
	clientConfig := oc.AdminConfig()
	networkClient := networkclient.NewForConfigOrDie(clientConfig)
	netns, err := networkClient.NetNamespaces().Get(context.Background(), ns.Name, metav1.GetOptions{})
	expectNoError(err)
	netns.NetID = 0
	_, err = networkClient.NetNamespaces().Update(context.Background(), netns, metav1.UpdateOptions{})
	expectNoError(err)
}

func makeNamespaceScheduleToAllNodes(f *e2e.Framework) {
	// to avoid hassles dealing with selector limits, set the namespace label selector to empty
	// to allow targeting all nodes
	for {
		ns, err := f.ClientSet.CoreV1().Namespaces().Get(context.Background(), f.Namespace.Name, metav1.GetOptions{})
		expectNoError(err)
		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		ns.Annotations[projectv1.ProjectNodeSelector] = ""
		_, err = f.ClientSet.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
		if err == nil {
			return
		}
		if kapierrs.IsConflict(err) {
			continue
		}
		expectNoError(err)
	}
}

func modifyNetworkConfig(configClient configv1client.Interface, autoAssignCIDRs, allowedCIDRs, rejectedCIDRs []string) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	kubeAPIServerRollout := exutil.WaitForOperatorToRollout(ctx, configClient, "kube-apiserver")
	<-kubeAPIServerRollout.StableBeforeStarting() // wait for the initial state to be stable

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		network, err := configClient.ConfigV1().Networks().Get(ctx, "cluster", metav1.GetOptions{})
		expectNoError(err)
		extIPConfig := &configv1.ExternalIPConfig{Policy: &configv1.ExternalIPPolicy{}}
		if len(allowedCIDRs) != 0 || len(rejectedCIDRs) != 0 || len(autoAssignCIDRs) != 0 {
			extIPConfig = &configv1.ExternalIPConfig{Policy: &configv1.ExternalIPPolicy{AllowedCIDRs: allowedCIDRs,
				RejectedCIDRs: rejectedCIDRs}, AutoAssignCIDRs: autoAssignCIDRs}
		}
		network.Spec.ExternalIP = extIPConfig
		_, err = configClient.ConfigV1().Networks().Update(ctx, network, metav1.UpdateOptions{})
		return err
	})
	expectNoError(err)

	<-kubeAPIServerRollout.Done()
	expectNoError(kubeAPIServerRollout.Err())
}

func setNamespaceExternalGateway(f *e2e.Framework, gatewayIP string) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ns, err := f.ClientSet.CoreV1().Namespaces().Get(context.Background(), f.Namespace.Name, metav1.GetOptions{})
		expectNoError(err)
		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		ns.Annotations["k8s.ovn.org/routing-external-gws"] = gatewayIP
		_, err = f.ClientSet.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
		return err
	})
	expectNoError(err)
}

// findAppropriateNodes tries to find a source and destination for a type of node connectivity
// test (same node, or different node).
func findAppropriateNodes(f *e2e.Framework, nodeType NodeType) (*corev1.Node, *corev1.Node, error) {
	nodes, err := e2enode.GetReadySchedulableNodes(context.TODO(), f.ClientSet)
	if err != nil {
		e2e.Logf("Unable to get schedulable nodes due to %v", err)
		return nil, nil, err
	}
	candidates := nodes.Items

	if len(candidates) == 0 {
		e2e.Failf("Unable to find any candidate nodes for e2e networking tests in \n%#v", nodes.Items)
	}

	// in general, avoiding masters is a good thing, so see if we can find nodes that aren't masters
	if len(candidates) > 1 {
		var withoutMasters []corev1.Node
		// look for anything that has the label value master or infra and try to skip it
		isAllowed := func(node *corev1.Node) bool {
			for _, value := range node.Labels {
				if value == "master" || value == "infra" {
					return false
				}
			}
			return true
		}
		for _, node := range candidates {
			if !isAllowed(&node) {
				continue
			}
			withoutMasters = append(withoutMasters, node)
		}
		if len(withoutMasters) >= 2 {
			candidates = withoutMasters
		}
	}

	var candidateNames, nodeNames []string
	for _, node := range candidates {
		candidateNames = append(candidateNames, node.Name)
	}
	for _, node := range nodes.Items {
		nodeNames = append(nodeNames, node.Name)
	}

	if nodeType == DIFFERENT_NODE {
		if len(candidates) <= 1 {
			e2eskipper.Skipf("Only one node is available in this environment (%v out of %v)", candidateNames, nodeNames)
		}
		e2e.Logf("Using %s and %s for test (%v out of %v)", candidates[0].Name, candidates[1].Name, candidateNames, nodeNames)
		return &candidates[0], &candidates[1], nil
	}
	e2e.Logf("Using %s for test (%v out of %v)", candidates[0].Name, candidateNames, nodeNames)
	return &candidates[0], &candidates[0], nil
}

func checkPodIsolation(f1, f2 *e2e.Framework, nodeType NodeType) error {
	makeNamespaceScheduleToAllNodes(f1)
	makeNamespaceScheduleToAllNodes(f2)
	serverNode, clientNode, err := findAppropriateNodes(f1, nodeType)
	if err != nil {
		return err
	}
	podName := "isolation-webserver"
	defer f1.ClientSet.CoreV1().Pods(f1.Namespace.Name).Delete(context.Background(), podName, metav1.DeleteOptions{})
	ip := exutil.LaunchWebserverPod(f1.ClientSet, f1.Namespace.Name, podName, serverNode.Name)

	return checkConnectivityToHost(f2, clientNode.Name, "isolation-wget", ip, 10*time.Second)
}

func checkServiceConnectivity(serverFramework, clientFramework *e2e.Framework, nodeType NodeType) error {
	makeNamespaceScheduleToAllNodes(serverFramework)
	makeNamespaceScheduleToAllNodes(clientFramework)
	serverNode, clientNode, err := findAppropriateNodes(serverFramework, nodeType)
	if err != nil {
		return err
	}
	podName := names.SimpleNameGenerator.GenerateName("service-")
	defer serverFramework.ClientSet.CoreV1().Pods(serverFramework.Namespace.Name).Delete(context.Background(), podName, metav1.DeleteOptions{})
	defer serverFramework.ClientSet.CoreV1().Services(serverFramework.Namespace.Name).Delete(context.Background(), podName, metav1.DeleteOptions{})
	ip := launchWebserverService(serverFramework.ClientSet, serverFramework.Namespace.Name, podName, serverNode.Name)

	return checkConnectivityToHost(clientFramework, clientNode.Name, "service-wget", ip, 10*time.Second)
}

func InNonIsolatingContext(body func()) {
	Context("when using a plugin in a mode that does not isolate namespaces by default", func() {
		BeforeEach(func() {
			if pluginIsolatesNamespaces() {
				e2eskipper.Skipf("This plugin isolates namespaces by default.")
			}
		})

		body()
	})
}

func InIsolatingContext(body func()) {
	Context("when using a plugin in a mode that isolates namespaces by default", func() {
		BeforeEach(func() {
			if !pluginIsolatesNamespaces() {
				e2eskipper.Skipf("This plugin does not isolate namespaces by default.")
			}
		})

		body()
	})
}

func InNetworkPolicyContext(body func()) {
	Context("when using a plugin that implements NetworkPolicy", func() {
		BeforeEach(func() {
			if !pluginImplementsNetworkPolicy() {
				e2eskipper.Skipf("This plugin does not implement NetworkPolicy.")
			}
		})

		body()
	})
}

func InopenshiftSDNModeContext(plugins []string, body func()) {
	Context(fmt.Sprintf("when using one of the OpenshiftSDN modes '%s'", strings.Join(plugins, ", ")),
		func() {
			BeforeEach(func() {
				found := false
				for _, plugin := range plugins {
					if openshiftSDNMode() == plugin {
						found = true
						break
					}
				}
				if !found {
					e2eskipper.Skipf("Not using one of the specified OpenshiftSDN modes")
				}
			})

			body()
		},
	)
}

func InOpenShiftSDNContext(body func()) {
	Context("when using openshift-sdn",
		func() {
			BeforeEach(func() {
				if networkPluginName() != OpenshiftSDNPluginName {
					e2eskipper.Skipf("Not using openshift-sdn")
				}
			})

			body()
		},
	)
}

func InBareMetalIPv4ClusterContext(oc *exutil.CLI, body func()) {
	Context("when running openshift ipv4 cluster on bare metal [apigroup:config.openshift.io]",
		func() {
			BeforeEach(func() {
				pType, err := platformType(oc.AdminConfigClient())
				expectNoError(err)
				if pType != configv1.BareMetalPlatformType || getIPFamilyForCluster(oc.KubeFramework()) != IPv4 {
					e2eskipper.Skipf("Not running in bare metal ipv4 cluster")
				}
			})

			body()
		},
	)
}

func InIPv4ClusterContext(oc *exutil.CLI, body func()) {
	Context("when running openshift ipv4 cluster",
		func() {
			BeforeEach(func() {
				if getIPFamilyForCluster(oc.KubeFramework()) != IPv4 {
					e2eskipper.Skipf("Not running in ipv4 cluster")
				}
			})

			body()
		},
	)
}

func InOVNKubernetesContext(body func()) {
	Context("when using openshift ovn-kubernetes",
		func() {
			BeforeEach(func() {
				if networkPluginName() != OVNKubernetesPluginName {
					e2eskipper.Skipf("Not using ovn-kubernetes")
				}
			})

			body()
		},
	)
}

func createNetworkAttachmentDefinition(config *rest.Config, namespace string, name string, nadConfig string) error {
	nad := netattdefv1.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: netattdefv1.NetworkAttachmentDefinitionSpec{
			Config: nadConfig,
		},
	}
	client, err := nadclient.NewForConfig(config)
	if err != nil {
		return err
	}
	_, err = client.K8sCniCncfIoV1().NetworkAttachmentDefinitions(namespace).Create(context.Background(), &nad, metav1.CreateOptions{})
	return err
}

func networkAttachmentDefinitionClient(config *rest.Config) (dynamic.NamespaceableResourceInterface, error) {
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	nadGVR := schema.GroupVersionResource{
		Group:    "k8s.cni.cncf.io",
		Version:  "v1",
		Resource: "network-attachment-definitions",
	}
	nadClient := dynClient.Resource(nadGVR)
	return nadClient, nil
}

func getIPFamilyForCluster(f *e2e.Framework) IPFamily {
	podIPs, err := createPod(f.ClientSet, f.Namespace.Name, "test-ip-family-pod")
	expectNoError(err)
	return getIPFamily(podIPs)
}

func getIPFamily(podIPs []v1.PodIP) IPFamily {
	switch len(podIPs) {
	case 1:
		ip := net.ParseIP(podIPs[0].IP)
		if ip.To4() != nil {
			return IPv4
		} else {
			return IPv6
		}
	case 2:
		ip1 := net.ParseIP(podIPs[0].IP)
		ip2 := net.ParseIP(podIPs[1].IP)
		if ip1 == nil || ip2 == nil {
			return Unknown
		}
		if (ip1.To4() == nil) == (ip2.To4() == nil) {
			return Unknown
		}
		return DualStack
	default:
		return Unknown
	}
}

func createPod(client k8sclient.Interface, ns, generateName string) ([]corev1.PodIP, error) {
	pod := frameworkpod.NewAgnhostPod(ns, "", nil, nil, nil)
	pod.ObjectMeta.GenerateName = generateName
	execPod, err := client.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
	expectNoError(err, "failed to create new pod in namespace: %s", ns)
	var podIPs []corev1.PodIP
	err = wait.PollImmediate(poll, 2*time.Minute, func() (bool, error) {
		retrievedPod, err := client.CoreV1().Pods(execPod.Namespace).Get(context.TODO(), execPod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		podIPs = retrievedPod.Status.PodIPs
		return retrievedPod.Status.Phase == corev1.PodRunning, nil
	})
	return podIPs, err
}

// SubnetIPs enumerates all IP addresses in an IP subnet (starting with the provided IP address and including the broadcast address).
func SubnetIPs(ipnet net.IPNet) ([]net.IP, error) {
	var ipList []net.IP
	ip := ipnet.IP
	for ; ipnet.Contains(ip); ip = incIP(ip) {
		ipList = append(ipList, ip)
	}

	return ipList, nil
}

// incIP increases the current IP address by one. This function works for both IPv4 and IPv6.
func incIP(ip net.IP) net.IP {
	// allocate a new IP
	newIp := make(net.IP, len(ip))
	copy(newIp, ip)

	byteIp := []byte(newIp)
	l := len(byteIp)
	var i int
	for k := range byteIp {
		// start with the rightmost index first
		// increment it
		// if the index is < 256, then no overflow happened and we increment and break
		// else, continue to the next field in the byte
		i = l - 1 - k
		if byteIp[i] < 0xff {
			byteIp[i]++
			break
		} else {
			byteIp[i] = 0
		}
	}
	return net.IP(byteIp)
}

// GetIPAddressFamily returns if this cloud uses IPv4 and/or IPv6.
func GetIPAddressFamily(oc *exutil.CLI) (bool, bool, error) {
	var hasIPv4 bool
	var hasIPv6 bool
	var err error

	networkConfig, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, false, err
	}
	for _, cidr := range networkConfig.Spec.ServiceNetwork {
		if utilnet.IsIPv6CIDRString(cidr) {
			hasIPv6 = true
		} else {
			hasIPv4 = true
		}
	}
	return hasIPv4, hasIPv6, nil
}

func deployNmstateHandler(oc *exutil.CLI) error {
	err := waitForDeploymentComplete(oc, nmstateNamespace, "nmstate-operator")
	if err != nil {
		return fmt.Errorf("nmstate operator is not running: %v", err)
	}
	nmStateConfigYaml := exutil.FixturePath("testdata", "ipsec", nmstateConfigureManifestFile)
	err = oc.AsAdmin().Run("apply").Args("-f", nmStateConfigYaml, fmt.Sprintf("--namespace=%s", nmstateNamespace)).Execute()
	if err != nil {
		return fmt.Errorf("error configuring nmstate: %v", err)
	}
	return ensureNmstateHandlerRunning(oc)
}

func ensureNmstateHandlerRunning(oc *exutil.CLI) error {
	err := wait.PollUntilContextTimeout(context.Background(), poll, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			// Ensure nmstate handler is running.
			return isDaemonSetRunning(oc, nmstateNamespace, "nmstate-handler")
		})
	if err != nil {
		return fmt.Errorf("failed to get nmstate handler running: %v", err)
	}
	err = waitForDeploymentComplete(oc, nmstateNamespace, "nmstate-webhook")
	if err != nil {
		return fmt.Errorf("nmstate webhook is not running: %v", err)
	}
	return nil
}

func waitForDeploymentComplete(oc *exutil.CLI, namespace, name string) error {
	deployment, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	return e2edeployment.WaitForDeploymentComplete(oc.AdminKubeClient(), deployment)
}

func isDaemonSetRunningOnGeneration(oc *exutil.CLI, namespace, name string, generation int64) (bool, error) {
	ds, err := getDaemonSet(oc, namespace, name)
	if err != nil {
		return false, err
	}
	if ds == nil {
		return false, nil
	}
	if generation == 0 {
		generation = ds.Generation
	}
	desired, ready := ds.Status.DesiredNumberScheduled, ds.Status.NumberReady
	return generation == ds.Status.ObservedGeneration && desired > 0 && desired == ready, nil
}

func isDaemonSetRunning(oc *exutil.CLI, namespace, name string) (bool, error) {
	return isDaemonSetRunningOnGeneration(oc, namespace, name, 0)
}

func getDaemonSet(oc *exutil.CLI, namespace, name string) (*appsv1.DaemonSet, error) {
	ds, err := oc.AdminKubeClient().AppsV1().DaemonSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, nil
	}
	return ds, err
}

func createIPsecCertsMachineConfig(oc *exutil.CLI) (*mcfgv1.MachineConfig, error) {
	nsCertMachineConfig, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigs().Get(context.Background(),
		nsCertMachineConfigName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if err == nil {
		return nsCertMachineConfig, nil
	}
	err = oc.AsAdmin().Run("create").Args("-f", nsMachineConfigFixture).Execute()
	if err != nil {
		return nil, fmt.Errorf("error deploying IPsec certs Machine Config: %v", err)
	}
	err = wait.PollUntilContextTimeout(context.Background(), poll, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		nsCertMachineConfig, err = oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigs().Get(context.Background(),
			nsCertMachineConfigName, metav1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return true, nil
	})
	return nsCertMachineConfig, err
}

func deleteNSCertMachineConfig(oc *exutil.CLI) error {
	err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigs().Delete(context.Background(),
		nsCertMachineConfigName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func launchHostNetworkedPodForTCPDump(f *e2e.Framework, tcpdumpImage, nodeName, generateName string) (*v1.Pod, error) {
	contName := fmt.Sprintf("%s-container", generateName)
	runAsUser := int64(0)
	securityContext := &v1.SecurityContext{
		RunAsUser: &runAsUser,
		Capabilities: &v1.Capabilities{
			Add: []v1.Capability{
				"SETFCAP",
				"CAP_NET_RAW",
				"CAP_NET_ADMIN",
			},
		},
	}

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            contName,
					Image:           tcpdumpImage,
					Command:         []string{"sleep", "100000"},
					SecurityContext: securityContext,
				},
			},
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyNever,
			HostNetwork:   true,
		},
	}

	p, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	err = wait.PollUntilContextTimeout(context.Background(), poll, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		retrievedPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(ctx, p.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		p = retrievedPod
		return retrievedPod.Status.Phase == corev1.PodRunning, nil
	})

	return p, err
}

func isConnResetErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "connection reset by peer")
}

// This checks master and worker role machine config pools status are set with ipsec
// extension which confirms extension is successfully rolled out on all nodes.
func areMachineConfigPoolsReadyWithIPsec(oc *exutil.CLI) (bool, error) {
	pools, err := getMachineConfigPoolByLabel(oc, masterRoleMachineConfigLabel)
	if err != nil {
		return false, err
	}
	masterWithIPsec := areMachineConfigPoolsReadyWithMachineConfig(pools, masterIPsecMachineConfigName)
	pools, err = getMachineConfigPoolByLabel(oc, workerRoleMachineConfigLabel)
	if err != nil {
		return false, err
	}
	workerWithIPsec := areMachineConfigPoolsReadyWithMachineConfig(pools, workerIPSecMachineConfigName)

	return masterWithIPsec && workerWithIPsec, nil
}

// This checks master and worker role machine config pools status are set without ipsec
// extension which confirms extension is successfully removed from all nodes.
func areMachineConfigPoolsReadyWithoutIPsec(oc *exutil.CLI) (bool, error) {
	pools, err := getMachineConfigPoolByLabel(oc, masterRoleMachineConfigLabel)
	if err != nil {
		return false, err
	}
	masterWithoutIPsec := areMachineConfigPoolsReadyWithoutMachineConfig(pools, masterIPsecMachineConfigName)
	pools, err = getMachineConfigPoolByLabel(oc, workerRoleMachineConfigLabel)
	if err != nil {
		return false, err
	}
	workerWithoutIPsec := areMachineConfigPoolsReadyWithoutMachineConfig(pools, workerIPSecMachineConfigName)

	return masterWithoutIPsec && workerWithoutIPsec, nil
}

func areMachineConfigPoolsReadyWithMachineConfig(pools []mcfgv1.MachineConfigPool, machineConfigName string) bool {
	mcExistsInPool := func(status mcfgv1.MachineConfigPoolStatus, name string) bool {
		return status.MachineCount == status.UpdatedMachineCount &&
			hasSourceInMachineConfigStatus(status, name)
	}
	for _, pool := range pools {
		if !mcExistsInPool(pool.Status, machineConfigName) {
			return false
		}
	}
	return true
}

func areMachineConfigPoolsReadyWithoutMachineConfig(pools []mcfgv1.MachineConfigPool, machineConfigName string) bool {
	mcNotExistsInPool := func(status mcfgv1.MachineConfigPoolStatus, name string) bool {
		return status.MachineCount == status.UpdatedMachineCount &&
			!hasSourceInMachineConfigStatus(status, name)
	}
	for _, pool := range pools {
		if !mcNotExistsInPool(pool.Status, machineConfigName) {
			return false
		}
	}
	return true
}

func hasSourceInMachineConfigStatus(machineConfigStatus mcfgv1.MachineConfigPoolStatus, machineConfigName string) bool {
	for _, source := range machineConfigStatus.Configuration.Source {
		if source.Name == machineConfigName {
			return true
		}
	}
	return false
}

// areClusterOperatorsReady returns true when every cluster operator is with available state and neither in degraded
// nor in progressing state, otherwise returns false.
func areClusterOperatorsReady(oc *exutil.CLI) (bool, error) {
	cos, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	for _, co := range cos.Items {
		available, degraded, progressing := false, true, true
		for _, condition := range co.Status.Conditions {
			isConditionTrue := condition.Status == configv1.ConditionTrue
			switch condition.Type {
			case configv1.OperatorAvailable:
				available = isConditionTrue
			case configv1.OperatorDegraded:
				degraded = isConditionTrue
			case configv1.OperatorProgressing:
				progressing = isConditionTrue
			}
		}
		isCOReady := available && !degraded && !progressing
		if !isCOReady {
			return false, nil
		}
	}
	return true, nil
}

func getMachineConfigPoolByLabel(oc *exutil.CLI, mcSelectorLabel labels.Set) ([]mcfgv1.MachineConfigPool, error) {
	poolList, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().List(context.Background(),
		metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var pools []mcfgv1.MachineConfigPool
	for _, pool := range poolList.Items {
		mcSelector, err := metav1.LabelSelectorAsSelector(pool.Spec.MachineConfigSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid machine config label selector in %s pool", pool.Name)
		}
		if mcSelector.Matches(mcSelectorLabel) {
			pools = append(pools, pool)
		}
	}
	if len(pools) == 0 {
		return nil, fmt.Errorf("empty machine config pools found for the selector")
	}
	return pools, nil
}
