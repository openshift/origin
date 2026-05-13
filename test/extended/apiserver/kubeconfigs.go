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

var kcLocations = map[string]string{
	"localhost.kubeconfig":          "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig",
	"lb-ext.kubeconfig":             "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-ext.kubeconfig",
	"lb-int.kubeconfig":             "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-int.kubeconfig",
	"localhost-recovery.kubeconfig": "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost-recovery.kubeconfig",
}
var kubeApiserverLocations = map[string]string{
	"check-endpoints.kubeconfig":    "/etc/kubernetes/static-pod-certs/configmaps/check-endpoints-kubeconfig/kubeconfig",
	"control-plane-node.kubeconfig": "/etc/kubernetes/static-pod-certs/configmaps/control-plane-node-kubeconfig/kubeconfig",
}

var _ = g.Describe("[Conformance][sig-api-machinery][Feature:APIServer] local kubeconfig", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("apiserver", admissionapi.LevelPrivileged)

	for kubeconfig := range kcLocations {
		g.It(fmt.Sprintf("%q should be present on all masters and work", kubeconfig), func() {
			testKubeConfig(oc, kubeconfig, testNode)
		})
	}

	for kubeconfig := range kubeApiserverLocations {
		g.It(fmt.Sprintf("%q should be present in all kube-apiserver containers", kubeconfig), func() {
			// skip on microshift
			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isMicroShift {
				g.Skip("Not supported on Microshift")
			}
			testKubeConfig(oc, kubeconfig, testKubeApiserverContainer)
		})
	}
})

func testKubeConfig(oc *exutil.CLI, kubeconfig string, testFn func(oc *exutil.CLI, kubeconfig, masterName string) error) {
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
				return testFn(oc, kubeconfig, master.Name)
			})
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func testNode(oc *exutil.CLI, kubeconfig, masterName string) error {
	g.By("Testing master node " + masterName)
	kubeconfigPath, ok := kcLocations[kubeconfig]
	if !ok {
		return fmt.Errorf("location for %s kubeconfig not found", kubeconfig)
	}
	framework.Logf("Verifying kubeconfig %q on master %q", kubeconfig, masterName)
	out, err := oc.AsAdmin().Run("debug").Args("node/"+masterName, "--", "chroot", "/host", "/bin/bash", "-euxo", "pipefail", "-c",
		fmt.Sprintf(`oc --kubeconfig "%s" get namespace kube-system`, kubeconfigPath)).Output()
	framework.Logf("%s", out)
	if err != nil {
		return fmt.Errorf("%s", out)
	}
	return nil
}

func testKubeApiserverContainer(oc *exutil.CLI, kubeconfig, masterName string) error {
	g.By("Testing kube-apiserver container on master node " + masterName)
	kubeconfigPath, ok := kubeApiserverLocations[kubeconfig]
	if !ok {
		return fmt.Errorf("location for %s kubeconfig not found", kubeconfig)
	}

	framework.Logf("Verifying kubeconfig %q in kube-apiserver container in master %q", kubeconfig, masterName)
	// Use curl to verify the kubeconfig is present and functional, extracting
	// cert paths directly from the kubeconfig file. This avoids copying the oc
	// binary from the host into the container, which fails when the host OS
	// (e.g., RHCOS 10 with glibc 2.38) has a newer glibc than the container.
	out, err := oc.AsAdmin().Run("exec").Args("-n", "openshift-kube-apiserver", "kube-apiserver-"+masterName, "--", "/bin/bash", "-euxo", "pipefail", "-c",
		fmt.Sprintf(`for k in server client-certificate client-key certificate-authority; do
  count=$(grep -Ec "^[[:space:]]*${k}:[[:space:]]+" "%[1]s")
  [ "$count" -eq 1 ] || { echo "expected exactly one ${k} in %[1]s, got $count" >&2; exit 1; }
done
server=$(grep '^[[:space:]]*server:' "%[1]s" | head -1 | awk '{print $2}')
cert=$(grep '^[[:space:]]*client-certificate:' "%[1]s" | head -1 | awk '{print $2}')
key=$(grep '^[[:space:]]*client-key:' "%[1]s" | head -1 | awk '{print $2}')
ca=$(grep '^[[:space:]]*certificate-authority:' "%[1]s" | head -1 | awk '{print $2}')
for v in server cert key ca; do
  eval "val=\$$v"
  [ -n "$val" ] || { echo "${v} field extraction failed from %[1]s" >&2; exit 1; }
done
curl -Ssf --connect-timeout 10 --max-time 45 --cert "$cert" --key "$key" --cacert "$ca" "${server}/api?timeout=32s"`, kubeconfigPath)).Output()
	framework.Logf("%s", out)
	if err != nil {
		return fmt.Errorf("%s", out)
	}
	return nil
}
