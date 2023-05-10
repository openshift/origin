package networking

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	projectv1 "github.com/openshift/api/project/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	networkclient "github.com/openshift/client-go/network/clientset/versioned/typed/network/v1"
	"github.com/openshift/library-go/pkg/network/networkutils"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
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
	openshiftSDNPluginName  = "OpenShiftSDN"
	OVNKubernetesPluginName = "OVNKubernetes"

	// IP Address Families
	IPv4      IPFamily = "ipv4"
	IPv6      IPFamily = "ipv6"
	DualStack IPFamily = "dual"
	Unknown   IPFamily = "unknown"
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
	case networkPluginName() == openshiftSDNPluginName && openshiftSDNMode() == networkutils.NetworkPolicyPluginName:
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
				if networkPluginName() != openshiftSDNPluginName {
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
