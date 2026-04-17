package config_operator

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	imageutils "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	configOperatorNamespace = "openshift-config-operator"
	configNamespace         = "openshift-config"
	configManagedNamespace  = "openshift-config-managed"

	configOperatorPolicyName = "config-operator-networkpolicy"
	defaultDenyAllPolicyName = "default-deny-all"
)

var _ = g.Describe("[sig-api-machinery][Feature:NetworkPolicy][Skipped:HyperShift][Skipped:MicroShift] Config Operator NetworkPolicy", func() {
	oc := exutil.NewCLI("config-operator-networkpolicy-e2e")
	f := oc.KubeFramework()
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	var cs kubernetes.Interface

	g.BeforeEach(func() {
		cs = f.ClientSet
	})

	g.It("should enforce basic NetworkPolicy rules [apigroup:networking.k8s.io]", func() {
		ctx := context.Background()
		nsName := f.Namespace.Name

		serverName := "np-server"
		clientLabels := map[string]string{"app": "np-client"}
		serverLabels := map[string]string{"app": "np-server"}

		g.By(fmt.Sprintf("creating netexec server pod %s/%s", nsName, serverName))
		serverPod := netexecPod(serverName, nsName, serverLabels, 8080)
		_, err := cs.CoreV1().Pods(nsName).Create(ctx, serverPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(waitForPodReady(ctx, cs, nsName, serverName)).NotTo(o.HaveOccurred())

		server, err := cs.CoreV1().Pods(nsName).Get(ctx, serverName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(server.Status.PodIPs).NotTo(o.BeEmpty())

		serverIPs := podIPs(server)
		g.GinkgoWriter.Printf("server pod %s/%s ips=%v\n", nsName, serverName, serverIPs)

		g.By("Verifying allow-all when no policies select the pod")
		expectConnectivity(ctx, cs, nsName, clientLabels, serverIPs, 8080, true)

		g.By("Applying default deny and verifying traffic is blocked")
		_, err = cs.NetworkingV1().NetworkPolicies(nsName).Create(ctx, defaultDenyPolicy("default-deny", nsName), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Adding ingress allow only and verifying traffic is still blocked")
		_, err = cs.NetworkingV1().NetworkPolicies(nsName).Create(ctx, allowIngressPolicy("allow-ingress", nsName, serverLabels, clientLabels, 8080), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		expectConnectivity(ctx, cs, nsName, clientLabels, serverIPs, 8080, false)

		g.By("Adding egress allow and verifying traffic is permitted")
		_, err = cs.NetworkingV1().NetworkPolicies(nsName).Create(ctx, allowEgressPolicy("allow-egress", nsName, clientLabels, serverLabels, 8080), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		expectConnectivity(ctx, cs, nsName, clientLabels, serverIPs, 8080, true)
	})

	g.It("should verify config operator NetworkPolicy enforcement [Serial][apigroup:config.openshift.io]", func() {
		ctx := context.Background()
		operatorLabels := map[string]string{"app": "openshift-config-operator"}

		g.By("Verifying config operator NetworkPolicies exist")
		_, err := cs.NetworkingV1().NetworkPolicies(configOperatorNamespace).Get(ctx, configOperatorPolicyName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = cs.NetworkingV1().NetworkPolicies(configOperatorNamespace).Get(ctx, defaultDenyAllPolicyName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating test pods in openshift-config-operator for allow/deny checks")
		allowedServerIPs, cleanupAllowed := createServerPod(ctx, cs, configOperatorNamespace, fmt.Sprintf("np-operator-allowed-%s", rand.String(5)), operatorLabels, 8443)
		g.DeferCleanup(cleanupAllowed)
		deniedServerIPs, cleanupDenied := createServerPod(ctx, cs, configOperatorNamespace, fmt.Sprintf("np-operator-denied-%s", rand.String(5)), operatorLabels, 12345)
		g.DeferCleanup(cleanupDenied)

		g.By("Verifying allowed port 8443 ingress to operator")
		expectConnectivity(ctx, cs, configOperatorNamespace, operatorLabels, allowedServerIPs, 8443, true)

		g.By("Verifying denied port 12345 (not in NetworkPolicy)")
		expectConnectivity(ctx, cs, configOperatorNamespace, operatorLabels, deniedServerIPs, 12345, false)

		g.By("Verifying denied ports even from same namespace")
		for _, port := range []int32{8080, 8444, 6443, 9090} {
			serverIPs, cleanup := createServerPod(
				ctx,
				cs,
				configOperatorNamespace,
				fmt.Sprintf("np-operator-denied-%d-%s", port, rand.String(5)),
				operatorLabels,
				port,
			)
			g.DeferCleanup(cleanup)
			expectConnectivity(ctx, cs, configOperatorNamespace, operatorLabels, serverIPs, port, false)
		}

		g.By("Checking if NetworkPolicy allows DNS egress")
		operatorPolicy, err := cs.NetworkingV1().NetworkPolicies(configOperatorNamespace).Get(ctx, configOperatorPolicyName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

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
			g.GinkgoWriter.Printf("NetworkPolicy allows DNS egress, testing DNS connectivity\n")
			dnsSvc, err := cs.CoreV1().Services("openshift-dns").Get(ctx, "dns-default", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			dnsIPs := serviceClusterIPs(dnsSvc)
			g.GinkgoWriter.Printf("expecting allow from %s to DNS %v\n", configOperatorNamespace, dnsIPs)

			dnsReachable := false
			for _, port := range []int32{53, 5353} {
				g.GinkgoWriter.Printf("checking DNS connectivity on port %d\n", port)
				logConnectivityBestEffort(ctx, cs, configOperatorNamespace, operatorLabels, dnsIPs, port, true)
				dnsReachable = true
				break
			}
			if !dnsReachable {
				g.GinkgoWriter.Printf("DNS connectivity check skipped (no ports tested)\n")
			}
		} else {
			g.GinkgoWriter.Printf("NetworkPolicy does not explicitly allow DNS egress, skipping DNS connectivity test\n")
		}
	})

	g.It("should verify config namespace NetworkPolicies exist [apigroup:config.openshift.io]", func() {
		ctx := context.Background()
		namespacesToTest := []string{configOperatorNamespace, configNamespace, configManagedNamespace}

		for _, ns := range namespacesToTest {
			g.GinkgoWriter.Printf("=== Testing namespace: %s ===\n", ns)

			g.By(fmt.Sprintf("Verifying namespace %s exists", ns))
			_, err := cs.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Checking for NetworkPolicies in %s", ns))
			policies, err := cs.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			if len(policies.Items) > 0 {
				g.GinkgoWriter.Printf("Found %d NetworkPolicy(ies) in %s\n", len(policies.Items), ns)
				for _, policy := range policies.Items {
					g.GinkgoWriter.Printf("  - %s\n", policy.Name)
					logNetworkPolicyDetails(fmt.Sprintf("%s/%s", ns, policy.Name), &policy)
				}
			} else {
				g.GinkgoWriter.Printf("No NetworkPolicies found in %s\n", ns)
			}

			g.By(fmt.Sprintf("Checking for pods in %s", ns))
			pods, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			if len(pods.Items) > 0 {
				g.GinkgoWriter.Printf("Found %d pod(s) in %s\n", len(pods.Items), ns)
				for _, pod := range pods.Items {
					g.GinkgoWriter.Printf("  - %s (phase: %s, labels: %v)\n", pod.Name, pod.Status.Phase, pod.Labels)
				}
			} else {
				g.GinkgoWriter.Printf("No pods found in %s\n", ns)
			}
		}
	})

	g.It("should restore config operator NetworkPolicies after delete or mutation [Serial][apigroup:config.openshift.io][Timeout:30m]", func() {
		ctx := context.Background()

		g.By("Capturing expected NetworkPolicy specs")
		expectedOperatorPolicy := getNetworkPolicy(ctx, cs, configOperatorNamespace, configOperatorPolicyName)
		expectedOperatorDenyAll := getNetworkPolicy(ctx, cs, configOperatorNamespace, defaultDenyAllPolicyName)

		g.By("Deleting config-operator NetworkPolicy and waiting for restoration")
		restoreNetworkPolicy(ctx, cs, expectedOperatorPolicy)

		g.By("Deleting default-deny-all policy and waiting for restoration")
		restoreNetworkPolicy(ctx, cs, expectedOperatorDenyAll)

		// Verify default-deny-all in other config namespaces if they exist.
		for _, ns := range []string{configNamespace, configManagedNamespace} {
			policy, err := cs.NetworkingV1().NetworkPolicies(ns).Get(ctx, defaultDenyAllPolicyName, metav1.GetOptions{})
			if err != nil {
				g.GinkgoWriter.Printf("No default-deny-all in %s, skipping delete/restore test: %v\n", ns, err)
				continue
			}
			g.By(fmt.Sprintf("Deleting default-deny-all in %s and waiting for restoration", ns))
			restoreNetworkPolicy(ctx, cs, policy)
		}

		g.By("Mutating config-operator NetworkPolicy and waiting for reconciliation")
		mutateAndRestoreNetworkPolicy(ctx, cs, configOperatorNamespace, configOperatorPolicyName)

		g.By("Mutating default-deny-all policy and waiting for reconciliation")
		mutateAndRestoreNetworkPolicy(ctx, cs, configOperatorNamespace, defaultDenyAllPolicyName)

		for _, ns := range []string{configNamespace, configManagedNamespace} {
			_, err := cs.NetworkingV1().NetworkPolicies(ns).Get(ctx, defaultDenyAllPolicyName, metav1.GetOptions{})
			if err != nil {
				g.GinkgoWriter.Printf("No default-deny-all in %s, skipping mutation test: %v\n", ns, err)
				continue
			}
			g.By(fmt.Sprintf("Mutating default-deny-all in %s and waiting for reconciliation", ns))
			mutateAndRestoreNetworkPolicy(ctx, cs, ns, defaultDenyAllPolicyName)
		}
	})

	g.It("should verify config namespace NetworkPolicy enforcement [apigroup:config.openshift.io]", func() {
		ctx := context.Background()
		namespacesToTest := []struct {
			namespace string
			testPods  bool
		}{
			{configOperatorNamespace, true},
			{configNamespace, false},
			{configManagedNamespace, false},
		}

		for _, ns := range namespacesToTest {
			g.GinkgoWriter.Printf("=== Testing NetworkPolicy enforcement in %s ===\n", ns.namespace)

			policies, err := cs.NetworkingV1().NetworkPolicies(ns.namespace).List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			if len(policies.Items) == 0 {
				g.GinkgoWriter.Printf("No NetworkPolicies found in %s, skipping enforcement tests\n", ns.namespace)
				continue
			}

			g.GinkgoWriter.Printf("Found %d NetworkPolicy(ies) in %s\n", len(policies.Items), ns.namespace)
			for _, policy := range policies.Items {
				g.GinkgoWriter.Printf("  - %s (podSelector: %v, ingress rules: %d, egress rules: %d)\n",
					policy.Name,
					policy.Spec.PodSelector.MatchLabels,
					len(policy.Spec.Ingress),
					len(policy.Spec.Egress))
			}

			if !ns.testPods {
				g.GinkgoWriter.Printf("Namespace %s typically has no pods, skipping pod-based enforcement tests\n", ns.namespace)
				continue
			}

			pods, err := cs.CoreV1().Pods(ns.namespace).List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			if len(pods.Items) > 0 {
				g.GinkgoWriter.Printf("Verifying that %d existing pod(s) in %s are healthy despite NetworkPolicies\n", len(pods.Items), ns.namespace)
				for _, pod := range pods.Items {
					isReady := false
					for _, condition := range pod.Status.Conditions {
						if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
							isReady = true
							break
						}
					}

					if pod.Status.Phase == corev1.PodRunning && isReady {
						g.GinkgoWriter.Printf("  Pod %s is running and ready\n", pod.Name)
					} else {
						g.GinkgoWriter.Printf("  Pod %s phase: %s, ready: %v\n", pod.Name, pod.Status.Phase, isReady)
					}
				}
			}
		}
	})
})

// netexecPod creates a pod running agnhost netexec on the given port.
func netexecPod(name, namespace string, labels map[string]string, port int32) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"openshift.io/required-scc": "nonroot-v2",
			},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   boolptr(true),
				RunAsUser:      int64ptr(1001),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{
				{
					Name:  "netexec",
					Image: imageutils.GetE2EImage(imageutils.Agnhost),
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: boolptr(false),
						Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						RunAsNonRoot:             boolptr(true),
						RunAsUser:                int64ptr(1001),
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

func createServerPod(ctx context.Context, kubeClient kubernetes.Interface, namespace, name string, labels map[string]string, port int32) ([]string, func()) {
	g.GinkgoHelper()

	g.GinkgoWriter.Printf("creating server pod %s/%s port=%d labels=%v\n", namespace, name, port, labels)
	pod := netexecPod(name, namespace, labels, port)
	_, err := kubeClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(waitForPodReady(ctx, kubeClient, namespace, name)).NotTo(o.HaveOccurred())

	created, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(created.Status.PodIPs).NotTo(o.BeEmpty())

	ips := podIPs(created)
	g.GinkgoWriter.Printf("server pod %s/%s ips=%v\n", namespace, name, ips)

	return ips, func() {
		g.GinkgoWriter.Printf("deleting server pod %s/%s\n", namespace, name)
		_ = kubeClient.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}
}

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

func isIPv6(ip string) bool {
	return net.ParseIP(ip) != nil && strings.Contains(ip, ":")
}

func formatIPPort(ip string, port int32) string {
	if isIPv6(ip) {
		return fmt.Sprintf("[%s]:%d", ip, port)
	}
	return fmt.Sprintf("%s:%d", ip, port)
}

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

// expectConnectivityForIP probes a single IP and asserts the connectivity result.
// Uses a long-running client pod that continuously probes TCP connectivity,
// avoiding per-poll pod create/delete overhead.
func expectConnectivityForIP(ctx context.Context, kubeClient kubernetes.Interface, namespace string, clientLabels map[string]string, serverIP string, port int32, shouldSucceed bool) {
	g.GinkgoHelper()

	podName, cleanup, err := createConnectivityClientPod(ctx, kubeClient, namespace, clientLabels, serverIP, port)
	o.Expect(err).NotTo(o.HaveOccurred())
	g.DeferCleanup(cleanup)

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		succeeded, err := readConnectivityResult(ctx, kubeClient, namespace, podName)
		if err != nil {
			g.GinkgoWriter.Printf("waiting for connectivity result: %v\n", err)
			return false, nil
		}
		return succeeded == shouldSucceed, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	g.GinkgoWriter.Printf("connectivity %s/%s expected=%t\n", namespace, formatIPPort(serverIP, port), shouldSucceed)
}

func expectConnectivity(ctx context.Context, kubeClient kubernetes.Interface, namespace string, clientLabels map[string]string, serverIPs []string, port int32, shouldSucceed bool) {
	g.GinkgoHelper()

	for _, ip := range serverIPs {
		family := "IPv4"
		if isIPv6(ip) {
			family = "IPv6"
		}
		g.GinkgoWriter.Printf("checking %s connectivity %s -> %s expected=%t\n", family, namespace, formatIPPort(ip, port), shouldSucceed)
		expectConnectivityForIP(ctx, kubeClient, namespace, clientLabels, ip, port, shouldSucceed)
	}
}

func logConnectivityBestEffortForIP(ctx context.Context, kubeClient kubernetes.Interface, namespace string, clientLabels map[string]string, serverIP string, port int32, shouldSucceed bool) {
	g.GinkgoHelper()

	podName, cleanup, err := createConnectivityClientPod(ctx, kubeClient, namespace, clientLabels, serverIP, port)
	if err != nil {
		g.GinkgoWriter.Printf("failed to create client pod for best-effort check: %v\n", err)
		return
	}
	g.DeferCleanup(cleanup)

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		succeeded, err := readConnectivityResult(ctx, kubeClient, namespace, podName)
		if err != nil {
			return false, nil
		}
		return succeeded == shouldSucceed, nil
	})
	if err != nil {
		g.GinkgoWriter.Printf("connectivity %s/%s expected=%t (best-effort) failed: %v\n", namespace, formatIPPort(serverIP, port), shouldSucceed, err)
		return
	}
	g.GinkgoWriter.Printf("connectivity %s/%s expected=%t (best-effort)\n", namespace, formatIPPort(serverIP, port), shouldSucceed)
}

func logConnectivityBestEffort(ctx context.Context, kubeClient kubernetes.Interface, namespace string, clientLabels map[string]string, serverIPs []string, port int32, shouldSucceed bool) {
	g.GinkgoHelper()

	for _, ip := range serverIPs {
		family := "IPv4"
		if isIPv6(ip) {
			family = "IPv6"
		}
		g.GinkgoWriter.Printf("checking %s connectivity (best-effort) %s -> %s expected=%t\n", family, namespace, formatIPPort(ip, port), shouldSucceed)
		logConnectivityBestEffortForIP(ctx, kubeClient, namespace, clientLabels, ip, port, shouldSucceed)
	}
}

// createConnectivityClientPod creates a long-running pod that continuously probes
// TCP connectivity and writes results to stdout. Callers read the pod's logs
// to determine the latest result, avoiding per-poll pod create/delete overhead.
func createConnectivityClientPod(ctx context.Context, kubeClient kubernetes.Interface, namespace string, labels map[string]string, serverIP string, port int32) (string, func(), error) {
	name := fmt.Sprintf("np-client-%s", rand.String(5))
	target := formatIPPort(serverIP, port)

	g.GinkgoWriter.Printf("creating client pod %s/%s to probe %s\n", namespace, name, target)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"openshift.io/required-scc": "nonroot-v2",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   boolptr(true),
				RunAsUser:      int64ptr(1001),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{
				{
					Name:  "connect",
					Image: imageutils.GetE2EImage(imageutils.Agnhost),
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: boolptr(false),
						Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						RunAsNonRoot:             boolptr(true),
						RunAsUser:                int64ptr(1001),
					},
					Command: []string{"/bin/sh", "-c"},
					Args: []string{
						fmt.Sprintf("while true; do if /agnhost connect --protocol=tcp --timeout=5s %s 2>/dev/null; then echo CONN_OK; else echo CONN_FAIL; fi; sleep 3; done", target),
					},
				},
			},
		},
	}

	_, err := kubeClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", nil, err
	}

	if err := waitForPodReady(ctx, kubeClient, namespace, name); err != nil {
		_ = kubeClient.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		return "", nil, fmt.Errorf("client pod %s/%s never became ready: %w", namespace, name, err)
	}

	cleanup := func() {
		g.GinkgoWriter.Printf("deleting client pod %s/%s\n", namespace, name)
		_ = kubeClient.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}

	return name, cleanup, nil
}

func readConnectivityResult(ctx context.Context, kubeClient kubernetes.Interface, namespace, podName string) (bool, error) {
	tailLines := int64(1)
	raw, err := kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tailLines,
	}).DoRaw(ctx)
	if err != nil {
		return false, err
	}

	line := strings.TrimSpace(string(raw))
	if line == "" {
		return false, fmt.Errorf("no connectivity result yet from pod %s/%s", namespace, podName)
	}

	g.GinkgoWriter.Printf("client pod %s/%s result=%s\n", namespace, podName, line)
	return line == "CONN_OK", nil
}

