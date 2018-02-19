package prometheus

import (
	"encoding/json"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	execPodName, ns, host, bearerToken string
	statsPort                          int
)

var _ = g.Describe("[Feature:Prometheus][Feature:Builds] Prometheus", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("prometheus", exutil.KubeConfigPath())
	)

	g.BeforeEach(func() {
		ns, host, bearerToken, statsPort = bringUpPrometheusFromTemplate(oc)
	})

	g.Describe("when installed to the cluster", func() {
		g.It("should start and expose a secured proxy and verify build metrics", func() {
			const (
				buildCountQuery = "openshift_build_total"
			)

			appTemplate := exutil.FixturePath("..", "..", "examples", "jenkins", "application-template.json")

			execPodName = e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) {
				pod.Spec.Containers[0].Image = "centos:7"
			})
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By("verifying the oauth-proxy reports a 403 on the root URL")
			// allow for some retry, a la prometheus.go and its initial hitting of the metrics endpoint after
			// instantiating prometheus tempalte
			var err error
			for i := 0; i < waitForPrometheusStartSeconds; i++ {
				err = expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("https://%s:%d", host, statsPort), 403)
				if err == nil {
					break
				}
				time.Sleep(time.Second)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying a service account token is able to authenticate")
			err = expectBearerTokenURLStatusCodeExec(ns, execPodName, fmt.Sprintf("https://%s:%d/graph", host, statsPort), bearerToken, 200)
			o.Expect(err).NotTo(o.HaveOccurred())

			br := startOpenShiftBuild(oc, appTemplate)

			g.By("verifying build completed successfully")
			err = exutil.WaitForBuildResult(oc.BuildClient().Build().Builds(oc.Namespace()), br)
			o.Expect(err).NotTo(o.HaveOccurred())
			br.AssertSuccess()

			g.By("verifying a service account token is able to query terminal build metrics from the Prometheus API")
			// note, no longer register a metric if it is zero, so a successful build won't have failed or cancelled metrics
			terminalTests := map[string][]metricTest{
				buildCountQuery: {
					metricTest{
						labels:           map[string]string{"phase": string(buildapi.BuildPhaseComplete)},
						greaterThanEqual: true,
					},
				},
			}
			runQueries(terminalTests, oc)

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

type metricTest struct {
	labels map[string]string
	// we are not more precise (greater than only, or equal only) becauses the extended build tests
	// run in parallel on the CI system, and some of the metrics are cross namespace, so we cannot
	// reliably filter; we do precise count validation in the unit tests, where "entire cluster" activity
	// is more controlled :-)
	greaterThanEqual bool
	value            float64
	success          bool
}

func runQueries(metricTests map[string][]metricTest, oc *exutil.CLI) {
	// expect all correct metrics within a reasonable time period
	errsMap := map[string]error{}
	for i := 0; i < waitForPrometheusStartSeconds; i++ {
		for query, tcs := range metricTests {
			//TODO when the http/query apis discussed at https://github.com/prometheus/client_golang#client-for-the-prometheus-http-api
			// and introduced at https://github.com/prometheus/client_golang/blob/master/api/prometheus/v1/api.go are vendored into
			// openshift/origin, look to replace this homegrown http request / query param with that API
			g.By("perform prometheus metric query " + query)
			contents, err := getBearerTokenURLViaPod(ns, execPodName, fmt.Sprintf("https://%s:%d/api/v1/query?query=%s", host, statsPort, query), bearerToken)
			o.Expect(err).NotTo(o.HaveOccurred())
			result := prometheusResponse{}
			json.Unmarshal([]byte(contents), &result)
			metrics := result.Data.Result

			// for each test case, register that one of the returned metrics has the desired labels and value
			for j, tc := range tcs {
				for _, sample := range metrics {
					if labelsWeWant(sample, tc.labels) && valueWeWant(sample, tc) {
						tcs[j].success = true
						break
					}
				}
			}

			// now check the results, see if any bad
			delete(errsMap, query) // clear out any prior faliures
			for _, tc := range tcs {
				if !tc.success {
					dbg := fmt.Sprintf("query %s for tests %#v had results %s", query, tcs, contents)
					fmt.Fprintf(g.GinkgoWriter, dbg)
					errsMap[query] = fmt.Errorf(dbg)
					break
				}
			}
		}

		if len(errsMap) == 0 {
			break
		}

		time.Sleep(time.Second)
	}

	if len(errsMap) != 0 {
		exutil.DumpPodLogsStartingWith("prometheus-0", oc)
	}
	o.Expect(errsMap).To(o.BeEmpty())
}

func startOpenShiftBuild(oc *exutil.CLI, appTemplate string) *exutil.BuildResult {
	g.By("waiting for builder service account")
	err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("calling oc new-app  %s ", appTemplate))
	err = oc.Run("new-app").Args(appTemplate).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	g.By("wait on imagestreams used by build")
	err = exutil.WaitForOpenShiftNamespaceImageStreams(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	g.By("explicitly set up image stream tag, avoid timing window")
	err = oc.AsAdmin().Run("tag").Args("openshift/nodejs:latest", oc.Namespace()+"/nodejs-010-centos7:latest").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("start build")
	br, err := exutil.StartBuildResult(oc, "frontend")
	o.Expect(err).NotTo(o.HaveOccurred())
	return br
}

func labelsWeWant(sample *model.Sample, labels map[string]string) bool {
	//NOTE - prometheus LabelSet.Equals is of little use to us, since the "instance" label
	// is specific to the host things are running on, so we can't craft an accurate Metric
	// to compare against
	for labelName, labelValue := range labels {
		if v, ok := sample.Metric[model.LabelName(labelName)]; ok {
			if string(v) != labelValue {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

func valueWeWant(sample *model.Sample, tc metricTest) bool {
	//NOTE - we could use SampleValue has an Equals func, but since SampleValue has no GreaterThanEqual,
	// we have to go down the float64 compare anyway
	if tc.greaterThanEqual {
		return float64(sample.Value) >= tc.value
	}
	return float64(sample.Value) < tc.value
}
