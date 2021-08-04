package apiserver

import (
	"context"
	"fmt"
	"regexp"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
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
				retry, err := testNode(oc, kubeconfig, master.Name)
				for retries := 2; retries > 0; retries-- {
					if !retry {
						break
					}
					g.By("There was a retryable error for " + fmt.Sprintf("%s/%s", master.Name, kubeconfig))
					retry, err = testNode(oc, kubeconfig, master.Name)
				}
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})
	}
})

func testNode(oc *exutil.CLI, kubeconfig, masterName string) (bool, error) {
	g.By("Testing master node " + masterName)
	kubeconfigPath := "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/" + kubeconfig
	framework.Logf("Verifying kubeconfig %q on master %q", kubeconfig, masterName)
	out, err := oc.AsAdmin().Run("debug").Args("node/"+masterName, "--", "chroot", "/host", "/bin/bash", "-euxo", "pipefail", "-c",
		fmt.Sprintf(`oc --kubeconfig "%s" get namespace kube-system`, kubeconfigPath)).Output()
	framework.Logf(out)
	// retry error when kube-apiserver was temporarily unavailable, this matches oc error coming from:
	// https://github.com/kubernetes/kubernetes/blob/cbb5ea8210596ada1efce7e7a271ca4217ae598e/staging/src/k8s.io/kubectl/pkg/cmd/util/helpers.go#L237-L243
	matched, _ := regexp.MatchString("The connection to the server .+ was refused - did you specify the right host or port", out)
	return !matched, err
}
