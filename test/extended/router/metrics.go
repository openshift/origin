package images

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("router-metrics", exutil.KubeConfigPath())

		username, password, execPodName, ns, host string
		statsPort                                 int
		hasHealth, hasMetrics                     bool
	)

	g.BeforeEach(func() {
		dc, err := oc.AdminAppsClient().Apps().DeploymentConfigs("default").Get("router", metav1.GetOptions{})
		if kapierrs.IsNotFound(err) {
			g.Skip("no router installed on the cluster")
			return
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		env := dc.Spec.Template.Spec.Containers[0].Env
		username, password = findEnvVar(env, "STATS_USERNAME"), findEnvVar(env, "STATS_PASSWORD")
		statsPortString := findEnvVar(env, "STATS_PORT")
		hasMetrics = len(findEnvVar(env, "ROUTER_METRICS_TYPE")) > 0
		listenAddr := findEnvVar(env, "ROUTER_LISTEN_ADDR")

		statsPort = 1936
		if len(listenAddr) > 0 {
			hasHealth = true
			_, port, _ := net.SplitHostPort(listenAddr)
			statsPortString = port
		}
		if len(statsPortString) > 0 {
			if port, err := strconv.Atoi(statsPortString); err == nil {
				statsPort = port
			}
		}

		// wait for the router endpoints to show up
		err = wait.PollImmediate(2*time.Second, 120*time.Second, func() (bool, error) {
			epts, err := oc.AdminKubeClient().CoreV1().Endpoints("default").Get("router", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(epts.Subsets) == 0 || len(epts.Subsets[0].Addresses) == 0 {
				return false, nil
			}
			host = epts.Subsets[0].Addresses[0].IP
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		ns = oc.KubeFramework().Namespace.Name
	})

	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			exutil.DumpPodLogsStartingWithInNamespace("router", "default", oc.AsAdmin())
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should expose a health check on the metrics port", func() {
			if !hasHealth {
				g.Skip("router does not have ROUTER_LISTEN_ADDR set")
			}
			execPodName = exutil.CreateExecPodOrFail(oc.AdminKubeClient().Core(), ns, "execpod")
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By("listening on the health port")
			err := expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s:%d/healthz", host, statsPort), 200)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should expose prometheus metrics for a route", func() {
			if !hasMetrics {
				g.Skip("router does not have ROUTER_METRICS_TYPE set")
			}

			g.By("when a route exists")
			configPath := exutil.FixturePath("testdata", "router-metrics.yaml")
			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			execPodName = exutil.CreateExecPodOrFail(oc.AdminKubeClient().Core(), ns, "execpod")
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By("preventing access without a username and password")
			err = expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s:%d/metrics", host, statsPort), 401, 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking for the expected metrics")
			routeLabels := labels{"backend": "http", "namespace": ns, "route": "weightedroute"}
			serverLabels := labels{"namespace": ns, "route": "weightedroute"}
			var metrics map[string]*dto.MetricFamily
			times := 10
			p := expfmt.TextParser{}
			var results string
			defer func() { e2e.Logf("initial metrics:\n%s", results) }()
			err = wait.PollImmediate(2*time.Second, 240*time.Second, func() (bool, error) {
				results, err = getAuthenticatedURLViaPod(ns, execPodName, fmt.Sprintf("http://%s:%d/metrics", host, statsPort), username, password)
				o.Expect(err).NotTo(o.HaveOccurred())

				metrics, err = p.TextToMetricFamilies(bytes.NewBufferString(results))
				o.Expect(err).NotTo(o.HaveOccurred())
				//e2e.Logf("Metrics:\n%s", results)
				if len(findGaugesWithLabels(metrics["haproxy_server_up"], serverLabels)) == 2 {
					if findGaugesWithLabels(metrics["haproxy_backend_connections_total"], routeLabels)[0] >= float64(times) {
						return true, nil
					}
					// send a burst of traffic to the router
					g.By("sending traffic to a weighted route")
					err = expectRouteStatusCodeRepeatedExec(ns, execPodName, fmt.Sprintf("http://%s", host), "weighted.metrics.example.com", http.StatusOK, times)
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				g.By("retrying metrics until all backend servers appear")
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			allEndpoints := sets.NewString()
			services := []string{"weightedendpoints1", "weightedendpoints2"}
			for _, name := range services {
				epts, err := oc.AdminKubeClient().CoreV1().Endpoints(ns).Get(name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				for _, s := range epts.Subsets {
					for _, a := range s.Addresses {
						allEndpoints.Insert(a.IP + ":8080")
					}
				}
			}
			foundEndpoints := sets.NewString(findMetricLabels(metrics["haproxy_server_http_responses_total"], serverLabels, "server")...)
			o.Expect(allEndpoints.List()).To(o.Equal(foundEndpoints.List()))
			foundServices := sets.NewString(findMetricLabels(metrics["haproxy_server_http_responses_total"], serverLabels, "service")...)
			o.Expect(services).To(o.Equal(foundServices.List()))
			foundPods := sets.NewString(findMetricLabels(metrics["haproxy_server_http_responses_total"], serverLabels, "pod")...)
			o.Expect([]string{"endpoint-1", "endpoint-2"}).To(o.Equal(foundPods.List()))

			// route specific metrics from server and backend
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_http_responses_total"], serverLabels.With("code", "2xx"))).To(o.ConsistOf(o.BeNumerically(">", 0), o.BeNumerically(">", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_http_responses_total"], serverLabels.With("code", "5xx"))).To(o.Equal([]float64{0, 0}))
			// only server returns response counts
			o.Expect(findGaugesWithLabels(metrics["haproxy_backend_http_responses_total"], routeLabels.With("code", "2xx"))).To(o.HaveLen(0))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_connections_total"], serverLabels)).To(o.ConsistOf(o.BeNumerically(">=", 0), o.BeNumerically(">=", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_backend_connections_total"], routeLabels)).To(o.ConsistOf(o.BeNumerically(">=", times)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_up"], serverLabels)).To(o.Equal([]float64{1, 1}))
			o.Expect(findGaugesWithLabels(metrics["haproxy_backend_up"], routeLabels)).To(o.Equal([]float64{1}))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_bytes_in_total"], serverLabels)).To(o.ConsistOf(o.BeNumerically(">=", 0), o.BeNumerically(">=", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_bytes_out_total"], serverLabels)).To(o.ConsistOf(o.BeNumerically(">=", 0), o.BeNumerically(">=", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_max_sessions"], serverLabels)).To(o.ConsistOf(o.BeNumerically(">", 0), o.BeNumerically(">", 0)))

			// generic metrics
			o.Expect(findGaugesWithLabels(metrics["haproxy_up"], nil)).To(o.Equal([]float64{1}))
			o.Expect(findGaugesWithLabels(metrics["haproxy_exporter_scrape_interval"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))
			o.Expect(findCountersWithLabels(metrics["haproxy_exporter_total_scrapes"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))
			o.Expect(findCountersWithLabels(metrics["haproxy_exporter_csv_parse_failures"], nil)).To(o.Equal([]float64{0}))
			o.Expect(findGaugesWithLabels(metrics["haproxy_process_resident_memory_bytes"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_process_max_fds"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))
			o.Expect(findGaugesWithLabels(metrics["openshift_build_info"], nil)).To(o.Equal([]float64{1}))

			// router metrics
			o.Expect(findMetricsWithLabels(metrics["template_router_reload_seconds"], nil)[0].Summary.GetSampleSum()).To(o.BeNumerically(">", 0))
			o.Expect(findMetricsWithLabels(metrics["template_router_write_config_seconds"], nil)[0].Summary.GetSampleSum()).To(o.BeNumerically(">", 0))

			// verify that across a reload metrics are preserved
			g.By("forcing a router restart after a pod deletion")

			// delete the pod
			err = oc.AdminKubeClient().CoreV1().Pods(ns).Delete("endpoint-2", nil)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the router to reload")
			time.Sleep(15 * time.Second)

			g.By("checking that some metrics are not reset to 0 after router restart")
			updatedResults, err := getAuthenticatedURLViaPod(ns, execPodName, fmt.Sprintf("http://%s:%d/metrics", host, statsPort), username, password)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() { e2e.Logf("final metrics:\n%s", updatedResults) }()

			updatedMetrics, err := p.TextToMetricFamilies(bytes.NewBufferString(updatedResults))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(findGaugesWithLabels(updatedMetrics["haproxy_backend_connections_total"], routeLabels)[0]).To(o.BeNumerically(">=", findGaugesWithLabels(metrics["haproxy_backend_connections_total"], routeLabels)[0]))
			o.Expect(findGaugesWithLabels(updatedMetrics["haproxy_server_bytes_in_total"], serverLabels)[0]).To(o.BeNumerically(">=", findGaugesWithLabels(metrics["haproxy_server_bytes_in_total"], serverLabels)[0]))
		})

		g.It("should expose the profiling endpoints", func() {
			if !hasHealth {
				g.Skip("router does not have ROUTER_LISTEN_ADDR set")
			}
			execPodName = exutil.CreateExecPodOrFail(oc.AdminKubeClient().Core(), ns, "execpod")
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By("preventing access without a username and password")
			err := expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s:%d/debug/pprof/heap", host, statsPort), 401, 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("at /debug/pprof")
			results, err := getAuthenticatedURLViaPod(ns, execPodName, fmt.Sprintf("http://%s:%d/debug/pprof/heap?debug=1", host, statsPort), username, password)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(results).To(o.ContainSubstring("# runtime.MemStats"))
		})
	})
})

type labels map[string]string

func (l labels) With(name, value string) labels {
	n := make(labels)
	for k, v := range l {
		n[k] = v
	}
	n[name] = value
	return n
}

func findEnvVar(vars []kapi.EnvVar, key string) string {
	for _, v := range vars {
		if v.Name == key {
			return v.Value
		}
	}
	return ""
}

func findMetricsWithLabels(f *dto.MetricFamily, labels map[string]string) []*dto.Metric {
	var result []*dto.Metric
	if f == nil {
		return result
	}
	for _, m := range f.Metric {
		matched := map[string]struct{}{}
		for _, l := range m.Label {
			if expect, ok := labels[l.GetName()]; ok {
				if expect != l.GetValue() {
					break
				}
				matched[l.GetName()] = struct{}{}
			}
		}
		if len(matched) != len(labels) {
			continue
		}
		result = append(result, m)
	}
	return result
}

func findCountersWithLabels(f *dto.MetricFamily, labels map[string]string) []float64 {
	var result []float64
	for _, m := range findMetricsWithLabels(f, labels) {
		result = append(result, m.Counter.GetValue())
	}
	return result
}

func findGaugesWithLabels(f *dto.MetricFamily, labels map[string]string) []float64 {
	var result []float64
	for _, m := range findMetricsWithLabels(f, labels) {
		result = append(result, m.Gauge.GetValue())
	}
	return result
}

func findMetricLabels(f *dto.MetricFamily, labels map[string]string, match string) []string {
	var result []string
	for _, m := range findMetricsWithLabels(f, labels) {
		for _, l := range m.Label {
			if l.GetName() == match {
				result = append(result, l.GetValue())
				break
			}
		}
	}
	return result
}

func expectURLStatusCodeExec(ns, execPodName, url string, statusCodes ...int) error {
	cmd := fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' %q", url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	for _, statusCode := range statusCodes {
		if output == strconv.Itoa(statusCode) {
			return nil
		}
	}
	return fmt.Errorf("last response from server was not any of %v: %s", statusCodes, output)
}

func getAuthenticatedURLViaPod(ns, execPodName, url, user, pass string) (string, error) {
	cmd := fmt.Sprintf("curl -s -u %s:%s %q", user, pass, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}
