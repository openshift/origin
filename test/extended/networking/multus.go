package networking

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network][Feature:Multus]", func() {
	var oc *exutil.CLI
	var ns string // namespace

	oc = exutil.NewCLIWithPodSecurityLevel("multus-e2e", admissionapi.LevelBaseline)

	f := oc.KubeFramework()
	podName := "multus-test-pod-"

	// Multus is already installed on origin. These tests aims to verify the integrity of the installation.

	g.It("should use multus to create net1 device from network-attachment-definition [apigroup:k8s.cni.cncf.io]", g.Label("Size:M"), func() {
		var err error
		ns = f.Namespace.Name

		g.By("creating a net-attach-def using bridgeCNI")
		nad_yaml := exutil.FixturePath("testdata", "net-attach-defs", "bridge-nad.yml")
		g.By(fmt.Sprintf("calling oc create -f %s", nad_yaml))
		err = oc.AsAdmin().Run("create").Args("-f", nad_yaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "created net-attach-def")

		g.By("launching pod with an annotation to use the net-attach-def")
		annotation := map[string]string{
			"k8s.v1.cni.cncf.io/networks": "bridge-nad",
		}
		testPod := frameworkpod.CreateExecPodOrFail(context.TODO(), f.ClientSet, ns, podName, func(pod *v1.Pod) {
			pod.ObjectMeta.Annotations = annotation
		})

		g.By("checking for net1 interface on pod")
		output, err := oc.AsAdmin().Run("exec").Args(testPod.Name, "--", "ip", "a", "show", "dev", "net1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Println(output)
		o.Expect(output).Should(o.ContainSubstring("net1"))
		o.Expect(output).Should(o.ContainSubstring("inet 10.10.0.1/24"))
	})

	// TODO: additional multus tests (see https://issues.redhat.com/browse/SDN-1180)
})
