package prometheus

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"

	buildv1 "github.com/openshift/api/build/v1"
	configv1 "github.com/openshift/api/config/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
)

var _ = g.Describe("[sig-instrumentation][sig-builds][Feature:Builds] Prometheus", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithPodSecurityLevel("prometheus", admissionapi.LevelBaseline)
	)

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodStatesInNamespace("openshift-monitoring", oc)
			exutil.DumpPodLogsStartingWithInNamespace("prometheus-k8s", "openshift-monitoring", oc)
		}
	})

	g.Describe("when installed on the cluster", func() {
		g.It("should start and expose a secured proxy and verify build metrics [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
			controlPlaneTopology, infra_err := exutil.GetControlPlaneTopology(oc)
			o.Expect(infra_err).NotTo(o.HaveOccurred())

			// https://issues.redhat.com/browse/CO-895
			if *controlPlaneTopology == configv1.ExternalTopologyMode {
				e2eskipper.Skipf("Prometheus in External clusters does not collect metrics from the OpenShift controller manager because it lives outside of the cluster. " +
					"The openshift_build_total metric expected by this test is reported by the OpenShift controller manager. " +
					"Remove this skip when https://issues.redhat.com/browse/CO-895 is implemented.")
			}

			appTemplate := exutil.FixturePath("testdata", "builds", "build-pruning", "successful-build-config.yaml")

			br := startOpenShiftBuild(oc, appTemplate)

			g.By("verifying build completed successfully")
			err := exutil.WaitForBuildResult(oc.BuildClient().BuildV1().Builds(oc.Namespace()), br)
			o.Expect(err).NotTo(o.HaveOccurred())
			br.AssertSuccess()

			g.By("verifying a service account token is able to query terminal build metrics from the Prometheus API")
			// note, no longer register a metric if it is zero, so a successful build won't have failed or cancelled metrics
			buildCountMetricName := fmt.Sprintf(`openshift_build_total{phase="%s"} >= 0`, string(buildv1.BuildPhaseComplete))
			terminalTests := map[string]bool{
				buildCountMetricName: true,
			}
			err = helper.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), terminalTests, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			// NOTE:  in manual testing on a laptop, starting several serial builds in succession was sufficient for catching
			// at least a few builds in new/pending state with the default prometheus query interval;  but that has not
			// proven to be the case with automated testing;
			// so for now, we have no tests with openshift_build_new_pending_phase_creation_time_seconds
		})
	})
})

func startOpenShiftBuild(oc *exutil.CLI, appTemplate string) *exutil.BuildResult {
	g.By(fmt.Sprintf("calling oc create -f %s ", appTemplate))
	err := oc.Run("create").Args("-f", appTemplate).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	g.By("start build")
	br, err := exutil.StartBuildResult(oc, "myphp")
	o.Expect(err).NotTo(o.HaveOccurred())
	return br
}
