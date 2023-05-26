package networking

import (
	"context"
	"fmt"
	"net"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

type podAddressSet struct {
	A *net.IPNet
	B *net.IPNet
	C *net.IPNet
}

var _ = g.Describe("[sig-network][Feature:MultiNetworkPolicy][Serial]", func() {

	oc := exutil.NewCLIWithPodSecurityLevel("multinetpol-e2e", admissionapi.LevelBaseline)
	f := oc.KubeFramework()

	var previousUseMultiNetworkPolicy bool
	g.BeforeEach(func() {
		previousUseMultiNetworkPolicy = isMultinetNetworkPolicyEnabled(oc)
	})

	g.AfterEach(func() {
		if !previousUseMultiNetworkPolicy {
			disableMultiNetworkPolicy(oc)
		}
	})

	g.DescribeTable("should enforce a network policies on secondary network", func(ctx context.Context, addrs podAddressSet) {
		var err error
		ns := f.Namespace.Name

		g.By("creating a macvlan net-attach-def")
		nad_yaml := exutil.FixturePath("testdata", "net-attach-defs", "macvlan-nad.yml")
		err = oc.AsAdmin().Run("create").Args("-f", nad_yaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		networksTemplate := `[
			  {
				"name": "macvlan1-nad",		
				"ips": ["%s"]
			  }
			]`

		podGenerateName := "multinetpol-test-pod-"
		podAListenAddress := formatHostAndPort(addrs.A.IP, 8889)
		podCListenAddress := formatHostAndPort(addrs.C.IP, 8889)

		nodes, err := e2enode.GetReadySchedulableNodes(ctx, f.ClientSet)
		o.Expect(err).To(o.Succeed())
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0))
		scheduledNode := nodes.Items[0]

		g.By("launching pods with an annotation to use the net-attach-def on node " + scheduledNode.Name)

		frameworkpod.CreateExecPodOrFail(ctx, f.ClientSet, ns, podGenerateName, func(pod *v1.Pod) {
			pod.Spec.NodeName = scheduledNode.Name
			pod.Spec.Containers[0].Args = []string{"net", "--serve", podAListenAddress}
			pod.ObjectMeta.Labels = map[string]string{"pod": "a"}
			pod.ObjectMeta.Annotations = map[string]string{
				"k8s.v1.cni.cncf.io/networks": fmt.Sprintf(networksTemplate, addrs.A.String())}
		})

		testPodB := frameworkpod.CreateExecPodOrFail(ctx, f.ClientSet, ns, podName, func(pod *v1.Pod) {
			pod.Spec.NodeName = scheduledNode.Name
			pod.ObjectMeta.Labels = map[string]string{"pod": "b"}
			pod.ObjectMeta.Annotations = map[string]string{
				"k8s.v1.cni.cncf.io/networks": fmt.Sprintf(networksTemplate, addrs.B.String())}
		})

		frameworkpod.CreateExecPodOrFail(ctx, f.ClientSet, ns, podName, func(pod *v1.Pod) {
			pod.Spec.NodeName = scheduledNode.Name
			pod.Spec.Containers[0].Args = []string{"net", "--serve", podCListenAddress}
			pod.ObjectMeta.Labels = map[string]string{"pod": "c"}
			pod.ObjectMeta.Annotations = map[string]string{
				"k8s.v1.cni.cncf.io/networks": fmt.Sprintf(networksTemplate, addrs.C.String())}
		})

		g.By("checking podB can connect to podA")
		podShouldReach(oc, testPodB.Name, podAListenAddress)

		g.By("enabling MultiNetworkPolicies on cluster")
		enableMultiNetworkPolicy(oc)

		g.By("creating a deny-all-ingress traffic to pod:a MultiNetworkPolicy")
		multinetpolicy_yaml := exutil.FixturePath("testdata", "multinetpolicy", "deny-ingress-pod-a.yml")
		err = oc.AsAdmin().Run("create").Args("-f", multinetpolicy_yaml).Execute()
		o.Expect(err).To(o.Succeed())

		g.By("checking podB can NOT connect to podA")
		podShouldNotReach(oc, testPodB.Name, podAListenAddress)

		g.By("checking podB can still connect to podC")
		podShouldReach(oc, testPodB.Name, podCListenAddress)
	},
		g.Entry("IPv4", podAddressSet{
			A: mustParseIPAndMask("192.0.2.1/24"),
			B: mustParseIPAndMask("192.0.2.2/24"),
			C: mustParseIPAndMask("192.0.2.3/24"),
		}),
		g.Entry("IPv6", podAddressSet{
			A: mustParseIPAndMask("2001:db8::1/32"),
			B: mustParseIPAndMask("2001:db8::2/32"),
			C: mustParseIPAndMask("2001:db8::3/32"),
		}),
	)

})

