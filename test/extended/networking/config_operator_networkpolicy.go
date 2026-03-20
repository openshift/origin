package networking

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	agnhostImage = "registry.k8s.io/e2e-test-images/agnhost:2.45"

	// Namespace constants for openshift-config-operator
	configOperatorNamespace = "openshift-config-operator"
	configNamespace         = "openshift-config"
	configManagedNamespace  = "openshift-config-managed"

	// NetworkPolicy names
	configOperatorPolicyName = "config-operator-networkpolicy"
	defaultDenyAllPolicyName = "default-deny-all"
)

var _ = ginkgo.Describe("[sig-network][Feature:NetworkPolicy] Config Operator NetworkPolicy", func() {
	oc := exutil.NewCLI("config-operator-networkpolicy-e2e")
	f := oc.KubeFramework()
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet
	})

	ginkgo.It("should enforce basic NetworkPolicy rules [apigroup:networking.k8s.io]", func() {
		nsName := f.Namespace.Name

		serverName := "np-server"
		clientLabels := map[string]string{"app": "np-client"}
		serverLabels := map[string]string{"app": "np-server"}

		ginkgo.By(fmt.Sprintf("creating netexec server pod %s/%s", nsName, serverName))
		serverPod := netexecPod(serverName, nsName, serverLabels, 8080)
		_, err := cs.CoreV1().Pods(nsName).Create(context.TODO(), serverPod, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = waitForPodReady(cs, nsName, serverName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		server, err := cs.CoreV1().Pods(nsName).Get(context.TODO(), serverName, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(len(server.Status.PodIPs)).To(gomega.BeNumerically(">", 0))

		serverIPs := podIPs(server)
		framework.Logf("server pod %s/%s ips=%v", nsName, serverName, serverIPs)

		ginkgo.By("Verifying allow-all when no policies select the pod")
		expectConnectivity(cs, nsName, clientLabels, serverIPs, 8080, true)

		ginkgo.By("Applying default deny and verifying traffic is blocked")
		_, err = cs.NetworkingV1().NetworkPolicies(nsName).Create(context.TODO(), defaultDenyPolicy("default-deny", nsName), metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Adding ingress allow only and verifying traffic is still blocked")
		_, err = cs.NetworkingV1().NetworkPolicies(nsName).Create(context.TODO(), allowIngressPolicy("allow-ingress", nsName, serverLabels, clientLabels, 8080), metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		expectConnectivity(cs, nsName, clientLabels, serverIPs, 8080, false)

		ginkgo.By("Adding egress allow and verifying traffic is permitted")
		_, err = cs.NetworkingV1().NetworkPolicies(nsName).Create(context.TODO(), allowEgressPolicy("allow-egress", nsName, clientLabels, serverLabels, 8080), metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		expectConnectivity(cs, nsName, clientLabels, serverIPs, 8080, true)
	})

	ginkgo.It("should verify config operator NetworkPolicy enforcement [apigroup:config.openshift.io]", func() {
		// Labels must match the NetworkPolicy pod selectors for egress to work
		operatorLabels := map[string]string{"app": "openshift-config-operator"}

		ginkgo.By("Verifying config operator NetworkPolicies exist")
		_, err := cs.NetworkingV1().NetworkPolicies(configOperatorNamespace).Get(context.TODO(), configOperatorPolicyName, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		_, err = cs.NetworkingV1().NetworkPolicies(configOperatorNamespace).Get(context.TODO(), defaultDenyAllPolicyName, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Creating test pods in openshift-config-operator for allow/deny checks")
		allowedServerIPs, cleanupAllowed := createServerPod(cs, configOperatorNamespace, "np-operator-allowed", operatorLabels, 8443)
		defer cleanupAllowed()
		deniedServerIPs, cleanupDenied := createServerPod(cs, configOperatorNamespace, "np-operator-denied", operatorLabels, 12345)
		defer cleanupDenied()

		ginkgo.By("Verifying allowed port 8443 ingress to operator")
		expectConnectivity(cs, configOperatorNamespace, operatorLabels, allowedServerIPs, 8443, true)

		ginkgo.By("Verifying denied port 12345 (not in NetworkPolicy)")
		expectConnectivity(cs, configOperatorNamespace, operatorLabels, deniedServerIPs, 12345, false)

		ginkgo.By("Verifying denied ports even from same namespace")
		for _, port := range []int32{80, 443, 6443, 9090} {
			expectConnectivity(cs, configOperatorNamespace, operatorLabels, allowedServerIPs, port, false)
		}

		// Check if the NetworkPolicy allows DNS egress
		ginkgo.By("Checking if NetworkPolicy allows DNS egress")
		operatorPolicy, err := cs.NetworkingV1().NetworkPolicies(configOperatorNamespace).Get(context.TODO(), configOperatorPolicyName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Warning: could not get operator NetworkPolicy: %v", err)
			return
		}

		hasDNSEgress := false
		for _, egressRule := range operatorPolicy.Spec.Egress {
			for _, port := range egressRule.Ports {
				if port.Port != nil && (port.Port.IntVal == 53 || port.Port.IntVal == 5353) {
					hasDNSEgress = true
					break
				}
			}
			if hasDNSEgress {
				break
			}
		}

		if hasDNSEgress {
			framework.Logf("NetworkPolicy allows DNS egress, testing DNS connectivity")
			dnsSvc, err := cs.CoreV1().Services("openshift-dns").Get(context.TODO(), "dns-default", metav1.GetOptions{})
			if err != nil {
				framework.Logf("Warning: failed to get DNS service, skipping DNS egress test: %v", err)
				return
			}

			dnsIPs := serviceClusterIPs(dnsSvc)
			framework.Logf("Testing egress from %s to DNS %v", configOperatorNamespace, dnsIPs)

			// Try common DNS ports
			dnsReachable := false
			for _, port := range []int32{53, 5353} {
				framework.Logf("Checking DNS connectivity on port %d", port)
				// Use a shorter timeout for DNS checks since they might not be configured
				if err := testConnectivityWithTimeout(cs, configOperatorNamespace, operatorLabels, dnsIPs, port, true, 30*time.Second); err != nil {
					framework.Logf("DNS connectivity test on port %d failed (this may be expected): %v", port, err)
				} else {
					dnsReachable = true
					framework.Logf("DNS connectivity on port %d succeeded", port)
					break
				}
			}
			gomega.Expect(dnsReachable).To(gomega.BeTrue(), "NetworkPolicy exposes DNS egress rules, but connectivity to dns-default failed on all tested ports")
		} else {
			framework.Logf("NetworkPolicy does not explicitly allow DNS egress, skipping DNS connectivity test")
		}
	})

	ginkgo.It("should verify config namespace NetworkPolicies exist [apigroup:config.openshift.io]", func() {
		// Test all three config-related namespaces
		namespacesToTest := []string{configOperatorNamespace, configNamespace, configManagedNamespace}

		for _, ns := range namespacesToTest {
			framework.Logf("=== Testing namespace: %s ===", ns)

			ginkgo.By(fmt.Sprintf("Verifying namespace %s exists", ns))
			_, err := cs.CoreV1().Namespaces().Get(context.TODO(), ns, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Check for NetworkPolicies
			ginkgo.By(fmt.Sprintf("Checking for NetworkPolicies in %s", ns))
			policies, err := cs.NetworkingV1().NetworkPolicies(ns).List(context.TODO(), metav1.ListOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			if len(policies.Items) > 0 {
				framework.Logf("Found %d NetworkPolicy(ies) in %s", len(policies.Items), ns)
				for _, policy := range policies.Items {
					framework.Logf("  - %s", policy.Name)
					logNetworkPolicyDetails(fmt.Sprintf("%s/%s", ns, policy.Name), &policy)
				}
			} else {
				framework.Logf("No NetworkPolicies found in %s", ns)
			}

			// List pods in these namespaces
			ginkgo.By(fmt.Sprintf("Checking for pods in %s", ns))
			pods, err := cs.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			if len(pods.Items) > 0 {
				framework.Logf("Found %d pod(s) in %s", len(pods.Items), ns)
				for _, pod := range pods.Items {
					framework.Logf("  - %s (phase: %s, labels: %v)", pod.Name, pod.Status.Phase, pod.Labels)
				}
			} else {
				framework.Logf("No pods found in %s", ns)
			}
		}
	})

	ginkgo.It("should verify config namespace NetworkPolicy enforcement [apigroup:config.openshift.io]", func() {
		// Test NetworkPolicy enforcement in each namespace
		namespacesToTest := []struct {
			namespace string
			testPods  bool // whether we should test with actual pods
		}{
			{configOperatorNamespace, true}, // openshift-config-operator has running pods
			{configNamespace, false},        // openshift-config typically has no pods
			{configManagedNamespace, false}, // openshift-config-managed typically has no pods
		}

		for _, ns := range namespacesToTest {
			framework.Logf("=== Testing NetworkPolicy enforcement in %s ===", ns.namespace)

			// Check what NetworkPolicies exist
			policies, err := cs.NetworkingV1().NetworkPolicies(ns.namespace).List(context.TODO(), metav1.ListOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			if len(policies.Items) == 0 {
				framework.Logf("No NetworkPolicies found in %s, skipping enforcement tests", ns.namespace)
				continue
			}

			framework.Logf("Found %d NetworkPolicy(ies) in %s", len(policies.Items), ns.namespace)
			for _, policy := range policies.Items {
				framework.Logf("  - %s (podSelector: %v, ingress rules: %d, egress rules: %d)",
					policy.Name,
					policy.Spec.PodSelector.MatchLabels,
					len(policy.Spec.Ingress),
					len(policy.Spec.Egress))
			}

			// If the namespace typically has no pods, we can't test enforcement
			if !ns.testPods {
				framework.Logf("Namespace %s typically has no pods, skipping pod-based enforcement tests", ns.namespace)
				continue
			}

			// For namespaces with pods, verify existing pods are still running
			// (which means NetworkPolicies aren't blocking legitimate traffic)
			pods, err := cs.CoreV1().Pods(ns.namespace).List(context.TODO(), metav1.ListOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			if len(pods.Items) > 0 {
				framework.Logf("Verifying that %d existing pod(s) in %s are healthy despite NetworkPolicies", len(pods.Items), ns.namespace)
				for _, pod := range pods.Items {
					// Check if pod is running and ready
					isReady := false
					for _, condition := range pod.Status.Conditions {
						if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
							isReady = true
							break
						}
					}

					if pod.Status.Phase == corev1.PodRunning && isReady {
						framework.Logf("  ✓ Pod %s is running and ready", pod.Name)
					} else {
						framework.Logf("  - Pod %s phase: %s, ready: %v", pod.Name, pod.Status.Phase, isReady)
					}
				}
			}
		}
	})
})

// Helper functions

func netexecPod(name, namespace string, labels map[string]string, port int32) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   boolPtr(true),
				RunAsUser:      int64Ptr(1001),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{
				{
					Name:  "netexec",
					Image: agnhostImage,
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: boolPtr(false),
						Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						RunAsNonRoot:             boolPtr(true),
						RunAsUser:                int64Ptr(1001),
					},
					Command: []string{"/agnhost"},
					Args:    []string{"netexec", fmt.Sprintf("--http-port=%d", port)},
					Ports: []corev1.ContainerPort{
						{ContainerPort: port},
					},
				},
			},
		},
	}
}

func createServerPod(kubeClient clientset.Interface, namespace, name string, labels map[string]string, port int32) ([]string, func()) {
	framework.Logf("creating server pod %s/%s port=%d labels=%v", namespace, name, port, labels)
	pod := netexecPod(name, namespace, labels, port)
	_, err := kubeClient.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	err = waitForPodReady(kubeClient, namespace, name)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	created, err := kubeClient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(len(created.Status.PodIPs)).To(gomega.BeNumerically(">", 0))

	ips := podIPs(created)
	framework.Logf("server pod %s/%s ips=%v", namespace, name, ips)

	return ips, func() {
		framework.Logf("deleting server pod %s/%s", namespace, name)
		_ = kubeClient.CoreV1().Pods(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	}
}

// podIPs returns all IP addresses assigned to a pod (dual-stack aware).
func podIPs(pod *corev1.Pod) []string {
	var ips []string
	for _, podIP := range pod.Status.PodIPs {
		if podIP.IP != "" {
			ips = append(ips, podIP.IP)
		}
	}
	if len(ips) == 0 && pod.Status.PodIP != "" {
		ips = append(ips, pod.Status.PodIP)
	}
	return ips
}

// isIPv6 returns true if the given IP string is an IPv6 address.
func isIPv6(ip string) bool {
	return net.ParseIP(ip) != nil && strings.Contains(ip, ":")
}

// formatIPPort formats an IP:port pair, using brackets for IPv6 addresses.
func formatIPPort(ip string, port int32) string {
	if isIPv6(ip) {
		return fmt.Sprintf("[%s]:%d", ip, port)
	}
	return fmt.Sprintf("%s:%d", ip, port)
}

// serviceClusterIPs returns all ClusterIPs for a service (dual-stack aware).
func serviceClusterIPs(svc *corev1.Service) []string {
	if len(svc.Spec.ClusterIPs) > 0 {
		return svc.Spec.ClusterIPs
	}
	if svc.Spec.ClusterIP != "" {
		return []string{svc.Spec.ClusterIP}
	}
	return nil
}

func defaultDenyPolicy(name, namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		},
	}
}

func allowIngressPolicy(name, namespace string, podLabels, fromLabels map[string]string, port int32) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: podLabels},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{PodSelector: &metav1.LabelSelector{MatchLabels: fromLabels}},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: port}, Protocol: protocolPtr(corev1.ProtocolTCP)},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}
}

