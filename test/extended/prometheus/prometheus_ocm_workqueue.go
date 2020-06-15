package prometheus

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/ibmcloud"
)

var _ = g.Describe("[Feature:Prometheus][Conformance] Prometheus", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("prometheus")

		url, bearerToken string
	)
	g.BeforeEach(func() {
		var ok bool
		url, bearerToken, ok = locatePrometheus(oc)
		if !ok {
			e2e.Skipf("Prometheus could not be located on this cluster, skipping prometheus test")
		}
	})

	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			exutil.DumpPodStatesInNamespace("openshift-monitoring", oc)
			exutil.DumpPodLogsStartingWithInNamespace("prometheus-k8s", "openshift-monitoring", oc)
		}
	})

	g.Describe("when installed on the cluster", func() {
		g.It("should start and expose a secured proxy and verify OCM workqueue metrics", func() {

			// https://issues.redhat.com/browse/CO-895
			if e2e.TestContext.Provider == ibmcloud.ProviderName {
				e2e.Skipf("Prometheus in IBM ROKS clusters does not collect metrics from the OpenShift controller manager because it lives outside of the cluster. " +
					"The metrics expected by this test are reported by the OpenShift controller manager. " +
					"Remove this skip when https://issues.redhat.com/browse/CO-895 is implemented.")
			}

			oc.SetupProject()
			ns := oc.Namespace()

			execPod := exutil.CreateCentosExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", nil)
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPod.Name, metav1.NewDeleteOptions(1)) }()

			g.By("verifying the oauth-proxy reports a 403 on the root URL")
			// allow for some retry, a la prometheus.go and its initial hitting of the metrics endpoint after
			// instantiating prometheus tempalte
			var err error
			for i := 0; i < maxPrometheusQueryAttempts; i++ {
				err = expectURLStatusCodeExec(ns, execPod.Name, url, 403)
				if err == nil {
					break
				}
				time.Sleep(prometheusQueryRetrySleep)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying a service account token is able to authenticate")
			err = expectBearerTokenURLStatusCodeExec(ns, execPod.Name, fmt.Sprintf("%s/graph", url), bearerToken, 200)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying a service account token is able to query terminal OCM workqueue metrics from the Prometheus API")
			// note, no longer register a metric if it is zero, so a successful build won't have failed or cancelled metrics
			buildCountMetricName := fmt.Sprintf(`workqueue_adds_total{name="image-signature-import",namespace="openshift-controller-manager"}  >= 0`)
			terminalTests := map[string]bool{
				buildCountMetricName: true,
			}
			runQueries(terminalTests, oc, ns, execPod.Name, url, bearerToken)

			// NOTE:  in manual testing on a laptop, starting several serial builds in succession was sufficient for catching
			// at least a few builds in new/pending state with the default prometheus query interval;  but that has not
			// proven to be the case with automated testing;
			// so for now, we have no tests with openshift_build_new_pending_phase_creation_time_seconds
		})
	})
})