func enableMultiNetworkPolicy(oc *exutil.CLI) {
	c := oc.AdminOperatorClient().OperatorV1().Networks()
	config := getCluserNetwork(c)

	b := true
	config.Spec.UseMultiNetworkPolicy = &b

	_, err := c.Update(context.Background(), config, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	o.Eventually(func() error {
		return oc.AsAdmin().Run("get").Args("multi-networkpolicies.k8s.cni.cncf.io").Execute()
	}, "90s", "2s").Should(o.Succeed())

	o.Eventually(func() error {
		daemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets("openshift-multus").Get(context.Background(), "multus-networkpolicy", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("can't get openshift-multus/multus-networkpolicy daemonset: %w", err)
		}

		if daemonset.Status.NumberAvailable != daemonset.Status.DesiredNumberScheduled {
			return fmt.Errorf("openshift-multus/multus-networkpolicy daemonset is not ready [%d Available / %d Desired]",
				daemonset.Status.NumberAvailable, daemonset.Status.DesiredNumberScheduled)
		}

		return nil
	}, "30s", "2s").Should(o.Succeed())

}

func disableMultiNetworkPolicy(oc *exutil.CLI) {
	c := oc.AdminOperatorClient().OperatorV1().Networks()
	config := getCluserNetwork(c)

	b := false
	config.Spec.UseMultiNetworkPolicy = &b

	_, err := c.Update(context.Background(), config, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	o.Eventually(func() error {
		return oc.AsAdmin().Run("get").Args("multi-networkpolicies.k8s.cni.cncf.io").Execute()
	}, "90s", "1s").Should(o.HaveOccurred())

	o.Eventually(func() bool {
		_, err := oc.AdminKubeClient().AppsV1().DaemonSets("openshift-multus").Get(context.Background(), "multus-networkpolicy", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, "30s", "1s").Should(o.BeTrue())
}

func isMultinetNetworkPolicyEnabled(oc *exutil.CLI) bool {
	c := oc.AdminOperatorClient().OperatorV1().Networks()
	config := getCluserNetwork(c)

	if config.Spec.UseMultiNetworkPolicy == nil {
		return false
	}

	return *config.Spec.UseMultiNetworkPolicy
}

func getCluserNetwork(c operatorclientv1.NetworkInterface) *operatorv1.Network {
	ret, err := c.Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return ret
}

func podShouldReach(oc *exutil.CLI, podName, address string) {
	out := ""
	o.EventuallyWithOffset(1, func() error {
		var err error
		out, err = oc.AsAdmin().Run("exec").Args(podName, "--", "curl", "--connect-timeout", "1", address).Output()
		return err
	}, "30s", "1s").ShouldNot(o.HaveOccurred(), "cmd output: %s", out)
}

func podShouldNotReach(oc *exutil.CLI, podName, address string) {
	out := ""
	o.EventuallyWithOffset(1, func() error {
		var err error
		out, err = oc.AsAdmin().Run("exec").Args(podName, "--", "curl", "--connect-timeout", "1", address).Output()
		return err
	}, "30s", "1s").Should(o.HaveOccurred(), "cmd output: %s", out)
}

func mustParseIPAndMask(in string) *net.IPNet {
	ip, addr, err := net.ParseCIDR(in)
	if err != nil {
		// This function is called with literal arguments only
		return &net.IPNet{IP: net.IPv4zero}
	}

	addr.IP = ip
	return addr
}

func formatHostAndPort(ip net.IP, port int) string {
	if ip.To4() != nil {
		return fmt.Sprintf("%s:%d", ip.String(), port)
	}

	return fmt.Sprintf("[%s]:%d", ip.String(), port)
}
