package networking

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-network][Feature:Multus]", func() {
	var oc *exutil.CLI
	var ns string // namespace

	oc = exutil.NewCLI("multus-e2e")

	f := oc.KubeFramework()
	podName := "multus-test-pod-"

	// Multus is already installed on origin. These tests aims to verify the integrity of the installation.

	g.It("should use multus to create net1 device from network-attachment-definition", func() {
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
		testPod := frameworkpod.CreateExecPodOrFail(f.ClientSet, ns, podName, func(pod *v1.Pod) {
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