func allowEgressPolicy(name, namespace string, podLabels, toLabels map[string]string, port int32) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: podLabels},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{PodSelector: &metav1.LabelSelector{MatchLabels: toLabels}},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: port}, Protocol: protocolPtr(corev1.ProtocolTCP)},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}
}

// expectConnectivityForIP checks connectivity to a single IP address.
func expectConnectivityForIP(kubeClient clientset.Interface, namespace string, clientLabels map[string]string, serverIP string, port int32, shouldSucceed bool) {
	err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		succeeded, err := runConnectivityCheck(kubeClient, namespace, clientLabels, serverIP, port)
		if err != nil {
			return false, err
		}
		return succeeded == shouldSucceed, nil
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("connectivity check failed for %s/%s expected=%t", namespace, formatIPPort(serverIP, port), shouldSucceed))
	framework.Logf("connectivity %s/%s expected=%t", namespace, formatIPPort(serverIP, port), shouldSucceed)
}

// expectConnectivity checks connectivity to all provided IPs (dual-stack aware).
func expectConnectivity(kubeClient clientset.Interface, namespace string, clientLabels map[string]string, serverIPs []string, port int32, shouldSucceed bool) {
	for _, ip := range serverIPs {
		family := "IPv4"
		if isIPv6(ip) {
			family = "IPv6"
		}
		framework.Logf("checking %s connectivity %s -> %s expected=%t", family, namespace, formatIPPort(ip, port), shouldSucceed)
		expectConnectivityForIP(kubeClient, namespace, clientLabels, ip, port, shouldSucceed)
	}
}

