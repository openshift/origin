package networking

import (
	"context"
	"fmt"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	networkingv1 "k8s.io/api/networking/v1"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	psapi "k8s.io/pod-security-admission/api"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	maxPokeRetrys = 15
	retryInterval = 1 * time.Second
)

var _ = Describe("[sig-network][Feature:Network Policy Audit logging]", func() {
	var oc *exutil.CLI
	var ns []string
	// this hook must be registered before the framework namespace teardown
	// hook
	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			// If test fails dump test pods logs
			exutil.DumpPodLogsStartingWithInNamespace("acl-logging", ns[0], oc.AsAdmin())
			exutil.DumpPodLogsStartingWithInNamespace("acl-logging", ns[1], oc.AsAdmin())
		}
	})

	oc = exutil.NewCLIWithPodSecurityLevel("acl-logging", psapi.LevelBaseline)

	// The OVNKubernetes subnet plugin should allow acl_logging for network policy.
	// For Openshift SDN and third party plugins, the behavior is unspecified and we should not run either test.
	InOVNKubernetesContext(
		func() {
			f := oc.KubeFramework()
			It("should ensure acl logs are created and correct [apigroup:project.openshift.io][apigroup:network.openshift.io]", Label("Size:M"), func() {
				ns = append(ns, f.Namespace.Name)
				makeNamespaceScheduleToAllNodes(f)
				makeNamespaceACLLoggingEnabled(oc)

				nsNoACLLog := oc.SetupProject()
				By("making namespace " + nsNoACLLog + " with acl-logging disabled")
				ns = append(ns, nsNoACLLog)

				testACLLogging(f, oc, ns)
			})
		},
	)
})

func makeNamespaceACLLoggingEnabled(oc *exutil.CLI) {
	nsName := oc.Namespace()

	By("setting the k8s.ovn.org/acl-logging annotation for the namespace: " + nsName)
	ns, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(context.Background(), nsName, metav1.GetOptions{})
	expectNoError(err)

	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string, 1)
	}
	ns.Annotations["k8s.ovn.org/acl-logging"] = `{ "deny": "alert", "allow": "alert" }`
	_, err = oc.AdminKubeClient().CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
	expectNoError(err)
}

// Test the Network policy audit logging feature
func testACLLogging(f *e2e.Framework, oc *exutil.CLI, ns []string) {
	// We launch 3 pods total; pod[0] and pod[1] will end up on node[0] in ns "acl-logging-on" , and pod[2]
	// will end up on node[1] in ns "acl-logging off", to know which acl-logging container to look in
	var nodes [2]*kapiv1.Node
	var pods [3]string
	var ips []string
	var err error
	var ipv6 bool

	nodes[0], nodes[1], err = findAppropriateNodes(f, DIFFERENT_NODE)
	expectNoError(err)

	By("making two pods in the acl-logging-on namespace and one in the acl-logging-off namespace")
	// make the first two pods in ns acl-logging-on on node[0] and the other in acl-logging-off on node[1]
	for i := range pods {
		pods[i] = fmt.Sprintf("acl-logging-%d", i+1)
		testPod := e2epod.NewAgnhostPod(ns[i/2], pods[i], nil, nil, nil, "netexec", "--http-port=80", "--udp-port=90")
		testPod.Spec.NodeName = nodes[i/2].Name
		_, err := f.ClientSet.CoreV1().Pods(ns[i/2]).Create(context.TODO(), testPod, metav1.CreateOptions{})
		expectNoError(err)
	}

	// make sure pods come up
	for i := range pods {
		tempIp, err := waitForACLLoggingPod(f, ns[i/2], pods[i])
		expectNoError(err)

		ips = append(ips, tempIp)
	}

	// Check if pods are have ipv6 addr
	if isIpv6(ips) {
		ipv6 = true
	}

	By("making \"default deny\" and \"allow from same namespace\" network policies")
	// a make network policy that allows ingress only from the same ns "acl-logging-on"
	_, err = makeFromSameNSPolicy(f, ns[0])
	expectNoError(err)

	By("sending traffic between acl-logging test pods and analyzing the audit logs")
	var errAllow error
	var errDeny error
	var podOut string
	var auditOut string
	allowReady := false
	denyReady := false
	allowLogFound := false
	denyLogFound := false

	// Retry here in the case where OVN acls have not been programmed yet
	for i := 1; i < maxPokeRetrys; i++ {
		// Ping pod[0] from pod[1] which should succeed since it's in the same namespace with out == ip[0]
		// Should hit the `allow-same-namespace` networkpolicy and have a response of pod[1]'s IP
		e2e.Logf("Poke pod %s from pod %s", pods[0], pods[1])
		podOut, errAllow = pokePod(oc, pods[1], ns[0], ips[0], ipv6)
		if errAllow == nil && strings.Contains(podOut, ips[1]) {
			e2e.Logf("Poke succeeded on try: %v", i)
			allowReady = true
		}
		// Ping pod[0] from pod[2] which should not succeed since it's in a different namespace
		// Should hit the all-deny policy
		e2e.Logf("Poke pod %s from pod %s", pods[0], pods[2])
		_, errDeny = pokePod(oc, pods[2], ns[1], ips[0], ipv6)
		if errDeny != nil {
			e2e.Logf("Poke failed successfully on try: %v", i)
			denyReady = true
		}

		time.Sleep(retryInterval)

		e2e.Logf("collecting the audit logs with 'oc adm node-logs' for node: %s", nodes[0].Name)
		// Ensure audit logs are there and that `oc adm node-logs` command adequately collects them
		auditOut, _, err = oc.AsAdmin().Run("adm").Args("node-logs", "--since=10s", nodes[0].Name, "--path=/ovn/acl-audit-log.log").Outputs()
		expectNoError(err)

		e2e.Logf("verifying the audit logs from node %s have the correct name and action", nodes[0].Name)
		allowLogFound, denyLogFound = verifyAuditLogs(auditOut, ns, ips, ipv6)

		// break if traffic flowed as expected and audit logs were correctly formed
		if allowReady && denyReady && allowLogFound && denyLogFound {
			break
		}

	}

	// Fail if allowed traffic was blocked
	expectNoError(errAllow)
	// Fail if allow curl response does not contain correct Ip
	if !strings.Contains(podOut, ips[1]) {
		expectNoError(fmt.Errorf("Incorrect Pod was poked, it's IP is %s", ips[1]))
	}
	// Fail if denied traffic was allowed
	expectError(errDeny)

	// Fail if correctly formed allow log was never found
	By("ensuring the correct allow log is found")
	Expect(allowLogFound).Should(Equal(true), "allow log not found in the logs\n%s", auditOut)
	// Fail if correctly formed deny log was never found
	By("ensuring the correct deny log is found")
	Expect(denyLogFound).Should(Equal(true), "deny log not found in the logs:\n%s", auditOut)
}