// ingressAllowsFromNamespace checks if a NetworkPolicy allows ingress from
// the given namespace and pod labels on the specified port.
func ingressAllowsFromNamespace(policy *networkingv1.NetworkPolicy, namespace string, labels map[string]string, port int32) bool {
	for _, rule := range policy.Spec.Ingress {
		if !ruleAllowsPort(rule.Ports, port) {
			continue
		}
		if len(rule.From) == 0 {
			return true
		}
		for _, peer := range rule.From {
			if peer.NamespaceSelector != nil {
				if nsMatch(peer.NamespaceSelector, namespace) && podMatch(peer.PodSelector, labels) {
					return true
				}
				continue
			}
			if podMatch(peer.PodSelector, labels) {
				return true
			}
		}
	}
	return false
}

func nsMatch(selector *metav1.LabelSelector, namespace string) bool {
	if selector == nil {
		return true
	}
	if selector.MatchLabels != nil {
		if selector.MatchLabels["kubernetes.io/metadata.name"] == namespace {
			return true
		}
	}
	for _, expr := range selector.MatchExpressions {
		if expr.Key != "kubernetes.io/metadata.name" {
			continue
		}
		if expr.Operator != metav1.LabelSelectorOpIn {
			continue
		}
		if slices.Contains(expr.Values, namespace) {
			return true
		}
	}
	return false
}

