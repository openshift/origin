package apiserver

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Conformance][sig-api-machinery][Feature:APIServer] local kubeconfig", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("apiserver")

	for _, kubeconfig := range []string{
		"localhost.kubeconfig",
		"lb-ext.kubeconfig",
		"lb-int.kubeconfig",
		"localhost-recovery.kubeconfig",
	} {
		g.It(fmt.Sprintf("%q should be present on all masters and work", kubeconfig), func() {
			masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
				LabelSelector: `node-role.kubernetes.io/master`,
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Discovered %d master nodes.", len(masterNodes.Items))
			o.Expect(masterNodes.Items).NotTo(o.HaveLen(0))
			for _, master := range masterNodes.Items {
				g.By("Testing master node " + master.Name)
				kubeconfigPath := "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/" + kubeconfig
				framework.Logf("Verifying kubeconfig %q on master %s", master.Name)
				out, err := oc.AsAdmin().Run("debug").Args("node/"+master.Name, "--", "chroot", "/host", "/bin/bash", "-euxo", "pipefail", "-c", fmt.Sprintf(`oc --kubeconfig "%s" get namespace kube-system`, kubeconfigPath)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				framework.Logf(out)
			}
		})
	}
})
