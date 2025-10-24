package networking

import (
	"context"
	"fmt"
	"net"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/kubevirt"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

const (
	egressFWTestPod   = "egressfirewall"
	egressFWE2E       = "egress-firewall-e2e"
	wcEgressFWE2E     = "wildcard-egress-firewall-e2e"
	noEgressFWE2E     = "no-egress-firewall-e2e"
	egressFWTestImage = "registry.k8s.io/e2e-test-images/agnhost:2.56"
	oVNKManifest      = "ovnk-egressfirewall-test.yaml"
	oVNKWCManifest    = "ovnk-egressfirewall-wildcard-test.yaml"
)

var _ = g.Describe("[sig-network][Feature:EgressFirewall]", func() {

	egFwoc := exutil.NewCLIWithPodSecurityLevel(egressFWE2E, admissionapi.LevelPrivileged)
	egFwf := egFwoc.KubeFramework()
	mgmtFw := e2e.NewDefaultFramework("mgmt-framework")
	mgmtFw.SkipNamespaceCreation = true

	InOVNKubernetesContext(
		func() {
			g.It("should ensure egressfirewall is created", func() {
				doEgressFwTest(egFwf, mgmtFw, egFwoc, oVNKManifest, true, false)
			})
		},
	)

	noegFwoc := exutil.NewCLIWithPodSecurityLevel(noEgressFWE2E, admissionapi.LevelBaseline)
	noegFwf := noegFwoc.KubeFramework()
	g.It("egressFirewall should have no impact outside its namespace", func() {
		g.By("creating test pod")
		pod := "dummy"
		o.Expect(createTestEgressFw(noegFwf, pod)).To(o.Succeed())
		// Skip EgressFw test if we cannot reach to external servers
		if !checkConnectivityToExternalHost(noegFwf, noegFwoc, pod) {
			e2e.Logf("Skip doing egress firewall")
			deleteTestEgressFw(noegFwf)
			return
		}
		g.By("sending traffic should all pass with no egress firewall impact")
		infra, err := noegFwoc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster-wide infrastructure")

		if platformSupportICMP(infra.Status.PlatformStatus.Type, mgmtFw) {
			_, err := noegFwoc.Run("exec").Args(pod, "--", "ping", "-c", "1", "8.8.8.8").Output()
			expectNoError(err)

			_, err = noegFwoc.Run("exec").Args(pod, "--", "ping", "-c", "1", "1.1.1.1").Output()
			expectNoError(err)
		}
		_, err = noegFwoc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m5", "https://redhat.com").Output()
		expectNoError(err)

		_, err = noegFwoc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m5", "http://www.google.com:80").Output()
		expectNoError(err)
		deleteTestEgressFw(noegFwf)
	})
})

var _ = g.Describe("[sig-network][OCPFeatureGate:DNSNameResolver][Feature:EgressFirewall]", func() {
	// When OVNKubernetes subnet and coredns-ocp-dnsnameresolver plugins are enabled.
	// coredns-ocp-dnsnameresolver plugin is a TechPreview feature.
	// TODO:
	// - Merge this section with main section when feature is GA.
	// - Merge oVNKManifest & oVNKWCManifest contents.
	// - Update doEgressFwTest and sendEgressFwTraffic functions.
	wcEgFwOc := exutil.NewCLIWithPodSecurityLevel(wcEgressFWE2E, admissionapi.LevelPrivileged)
	wcEgFwF := wcEgFwOc.KubeFramework()
	mgmtFramework := e2e.NewDefaultFramework("mgmt-framework")
	mgmtFramework.SkipNamespaceCreation = true
	InOVNKubernetesContext(
		func() {
			g.It("should ensure egressfirewall with wildcard dns rules is created", func() {
				doEgressFwTest(wcEgFwF, mgmtFramework, wcEgFwOc, oVNKWCManifest, true, true)
			})
		},
	)
})

func doEgressFwTest(f *e2e.Framework, mgmtFw *e2e.Framework, oc *exutil.CLI, manifest string, nodeSelectorSupport, checkWildcard bool) error {
	g.By("creating test pod")
	o.Expect(createTestEgressFw(f, egressFWTestPod)).To(o.Succeed())

	// Skip EgressFw test if we cannot reach to external servers
	if !checkConnectivityToExternalHost(f, oc, egressFWTestPod) {
		e2e.Logf("Skip doing egress firewall")
		deleteTestEgressFw(f)
		return nil
	}
	g.By("creating an egressfirewall object")
	egFwYaml := exutil.FixturePath("testdata", "egress-firewall", manifest)

	g.By(fmt.Sprintf("calling oc create -f %s", egFwYaml))
	err := oc.AsAdmin().Run("create").Args("-f", egFwYaml).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "created egress-firewall object")

	o.Expect(sendEgressFwTraffic(f, mgmtFw, oc, egressFWTestPod, nodeSelectorSupport, checkWildcard)).To(o.Succeed())

	g.By("deleting test pod")
	deleteTestEgressFw(f)
	return err
}

