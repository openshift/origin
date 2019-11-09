package prometheus

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildv1 "github.com/openshift/api/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Prometheus][Feature:Builds] Prometheus", func() {
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
		g.It("should start and expose a secured proxy and verify build metrics", func() {
			oc.SetupProject()
			ns := oc.Namespace()
			appTemplate := exutil.FixturePath("testdata", "builds", "build-pruning", "successful-build-config.yaml")

			execPod := exutil.CreateCentosExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", nil)
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPod.Name, metav1.NewDeleteOptions(1)) }()

			g.By("verifying the oauth-proxy reports a 403 on the root URL")
			// allow for some retry, a la prometheus.go and its initial hitting of the metrics endpoint after
			// instantiating prometheus tempalte
			var err error
			for i := 0; i < maxPrometheusQueryRetries; i++ {
				err = expectURLStatusCodeExec(ns, execPod.Name, url, 403)
				if err == nil {
					break
				}
				time.Sleep(time.Second)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying a service account token is able to authenticate")
			err = expectBearerTokenURLStatusCodeExec(ns, execPod.Name, fmt.Sprintf("%s/graph", url), bearerToken, 200)
			o.Expect(err).NotTo(o.HaveOccurred())

			br := startOpenShiftBuild(oc, appTemplate)

			g.By("verifying build completed successfully")
			err = exutil.WaitForBuildResult(oc.BuildClient().BuildV1().Builds(oc.Namespace()), br)
			o.Expect(err).NotTo(o.HaveOccurred())
			br.AssertSuccess()

			g.By("verifying a service account token is able to query terminal build metrics from the Prometheus API")
			// note, no longer register a metric if it is zero, so a successful build won't have failed or cancelled metrics
			buildCountMetricName := fmt.Sprintf(`openshift_build_total{phase="%s"} >= 0`, string(buildv1.BuildPhaseComplete))
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

type prometheusResponse struct {
	Status string                 `json:"status"`
	Data   prometheusResponseData `json:"data"`
}

type prometheusResponseData struct {
	ResultType string       `json:"resultType"`
	Result     model.Vector `json:"result"`
}

func runQueries(promQueries map[string]bool, oc *exutil.CLI, ns, execPodName, baseURL, bearerToken string) {
	// expect all correct metrics within a reasonable time period
	queryErrors := make(map[string]error)
	passed := make(map[string]struct{})
	for i := 0; i < maxPrometheusQueryRetries; i++ {
		for query, expected := range promQueries {
			if _, ok := passed[query]; ok {
				continue
			}
			//TODO when the http/query apis discussed at https://github.com/prometheus/client_golang#client-for-the-prometheus-http-api
			// and introduced at https://github.com/prometheus/client_golang/blob/master/api/prometheus/v1/api.go are vendored into
			// openshift/origin, look to replace this homegrown http request / query param with that API
			g.By("perform prometheus metric query " + query)
			url := fmt.Sprintf("%s/api/v1/query?%s", baseURL, (url.Values{"query": []string{query}}).Encode())
			contents, err := getBearerTokenURLViaPod(ns, execPodName, url, bearerToken)
			o.Expect(err).NotTo(o.HaveOccurred())

			// check query result, if this is a new error log it, otherwise remain silent
			var result prometheusResponse
			if err := json.Unmarshal([]byte(contents), &result); err != nil {
				e2e.Logf("unable to parse query response for %s: %v", query, err)
				continue
			}
			metrics := result.Data.Result
			if result.Status != "success" {
				msg := fmt.Sprintf("promQL query: %s had reported incorrect status: %#v", query, metrics)
				if prev, ok := queryErrors[query]; !ok || prev.Error() != msg {
					e2e.Logf("%s", msg)
				}
				queryErrors[query] = fmt.Errorf(msg)
				continue
			}
			if (len(metrics) > 0 && !expected) || (len(metrics) == 0 && expected) {
				msg := fmt.Sprintf("promQL query: %s had reported incorrect results: %#v", query, metrics)
				if prev, ok := queryErrors[query]; !ok || prev.Error() != msg {
					e2e.Logf("%s", msg)
				}
				queryErrors[query] = fmt.Errorf(msg)
				continue
			}

			// query successful
			passed[query] = struct{}{}
			delete(queryErrors, query)
		}

		if len(queryErrors) == 0 {
			break
		}
		time.Sleep(time.Second)
	}

	if len(queryErrors) != 0 {
		exutil.DumpPodLogsStartingWith("prometheus-0", oc)
	}
	o.Expect(queryErrors).To(o.BeEmpty())
}

func startOpenShiftBuild(oc *exutil.CLI, appTemplate string) *exutil.BuildResult {
	g.By(fmt.Sprintf("calling oc create -f %s ", appTemplate))
	err := oc.Run("create").Args("-f", appTemplate).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	g.By("start build")
	br, err := exutil.StartBuildResult(oc, "myphp")
	o.Expect(err).NotTo(o.HaveOccurred())
	return br
}
