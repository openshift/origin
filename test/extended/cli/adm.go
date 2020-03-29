package cli

import (
	"fmt"
	"math/rand"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc adm", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("oc-adm")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("oc-adm").AsAdmin()

	g.It("oc adm node-logs", func() {
		o.Expect(oc.Run("adm", "node-logs").Args(randomNode(oc)).Execute()).To(o.Succeed())
	})

	g.It("oc adm node-logs --role=master --since=-2m", func() {
		masters, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err := oc.Run("adm", "node-logs").Args("--role=master", "--since=-2m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, m := range masters.Items {
			if hostname, ok := m.Labels["kubernetes.io/hostname"]; ok {
				o.Expect(out).To(o.ContainSubstring(hostname))
			}
		}
	})

	g.It("oc adm node-logs --boot=0", func() {
		o.Expect(oc.Run("adm", "node-logs").Args(randomNode(oc), "--boot=0").Execute()).To(o.Succeed())
	})

	g.It("oc adm node-logs --since=-2m --until=-1m", func() {
		o.Expect(oc.Run("adm", "node-logs").Args(randomNode(oc), "--since=-2m", "--until=-1m").Execute()).To(o.Succeed())
	})

	g.It("oc adm node-logs --since=<explicit-date> --until=-1m", func() {
		since := time.Now().Add(-2 * time.Minute).Format("2006-01-02 15:04:05")
		out, err := oc.Run("adm", "node-logs").Args(randomNode(oc), fmt.Sprintf("--since=%s", since)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("Failed to parse timestamp: "))
	})

	g.It("oc adm node-logs --unit=kubelet --since=-1m", func() {
		o.Expect(oc.Run("adm", "node-logs").Args(randomNode(oc), "--unit=kubelet", "--since=-2m").Execute()).To(o.Succeed())
	})

	g.It("oc adm node-logs --tail=5", func() {
		o.Expect(oc.Run("adm", "node-logs").Args(randomNode(oc), "--tail=5").Execute()).To(o.Succeed())
	})
})

func randomNode(oc *exutil.CLI) string {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return nodes.Items[rand.Intn(len(nodes.Items))].Name
}