func pokePod(oc *exutil.CLI, srcPodName string, srcNamespace string, dstPodIP string, ipv6 bool) (string, error) {
	var url = strings.Join([]string{dstPodIP, "/clientip"}, "")

	if ipv6 {
		url = strings.Join([]string{"[", dstPodIP, "]", "/clientip"}, "")
	}

	out, _, err := oc.AsAdmin().Run("exec").Args(srcPodName, "-n", srcNamespace, "--", "curl", "-m", "1", url).Outputs()
	return out, err
}

func verifyAuditLogs(out string, ns []string, ips []string, ipv6 bool) (bool, bool) {
	ipMatchSrc := "nw_src="
	ipMatchDst := "nw_dst="
	allowLogFound := false
	denyLogFound := false

	if ipv6 {
		ipMatchSrc = "ipv6_src="
		ipMatchDst = "ipv6_dst="
	}

	e2e.Logf("Ensuring the audit log contains: '%s:allow-from-same-ns:Ingress:0\", verdict=allow' AND '%s' AND '%s'", ns[0], ipMatchSrc+ips[1], ipMatchDst+ips[0])
	e2e.Logf("Ensuring the audit log contains: '%s:Ingress\", verdict=drop' AND '%s' AND '%s'", ns[0], ipMatchSrc+ips[2], ipMatchDst+ips[0])
	// Ensure the ACL audit logs are correct
	for _, logLine := range strings.Split(out, "\n") {
		if strings.Contains(logLine, ns[0]+":allow-from-same-ns:Ingress:0\", verdict=allow") && strings.Contains(logLine, ipMatchSrc+ips[1]) &&
			strings.Contains(logLine, ipMatchDst+ips[0]) {
			allowLogFound = true
			continue
		}
		if strings.Contains(logLine, ns[0]+":Ingress\", verdict=drop") && strings.Contains(logLine, ipMatchSrc+ips[2]) &&
			strings.Contains(logLine, ipMatchDst+ips[0]) {
			denyLogFound = true
			continue
		}

	}

	return allowLogFound, denyLogFound
}

func waitForACLLoggingPod(f *e2e.Framework, namespace string, podName string) (string, error) {
	var podIP string
	err := e2epod.WaitForPodCondition(context.TODO(), f.ClientSet, namespace, podName, "running", podStartTimeout, func(pod *kapiv1.Pod) (bool, error) {
		podIP = pod.Status.PodIP
		return (podIP != "" && pod.Status.Phase != kapiv1.PodPending), nil
	})
	return podIP, err
}

func makeDenyAllPolicy(f *e2e.Framework, ns string) (*networkingv1.NetworkPolicy, error) {

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-deny-all",
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress, networkingv1.PolicyTypeIngress},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			Egress:      []networkingv1.NetworkPolicyEgressRule{},
		},
	}

	policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(ns).Create(context.TODO(), policy, metav1.CreateOptions{})

	return policy, err
}

func makeFromSameNSPolicy(f *e2e.Framework, ns string) (*networkingv1.NetworkPolicy, error) {

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "allow-from-same-ns",
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{},
				}},
			}},
		},
	}

	policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(ns).Create(context.TODO(), policy, metav1.CreateOptions{})

	return policy, err
}