func sendEgressFwTraffic(f *e2e.Framework, mgmtFw *e2e.Framework, oc *exutil.CLI, pod string, nodeSelectorSupport, checkWildcard bool) error {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster-wide infrastructure")

	if platformSupportICMP(infra.Status.PlatformStatus.Type, mgmtFw) {
		// Test ICMP / Ping to Google’s DNS IP (8.8.8.8) should pass
		// because we have allow cidr rule for 8.8.8.8
		g.By("sending traffic that matches allow cidr rule")
		_, err := oc.Run("exec").Args(pod, "--", "ping", "-c", "1", "8.8.8.8").Output()
		expectNoError(err)

		// Test ICMP / Ping to Cloudfare DNS IP (1.1.1.1) should fail
		// because there is no allow cidr match for 1.1.1.1
		g.By("sending traffic that does not match allow cidr rule")
		_, err = oc.Run("exec").Args(pod, "--", "ping", "-c", "1", "1.1.1.1").Output()
		expectError(err)
	}
	// Test curl to redhat.com should pass
	// because we have allow dns rule for redhat.com
	g.By("sending traffic that matches allow dns rule")
	_, err = oc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m5", "https://redhat.com").Output()
	expectNoError(err)

	// Test curl to amazon.com should pass
	// because we have allow dns rule for amazon.com
	g.By("sending traffic that matches allow dns rule")
	_, err = oc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m5", "https://amazon.com").Output()
	expectNoError(err)

	if checkWildcard {
		// Test curl to `www.google.com` and `translate.google.com` should pass
		// because we have allow dns rule for `*.google.com`.
		g.By("sending traffic to `www.google.com` that matches allow dns rule for `*.google.com`")
		_, err = oc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m5", "https://www.google.com").Output()
		expectNoError(err)

		g.By("sending traffic to `translate.google.com` that matches allow dns rule for `*.google.com`")
		_, err = oc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m5", "https://translate.google.com").Output()
		expectNoError(err)
	}

	// Test curl to www.redhat.com should fail
	// because we don't have allow dns rule for www.redhat.com
	g.By("sending traffic that does not match allow dns rule")
	_, err = oc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m5", "http://www.redhat.com").Output()
	expectError(err)

	if nodeSelectorSupport {
		// Access to control plane nodes should work
		g.By("sending traffic to control plane nodes should work")
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.TODO(),
			metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/control-plane"})
		expectNoError(err)
		for _, node := range nodes.Items {
			var nodeIP string
			for _, ip := range node.Status.Addresses {
				if ip.Type == kapiv1.NodeInternalIP && len(ip.Address) > 0 {
					nodeIP = ip.Address
					break
				}
			}
			o.Expect(len(nodeIP)).NotTo(o.BeZero())
			hostPort := net.JoinHostPort(nodeIP, "6443")
			url := fmt.Sprintf("https://%s", hostPort)
			_, err = oc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m5", "-k", url).Output()
			expectNoError(err)
		}
	}
	return nil
}

func checkConnectivityToExternalHost(f *e2e.Framework, oc *exutil.CLI, pod string) bool {
	g.By("executing a successful access to external internet")
	_, err := oc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m5", "http://www.google.com:80").Output()
	if err != nil {
		e2e.Logf("Unable to connect/talk to the internet: %v", err)
		return false
	}
	return true
}

func createTestEgressFw(f *e2e.Framework, pod string) error {
	makeNamespaceScheduleToAllNodes(f)

	var nodes *kapiv1.Node
	var err error
	nodes, _, err = findAppropriateNodes(f, DIFFERENT_NODE)
	if err != nil {
		return err
	}
	err = launchTestEgressFwPod(f, nodes.Name, pod)
	expectNoError(err)

	_, err = waitForTestEgressFwPod(f, pod)
	expectNoError(err)
	return nil
}

func deleteTestEgressFw(f *e2e.Framework) error {
	var zero int64
	pod := egressFWTestPod
	err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.Background(), pod, metav1.DeleteOptions{GracePeriodSeconds: &zero})
	return err
}

func launchTestEgressFwPod(f *e2e.Framework, nodeName string, podName string) error {
	contName := fmt.Sprintf("%s-container", podName)
	pod := &kapiv1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: kapiv1.PodSpec{
			Containers: []kapiv1.Container{
				{
					Name:    contName,
					Image:   image.LocationFor(egressFWTestImage),
					Command: []string{"sleep", "1000"},
				},
			},
			NodeName:      nodeName,
			RestartPolicy: kapiv1.RestartPolicyNever,
		},
	}
	_, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.Background(), pod, metav1.CreateOptions{})
	return err
}

func waitForTestEgressFwPod(f *e2e.Framework, podName string) (string, error) {
	podIP := ""
	err := frameworkpod.WaitForPodCondition(context.TODO(), f.ClientSet, f.Namespace.Name, podName, "running", podStartTimeout, func(pod *kapiv1.Pod) (bool, error) {
		podIP = pod.Status.PodIP
		return (podIP != "" && pod.Status.Phase != kapiv1.PodPending), nil
	})
	return podIP, err
}

func platformSupportICMP(platformType configv1.PlatformType, framework *e2e.Framework) bool {
	switch platformType {
	// Azure has security rules to prevent icmp response from outside the cluster by default
	case configv1.AzurePlatformType:
		return false
	case configv1.KubevirtPlatformType:
		kubevirt.SetMgmtFramework(framework)
		isAzure, err := kubevirt.MgmtClusterIsType(framework, configv1.AzurePlatformType)
		expectNoError(err)
		return !isAzure
	default:
		return true
	}
}
