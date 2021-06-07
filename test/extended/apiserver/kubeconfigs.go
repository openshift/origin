package apiserver

import (
	"context"
	"fmt"
	"strings"

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
		// it looks like g.It runs the test in parallel, capture the kubeconfig so that we test them all
		func(kubeconfig string) {
			g.It(fmt.Sprintf("%q should be present on all masters and work", kubeconfig), func() {
				masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
					LabelSelector: `node-role.kubernetes.io/master`,
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				framework.Logf("Discovered %d master nodes.", len(masterNodes.Items))
				o.Expect(masterNodes.Items).NotTo(o.HaveLen(0))

				apiServerPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-kube-apiserver").List(context.Background(), metav1.ListOptions{
					LabelSelector: "apiserver=true",
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				framework.Logf("Discovered %d Kube API server pods.", len(apiServerPods.Items))
				o.Expect(apiServerPods.Items).NotTo(o.HaveLen(0))
				currentRevision := apiServerPods.Items[0].Labels["revision"]

				kubeconfigPaths := []string{
					// old path
					"/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs",

					// new path
					fmt.Sprintf("/etc/kubernetes/static-pod-resources/kube-apiserver-pod-%s/dynamic/secrets/node-kubeconfigs", currentRevision),
				}

				for _, master := range masterNodes.Items {
					g.By("Testing master node " + master.Name)
					foundGoodKubeConfig := false
					for _, kubeconfigPath := range kubeconfigPaths {
						kubeconfigPath = fmt.Sprintf("%s/%s", kubeconfigPath, kubeconfig)
						framework.Logf("Verifying kubeconfig %q on master %s", kubeconfigPath, master.Name)
						out, err := oc.AsAdmin().Run("debug").Args("node/"+master.Name, "--", "chroot", "/host", "/bin/bash", "-euxo", "pipefail", "-c", fmt.Sprintf(`oc --kubeconfig "%s" get namespace kube-system -ojson`, kubeconfigPath)).Output()

						// it looks like relying on the exit code from the debug command is not enough
						// make sure the output contains what we have asked for
						if err == nil && strings.Contains(out, `"name": "kube-system"`) {
							foundGoodKubeConfig = true
							break
						}
						// log the output for troubleshooting
						framework.Logf(out)
					}
					o.Expect(foundGoodKubeConfig).To(o.BeTrue())
				}
			})
		}(kubeconfig)
	}
})
