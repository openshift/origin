package networking

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network][Feature:MultiNetworkPolicy]", func() {

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

	g.It("should enforce a network policies on secondary network", func() {
		var err error
		ns := f.Namespace.Name

		g.By("creating a macvlan net-attach-def")
		nad_yaml := exutil.FixturePath("testdata", "net-attach-defs", "macvlan-nad.yml")
		err = oc.AsAdmin().Run("create").Args("-f", nad_yaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("launching pod with an annotation to use the net-attach-def")
		networksTemplate := `[
			  {
				"name": "macvlan1-nad",		
				"ips": ["%s"]
			  }
			]`

		podName := "multinetpol-test-pod-"

		frameworkpod.CreateExecPodOrFail(f.ClientSet, ns, podName, func(pod *v1.Pod) {
			pod.Spec.Containers[0].Args = []string{"net", "--serve", "192.0.2.1:8889"}
			pod.ObjectMeta.Labels = map[string]string{"pod": "a"}
			pod.ObjectMeta.Annotations = map[string]string{
				"k8s.v1.cni.cncf.io/networks": fmt.Sprintf(networksTemplate, "192.0.2.1/24")}
		})

		testPodB := frameworkpod.CreateExecPodOrFail(f.ClientSet, ns, podName, func(pod *v1.Pod) {
			pod.ObjectMeta.Labels = map[string]string{"pod": "b"}
			pod.ObjectMeta.Annotations = map[string]string{
				"k8s.v1.cni.cncf.io/networks": fmt.Sprintf(networksTemplate, "192.0.2.2/24")}
		})

		frameworkpod.CreateExecPodOrFail(f.ClientSet, ns, podName, func(pod *v1.Pod) {
			pod.Spec.Containers[0].Args = []string{"net", "--serve", "192.0.2.3:8889"}
			pod.ObjectMeta.Labels = map[string]string{"pod": "c"}
			pod.ObjectMeta.Annotations = map[string]string{
				"k8s.v1.cni.cncf.io/networks": fmt.Sprintf(networksTemplate, "192.0.2.3/24")}
		})

		g.By("checking podB can connnect to podA")
		podShouldReach(oc, testPodB.Name, "192.0.2.1:8889")

		g.By("enabling MultiNetworkPolicies on cluster")
		enableMultiNetworkPolicy(oc)

		g.By("creating a deny-all-ingress traffic to pod:a MultiNetworkPolicy")
		multinetpolicy_yaml := exutil.FixturePath("testdata", "multinetpolicy", "deny-ingress-pod-a.yml")
		err = oc.AsAdmin().Run("create").Args("-f", multinetpolicy_yaml).Execute()
		o.Expect(err).To(o.Succeed())

		g.By("checking podB can NOT connnect to podA")
		podShouldNotReach(oc, testPodB.Name, "192.0.2.1:8889")

		g.By("checking podB can still connnect to podC")
		podShouldReach(oc, testPodB.Name, "192.0.2.3:8889")
	})

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
	}, "20s", "1s").Should(o.HaveOccurred())
}

func isMultinetNetworkPolicyEnabled(oc *exutil.CLI) bool {
	c := oc.AdminOperatorClient().OperatorV1().Networks()
	config := getCluserNetwork(c)
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
