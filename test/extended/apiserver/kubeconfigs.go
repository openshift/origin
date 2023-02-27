package apiserver

import (
	"context"
	"fmt"
	"regexp"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][sig-api-machinery][Feature:APIServer] local kubeconfig", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("apiserver", admissionapi.LevelPrivileged)

	for _, kc := range []string{
		"localhost.kubeconfig",
		"lb-ext.kubeconfig",
		"lb-int.kubeconfig",
		"localhost-recovery.kubeconfig",
	} {
		kubeconfig := kc
		g.It(fmt.Sprintf("%q should be present on all masters and work", kubeconfig), func() {
			// external controlplane topology doesn't have master nodes
			controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			if *controlPlaneTopology == configv1.ExternalTopologyMode {
				g.Skip("ExternalControlPlaneTopology doesn't have master node kubeconfigs")
			}

			masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
				LabelSelector: `node-role.kubernetes.io/master`,
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Discovered %d master nodes.", len(masterNodes.Items))
			o.Expect(masterNodes.Items).NotTo(o.HaveLen(0))
			for _, master := range masterNodes.Items {
				err := retry.OnError(
					wait.Backoff{
						Duration: 2 * time.Second,
						Steps:    3,
						Factor:   5.0,
						Jitter:   0.1,
					},
					func(err error) bool {
						// retry error when kube-apiserver was temporarily unavailable, this matches oc error coming from:
						// https://github.com/kubernetes/kubernetes/blob/cbb5ea8210596ada1efce7e7a271ca4217ae598e/staging/src/k8s.io/kubectl/pkg/cmd/util/helpers.go#L237-L243
						matched, _ := regexp.MatchString("The connection to the server .+ was refused - did you specify the right host or port", err.Error())
						return !matched
					},
					func() error {
						return testNode(oc, kubeconfig, master.Name)
					})
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})
	}
})

func testNode(oc *exutil.CLI, kubeconfig, masterName string) error {
	g.By("Testing master node " + masterName)
	kubeconfigPath := "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/" + kubeconfig
	framework.Logf("Verifying kubeconfig %q on master %q", kubeconfig, masterName)
	out, err := oc.AsAdmin().Run("debug").Args("node/"+masterName, "--", "chroot", "/host", "/bin/bash", "-euxo", "pipefail", "-c",
		fmt.Sprintf(`oc --kubeconfig "%s" get namespace kube-system`, kubeconfigPath)).Output()
	framework.Logf(out)
	if err != nil {
		return fmt.Errorf(out)
	}
	return nil
}
