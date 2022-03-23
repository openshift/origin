package networking

import (
	"context"
	"fmt"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
)

const (
	EgressFWTestPod      = "egressfirewall"
	EgressFWE2E          = "egress-firewall-e2e"
	NoEgressFWE2E        = "no-egress-firewall-e2e"
	EgressFWTestImage    = "quay.io/redhat-developer/nfs-server:1.1"
	OVNKManifest         = "ovnk-egressfirewall-test.yaml"
	OpenShiftSDNManifest = "sdn-egressnetworkpolicy-test.yaml"
)

var _ = g.Describe("[sig-network][Feature:EgressFirewall]", func() {

	egFwoc := exutil.NewCLI(EgressFWE2E)
	egFwf := egFwoc.KubeFramework()

	// The OVNKubernetes subnet plugin supports EgressFirewall objects.
	InOVNKubernetesContext(
		func() {
			g.It("Should ensure egressfirewall is created", func() {
				doEgressFwTest(egFwf, egFwoc, OVNKManifest)
			})
		},
	)
	// For Openshift SDN its supports EgressNetworkPolicy objects
	InOpenShiftSDNContext(
		func() {
			g.It("Should ensure egressnetworkpolicy is created", func() {
				doEgressFwTest(egFwf, egFwoc, OpenShiftSDNManifest)
			})
		},
	)
	noegFwoc := exutil.NewCLI(NoEgressFWE2E)
	noegFwf := noegFwoc.KubeFramework()
	g.It("EgressFirewall should have no impact outside its namespace", func() {
		g.By("Creating test pod")
		pod := "dummy"
		o.Expect(createTestEgressFw(noegFwf, pod)).To(o.Succeed())
		g.By("Sending traffic should all pass with no egress firewall impact")
		_, err := noegFwoc.Run("exec").Args(pod, "--", "ping", "-c", "1", "8.8.8.8").Output()
		expectNoError(err)

		_, err = noegFwoc.Run("exec").Args(pod, "--", "ping", "-c", "1", "1.1.1.1").Output()
		expectNoError(err)

		_, err = noegFwoc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m1", "https://docs.openshift.com").Output()
		expectNoError(err)

		_, err = noegFwoc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m1", "http://www.google.com:80").Output()
		expectNoError(err)

	})
})

func doEgressFwTest(f *e2e.Framework, oc *exutil.CLI, manifest string) error {
	g.By("Creating test pod")
	o.Expect(createTestEgressFw(f, EgressFWTestPod)).To(o.Succeed())

	g.By("Creating an egressfirewall object")
	egFwYaml := exutil.FixturePath("testdata", "egress-firewall", manifest)

	g.By(fmt.Sprintf("Calling oc create -f %s", egFwYaml))
	err := oc.AsAdmin().Run("create").Args("-f", egFwYaml).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "created egress-firewall object")

	o.Expect(sendEgressFwTraffic(f, oc, EgressFWTestPod)).To(o.Succeed())

	g.By("Deleting test pod")
	deleteTestEgressFw(f)
	return err
}

func sendEgressFwTraffic(f *e2e.Framework, oc *exutil.CLI, pod string) error {
	// Test ICMP / Ping to Google’s DNS IP (8.8.8.8) should pass
	// because we have allow cidr rule for 8.8.8.8
	g.By("Sending traffic that matches allow cidr rule")
	_, err := oc.Run("exec").Args(pod, "--", "ping", "-c", "1", "8.8.8.8").Output()
	expectNoError(err)

	// Test ICMP / Ping to Cloudfare DNS IP (1.1.1.1) should fail
	// because there is no allow cidr match for 1.1.1.1
	g.By("Sending traffic that does not match allow cidr rule")
	_, err = oc.Run("exec").Args(pod, "--", "ping", "-c", "1", "1.1.1.1").Output()
	expectError(err)

	// Test curl to docs.openshift.com should pass
	// because we have allow dns rule for docs.openshift.com
	g.By("Sending traffic that matches allow dns rule")
	_, err = oc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m1", "https://docs.openshift.com").Output()
	expectNoError(err)

	// Test curl to www.google.com:80 should fail
	// because we don't have allow dns rule for www.google.com:80
	g.By("Sending traffic that does not match allow dns rule")
	_, err = oc.Run("exec").Args(pod, "--", "curl", "-q", "-s", "-I", "-m1", "http://www.google.com:80").Output()
	expectError(err)
	return nil
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
	pod := EgressFWTestPod
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
					Image:   image.LocationFor(EgressFWTestImage),
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
	err := frameworkpod.WaitForPodCondition(f.ClientSet, f.Namespace.Name, podName, "running", podStartTimeout, func(pod *kapiv1.Pod) (bool, error) {
		podIP = pod.Status.PodIP
		return (podIP != "" && pod.Status.Phase != kapiv1.PodPending), nil
	})
	return podIP, err
}