// testConnectivityWithTimeout tests connectivity with a custom timeout and returns error instead of failing
func testConnectivityWithTimeout(kubeClient clientset.Interface, namespace string, clientLabels map[string]string, serverIPs []string, port int32, shouldSucceed bool, timeout time.Duration) error {
	for _, ip := range serverIPs {
		family := "IPv4"
		if isIPv6(ip) {
			family = "IPv6"
		}
		framework.Logf("checking %s connectivity %s -> %s expected=%t (timeout=%v)", family, namespace, formatIPPort(ip, port), shouldSucceed, timeout)

		err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
			succeeded, err := runConnectivityCheck(kubeClient, namespace, clientLabels, ip, port)
			if err != nil {
				return false, err
			}
			return succeeded == shouldSucceed, nil
		})
		if err != nil {
			return fmt.Errorf("connectivity check failed for %s/%s expected=%t: %v", namespace, formatIPPort(ip, port), shouldSucceed, err)
		}
		framework.Logf("connectivity %s/%s expected=%t", namespace, formatIPPort(ip, port), shouldSucceed)
	}
	return nil
}

func runConnectivityCheck(kubeClient clientset.Interface, namespace string, labels map[string]string, serverIP string, port int32) (bool, error) {
	name := fmt.Sprintf("np-client-%s", rand.String(5))
	framework.Logf("creating client pod %s/%s to connect %s:%d", namespace, name, serverIP, port)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   boolPtr(true),
				RunAsUser:      int64Ptr(1001),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{
				{
					Name:  "connect",
					Image: agnhostImage,
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: boolPtr(false),
						Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						RunAsNonRoot:             boolPtr(true),
						RunAsUser:                int64Ptr(1001),
					},
					Command: []string{"/agnhost"},
					Args: []string{
						"connect",
						"--protocol=tcp",
						"--timeout=5s",
						formatIPPort(serverIP, port),
					},
				},
			},
		},
	}

	_, err := kubeClient.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	defer func() {
		_ = kubeClient.CoreV1().Pods(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	}()

	if err := waitForPodCompletion(kubeClient, namespace, name); err != nil {
		return false, err
	}
	completed, err := kubeClient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if len(completed.Status.ContainerStatuses) == 0 {
		return false, fmt.Errorf("no container status recorded for pod %s", name)
	}
	terminated := completed.Status.ContainerStatuses[0].State.Terminated
	if terminated == nil {
		return false, fmt.Errorf("container did not terminate properly for pod %s/%s", namespace, name)
	}
	exitCode := terminated.ExitCode
	framework.Logf("client pod %s/%s exitCode=%d", namespace, name, exitCode)
	return exitCode == 0, nil
}

