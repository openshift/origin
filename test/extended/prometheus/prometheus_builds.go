package prometheus

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/api/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Prometheus][Feature:Builds] Prometheus", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("prometheus", exutil.KubeConfigPath())

		execPodName, ns, host, bearerToken string
		statsPort                          int
	)

	g.BeforeEach(func() {
		ns, host, bearerToken, statsPort = bringUpPrometheusFromTemplate(oc)
	})

	g.Describe("when installed to the cluster", func() {
		g.It("should start and expose a secured proxy and verify build metrics after a completed build", func() {
			const (
				terminalBuildCountQuery = "openshift_build_terminal_phase_total"
				activeBuildCountQuery   = "openshift_build_running_phase_start_time_seconds"
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

			executeOpenShiftBuild(oc, appTemplate)

			g.By("verifying a service account token is able to query build metrics from the Prometheus API")
			metricTests := map[string][]metricTest{
				// NOTE - activeBuildCountQuery is dependent on prometheus querying while the build is running;
				// so far the prometheus query interval and the length of the frontend build have
				// been sufficient for reliable success here, but bear in mind the timing windows
				// if this particular metricTest starts flaking
				activeBuildCountQuery: {
					metricTest{
						labels:      map[string]string{"phase": "running"},
						greaterThan: "0",
					},
				},
				terminalBuildCountQuery: {
					metricTest{
						labels:      map[string]string{"phase": "complete"},
						greaterThan: "0",
					},
					metricTest{
						labels: map[string]string{"phase": "error"},
						equals: "0",
					},
					metricTest{
						labels: map[string]string{"phase": "failed"},
						equals: "0",
					},
					metricTest{
						labels: map[string]string{"phase": "cancelled"},
						equals: "0",
					},
				},
			}
			// expect all correct metrics within 60 seconds
			lastErrsMap := map[string]error{}
			for i := 0; i < 60; i++ {
				for query, tcs := range metricTests {
					g.By("perform prometheus metric query " + query)
					contents, err := getBearerTokenURLViaPod(ns, execPodName, fmt.Sprintf("https://%s:%d/api/v1/query?query=%s", host, statsPort, query), bearerToken)
					o.Expect(err).NotTo(o.HaveOccurred())

					correctMetrics := map[int]bool{}
					for i, tc := range tcs {
						result := prometheusResponse{}
						json.Unmarshal([]byte(contents), &result)
						metrics := result.Data.Result

						for _, sample := range metrics {
							// first see if a metric has all the label names and label values we are looking for
							foundCorrectLabels := true
							for labelName, labelValue := range tc.labels {
								if v, ok := sample.Metric[model.LabelName(labelName)]; ok {
									if string(v) != labelValue {
										foundCorrectLabels = false
										break
									}
								} else {
									foundCorrectLabels = false
									break
								}
							}

							// if found metric with correct set of labels, now see if the metric value is what we are expecting
							if foundCorrectLabels {
								switch {
								case len(tc.equals) > 0:
									if x, err := strconv.ParseFloat(tc.equals, 64); err == nil && float64(sample.Value) == x {
										correctMetrics[i] = true
										break
									}
								case len(tc.greaterThan) > 0:
									if x, err := strconv.ParseFloat(tc.greaterThan, 64); err == nil && float64(sample.Value) > x {
										correctMetrics[i] = true
										break
									}
								}
							}

						}
					}

					if len(correctMetrics) == len(tcs) {
						delete(metricTests, query) // delete in case there are retries on remaining tests
						delete(lastErrsMap, query)
					} else {
						// maintain separate map of errors for diagnostics
						lastErrsMap[query] = fmt.Errorf("query %s with results %s only had correct metrics %v", query, contents, correctMetrics)
					}
				}

				if len(metricTests) == 0 {
					break
				}

				time.Sleep(time.Second)
			}

			o.Expect(lastErrsMap).To(o.BeEmpty())
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
	equals      string
	greaterThan string
}

func executeOpenShiftBuild(oc *exutil.CLI, appTemplate string) {
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

	g.By("start build, wait for completion")
	br, err := exutil.StartBuildAndWait(oc, "frontend")
	o.Expect(err).NotTo(o.HaveOccurred())
	br.AssertSuccess()
}
