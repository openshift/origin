package etcd

import (
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("Etcd basic verification", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("etcd", exutil.KubeConfigPath())

	var podName string

	g.It("Check etcd pod status", func() {
		pods, err := oc.AsAdmin().KubeClient().CoreV1().Pods("openshift-etcd").List(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(pods.Items)).ToNot(o.BeZero())
		podName = pods.Items[0].Name

		nodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})

		o.Expect(len(nodes.Items)).Should(o.Equal(len(pods.Items)))

		g.By("get etcd pod status in ns openshift-etcd")
		out, _, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-etcd").Outputs()
		o.Expect(out).To(o.ContainSubstring("Running"))
		runCount := strings.Count(out, "Running")
		o.Expect(runCount).Should(o.Equal(len(pods.Items)))

		g.By("Verifying something in pod")
		result, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-etcd", podName, "whoami").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("root"))
		o.Expect(result).NotTo(o.ContainSubstring("image-streams-rhel7.json"))

	})
})
