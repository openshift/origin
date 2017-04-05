package images

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][networking][router] openshift router metrics", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("router-metrics", exutil.KubeConfigPath())

		username, password, execPodName, ns, host string
		hasHealth, hasMetrics                     bool
	)

	g.BeforeEach(func() {
		dc, err := oc.AdminClient().DeploymentConfigs("default").Get("router")
		if kapierrs.IsNotFound(err) {
			g.Skip("no router installed on the cluster")
			return
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		env := dc.Spec.Template.Spec.Containers[0].Env
		username, password = findEnvVar(env, "STATS_USERNAME"), findEnvVar(env, "STATS_PASSWORD")

		hasMetrics = len(findEnvVar(env, "ROUTER_METRICS_TYPE")) > 0
		hasHealth = len(findEnvVar(env, "ROUTER_LISTEN_ADDR")) > 0

		epts, err := oc.AdminKubeClient().Endpoints("default").Get("router")
		o.Expect(err).NotTo(o.HaveOccurred())
		host = epts.Subsets[0].Addresses[0].IP

		ns = oc.KubeFramework().Namespace.Name
	})

	g.Describe("The HAProxy router", func() {
		g.It("should expose a health check on the metrics port", func() {
			if !hasHealth {
				g.Skip("router does not have ROUTER_LISTEN_ADDR set")
			}
			execPodName = exutil.CreateExecPodOrFail(oc.AdminKubeClient().Core(), ns, "execpod")
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, kapi.NewDeleteOptions(1)) }()

			g.By("listening on the health port")
			err := expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s:1935/healthz", host), 200)
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
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, kapi.NewDeleteOptions(1)) }()

			g.By("preventing access without a username and password")
			err = expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s:1935/metrics", host), 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking for the expected metrics")
			routeLabels := labels{"backend": "http", "namespace": ns, "route": "weightedroute"}
			serverLabels := labels{"namespace": ns, "route": "weightedroute"}
			var metrics map[string]*dto.MetricFamily
			times := 10
			var results string
			defer func() { e2e.Logf("received metrics:\n%s", results) }()
			for {
				results, err = getAuthenticatedURLViaPod(ns, execPodName, fmt.Sprintf("http://%s:1935/metrics", host), username, password)
				o.Expect(err).NotTo(o.HaveOccurred())

				p := expfmt.TextParser{}
				metrics, err = p.TextToMetricFamilies(bytes.NewBufferString(results))
				o.Expect(err).NotTo(o.HaveOccurred())
				//e2e.Logf("Metrics:\n%s", results)
				if len(findGaugesWithLabels(metrics["haproxy_server_up"], serverLabels)) == 2 {
					if findGaugesWithLabels(metrics["haproxy_server_connections_total"], serverLabels)[0] > 0 {
						break
					}
					// send a burst of traffic to the router
					g.By("sending traffic to a weighted route")
					err = expectRouteStatusCodeRepeatedExec(ns, execPodName, fmt.Sprintf("http://%s", host), "weighted.example.com", http.StatusOK, times)
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				time.Sleep(2 * time.Second)
				g.By("retrying metrics until all backend servers appear")
			}

			allEndpoints := sets.NewString()
			services := []string{"weightedendpoints1", "weightedendpoints2"}
			for _, name := range services {
				epts, err := oc.AdminKubeClient().Endpoints(ns).Get(name)
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
		})

		g.It("should expose the profiling endpoints", func() {
			if !hasHealth {
				g.Skip("router does not have ROUTER_LISTEN_ADDR set")
			}
			execPodName = exutil.CreateExecPodOrFail(oc.AdminKubeClient().Core(), ns, "execpod")
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, kapi.NewDeleteOptions(1)) }()

			g.By("preventing access without a username and password")
			err := expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s:1935/debug/pprof/heap", host), 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("at /debug/pprof")
			results, err := getAuthenticatedURLViaPod(ns, execPodName, fmt.Sprintf("http://%s:1935/debug/pprof/heap", host), username, password)
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

func expectURLStatusCodeExec(ns, execPodName, url string, statusCode int) error {
	cmd := fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' %q", url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	if output != strconv.Itoa(statusCode) {
		return fmt.Errorf("last response from server was not %d: %s", statusCode, output)
	}
	return nil
}

func getAuthenticatedURLViaPod(ns, execPodName, url, user, pass string) (string, error) {
	cmd := fmt.Sprintf("curl -s -u %s:%s %q", user, pass, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}