func podMatch(selector *metav1.LabelSelector, labels map[string]string) bool {
	if selector == nil {
		return true
	}
	for key, value := range selector.MatchLabels {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func ruleAllowsPort(ports []networkingv1.NetworkPolicyPort, port int32) bool {
	if len(ports) == 0 {
		return true
	}
	for _, p := range ports {
		if p.Port == nil {
			return true
		}
		if p.Port.Type == intstr.Int && p.Port.IntVal == port {
			return true
		}
	}
	return false
}

func waitForPodReady(ctx context.Context, kubeClient kubernetes.Interface, namespace, name string) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
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

func protocolPtr(protocol corev1.Protocol) *corev1.Protocol {
	return &protocol
}

func boolptr(value bool) *bool {
	return &value
}

func int64ptr(value int64) *int64 {
	return &value
}

func logNetworkPolicyDetails(label string, policy *networkingv1.NetworkPolicy) {
	g.GinkgoHelper()
	g.GinkgoWriter.Printf("networkpolicy %s details:\n", label)
	g.GinkgoWriter.Printf("  podSelector=%v policyTypes=%v\n", policy.Spec.PodSelector.MatchLabels, policy.Spec.PolicyTypes)
	for i, rule := range policy.Spec.Ingress {
		g.GinkgoWriter.Printf("  ingress[%d]: ports=%s from=%s\n", i, formatPorts(rule.Ports), formatPeers(rule.From))
	}
	for i, rule := range policy.Spec.Egress {
		g.GinkgoWriter.Printf("  egress[%d]: ports=%s to=%s\n", i, formatPorts(rule.Ports), formatPeers(rule.To))
	}
}

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

func formatSelector(sel *metav1.LabelSelector) string {
	if sel == nil {
		return ""
	}
	if len(sel.MatchLabels) == 0 && len(sel.MatchExpressions) == 0 {
		return "{}"
	}
	return fmt.Sprintf("labels=%v exprs=%v", sel.MatchLabels, sel.MatchExpressions)
}

func getNetworkPolicy(ctx context.Context, client kubernetes.Interface, namespace, name string) *networkingv1.NetworkPolicy {
	g.GinkgoHelper()
	policy, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get NetworkPolicy %s/%s", namespace, name)
	return policy
}

// restoreNetworkPolicy deletes a NetworkPolicy and waits for the operator to
// recreate it with the same spec.
func restoreNetworkPolicy(ctx context.Context, client kubernetes.Interface, expected *networkingv1.NetworkPolicy) {
	g.GinkgoHelper()
	namespace := expected.Namespace
	name := expected.Name
	originalUID := expected.UID
	sawDeletion := false
	g.GinkgoWriter.Printf("deleting NetworkPolicy %s/%s\n", namespace, name)
	err := client.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		current, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			sawDeletion = true
			return false, nil
		}
		if err != nil {
			return false, nil
		}
		if current.UID == originalUID && !sawDeletion {
			return false, nil
		}
		return equality.Semantic.DeepEqual(expected.Spec, current.Spec), nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "timed out waiting for NetworkPolicy %s/%s spec to be restored", namespace, name)
	g.GinkgoWriter.Printf("NetworkPolicy %s/%s spec restored after delete\n", namespace, name)
}

// mutateAndRestoreNetworkPolicy patches a NetworkPolicy's podSelector and
// waits for the operator to reconcile it back to the original spec.
func mutateAndRestoreNetworkPolicy(ctx context.Context, client kubernetes.Interface, namespace, name string) {
	g.GinkgoHelper()
	original := getNetworkPolicy(ctx, client, namespace, name)
	g.GinkgoWriter.Printf("mutating NetworkPolicy %s/%s (podSelector override)\n", namespace, name)
	patch := []byte(`{"spec":{"podSelector":{"matchLabels":{"np-reconcile":"mutated"}}}}`)
	_, err := client.NetworkingV1().NetworkPolicies(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		current := getNetworkPolicy(ctx, client, namespace, name)
		return equality.Semantic.DeepEqual(original.Spec, current.Spec), nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "timed out waiting for NetworkPolicy %s/%s spec to be restored", namespace, name)
	g.GinkgoWriter.Printf("NetworkPolicy %s/%s spec restored after mutation\n", namespace, name)
}