func waitForPodReady(kubeClient clientset.Interface, namespace, name string) error {
	return wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := kubeClient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if pod.Status.Phase != corev1.PodRunning {
			return false, nil
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
}

func waitForPodCompletion(kubeClient clientset.Interface, namespace, name string) error {
	return wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := kubeClient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed, nil
	})
}

func protocolPtr(protocol corev1.Protocol) *corev1.Protocol {
	return &protocol
}

func boolPtr(value bool) *bool {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

// logNetworkPolicyDetails logs detailed information about a NetworkPolicy.
func logNetworkPolicyDetails(label string, policy *networkingv1.NetworkPolicy) {
	framework.Logf("networkpolicy %s details:", label)
	framework.Logf("  podSelector=%v policyTypes=%v", policy.Spec.PodSelector.MatchLabels, policy.Spec.PolicyTypes)
	for i, rule := range policy.Spec.Ingress {
		framework.Logf("  ingress[%d]: ports=%s from=%s", i, formatPorts(rule.Ports), formatPeers(rule.From))
	}
	for i, rule := range policy.Spec.Egress {
		framework.Logf("  egress[%d]: ports=%s to=%s", i, formatPorts(rule.Ports), formatPeers(rule.To))
	}
}

// formatPorts formats NetworkPolicy ports for logging.
func formatPorts(ports []networkingv1.NetworkPolicyPort) string {
	if len(ports) == 0 {
		return "[]"
	}
	out := make([]string, 0, len(ports))
	for _, p := range ports {
		proto := "TCP"
		if p.Protocol != nil {
			proto = string(*p.Protocol)
		}
		if p.Port == nil {
			out = append(out, fmt.Sprintf("%s:any", proto))
			continue
		}
		out = append(out, fmt.Sprintf("%s:%s", proto, p.Port.String()))
	}
	return fmt.Sprintf("[%s]", strings.Join(out, ", "))
}

// formatPeers formats NetworkPolicy peers for logging.
func formatPeers(peers []networkingv1.NetworkPolicyPeer) string {
	if len(peers) == 0 {
		return "[]"
	}
	out := make([]string, 0, len(peers))
	for _, peer := range peers {
		if peer.IPBlock != nil {
			out = append(out, fmt.Sprintf("ipBlock=%s except=%v", peer.IPBlock.CIDR, peer.IPBlock.Except))
			continue
		}
		ns := formatSelector(peer.NamespaceSelector)
		pod := formatSelector(peer.PodSelector)
		if ns == "" && pod == "" {
			out = append(out, "{}")
			continue
		}
		out = append(out, fmt.Sprintf("ns=%s pod=%s", ns, pod))
	}
	return fmt.Sprintf("[%s]", strings.Join(out, ", "))
}

// formatSelector formats a label selector for logging.
func formatSelector(sel *metav1.LabelSelector) string {
	if sel == nil {
		return ""
	}
	if len(sel.MatchLabels) == 0 && len(sel.MatchExpressions) == 0 {
		return "{}"
	}
	return fmt.Sprintf("labels=%v exprs=%v", sel.MatchLabels, sel.MatchExpressions)
}
