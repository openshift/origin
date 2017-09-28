package prometheus

import (
	"encoding/json"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/api/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

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
				terminalBuildCountQuery = "openshift_build_terminal_phase_total"
				activeBuildCountQuery   = "openshift_build_running_phase_start_time_seconds"
				failedBuildCountQuery   = "openshift_build_failed_phase_total"
			)

			appTemplate := exutil.FixturePath("..", "..", "examples", "jenkins", "application-template.json")

			execPodName = e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) {
				pod.Spec.Containers[0].Image = "centos:7"
			})
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By("verifying the oauth-proxy reports a 403 on the root URL")
			err := expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("https://%s:%d", host, statsPort), 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying a service account token is able to authenticate")
			err = expectBearerTokenURLStatusCodeExec(ns, execPodName, fmt.Sprintf("https://%s:%d/graph", host, statsPort), bearerToken, 200)
			o.Expect(err).NotTo(o.HaveOccurred())

			br := startOpenShiftBuild(oc, appTemplate)

			g.By("verifying a service account token is able to query active build metrics from the Prometheus API")
			// NOTE - activeBuildCountQuery is dependent on prometheus querying while the build is running;
			// timing has been a bit tricky when attempting to query after the build is complete based on the
			// default prometheus scrapping window, so we do the active query while the build is running
			activeTests := map[string][]metricTest{
				activeBuildCountQuery: {
					metricTest{
						labels:      map[string]string{"name": "frontend-1"},
						greaterThan: true,
					},
				},
			}
			runQueries(activeTests)

			g.By("verifying build completed successfully")
			err = exutil.WaitForBuildResult(oc.BuildClient().Build().Builds(oc.Namespace()), br)
			o.Expect(err).NotTo(o.HaveOccurred())
			br.AssertSuccess()

			g.By("verifying a service account token is able to query terminal build metrics from the Prometheus API")
			terminalTests := map[string][]metricTest{
				terminalBuildCountQuery: {
					metricTest{
						labels:      map[string]string{"phase": "complete"},
						greaterThan: true,
					},
					metricTest{
						labels: map[string]string{"phase": "cancelled"},
					},
				},
			}
			runQueries(terminalTests)

			g.By("verifying a service account token is able to query failed build metrics from the Prometheus API")
			failedTests := map[string][]metricTest{
				failedBuildCountQuery: {
					metricTest{
						labels: map[string]string{"reason": ""},
					},
				},
			}
			runQueries(failedTests)

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
	labels      map[string]string
	greaterThan bool
	value       float64
	success     bool
}

func runQueries(metricTests map[string][]metricTest) {
	// expect all correct metrics within 60 seconds
	errsMap := map[string]error{}
	for i := 0; i < 60; i++ {
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
					errsMap[query] = fmt.Errorf("query %s for tests %#v had results %s", query, tcs, contents)
					break
				}
			}
		}

		if len(errsMap) == 0 {
			break
		}

		time.Sleep(time.Second)
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
	//NOTE - we could use SampleValue has an Equals func, but since SampleValue has no GreaterThan,
	// we have to go down the float64 compare anyway
	if tc.greaterThan {
		return float64(sample.Value) > tc.value
	}
	return float64(sample.Value) == tc.value
}
