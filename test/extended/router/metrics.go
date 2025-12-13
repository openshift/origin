package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-network][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		oc                              = exutil.NewCLIWithPodSecurityLevel("router-metrics", admissionapi.LevelBaseline)
		username, password, bearerToken string
		metricsPort                     int32
		execPodName, ns, host           string

		proxyProtocol bool
	)

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("MicroShift does not have Prometheus")
		}

		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		platformType := infra.Status.Platform
		if infra.Status.PlatformStatus != nil {
			platformType = infra.Status.PlatformStatus.Type
		}
		proxyProtocol = platformType == configv1.AWSPlatformType

		// This test needs to make assertions against a single router pod, so all access
		// to the router should happen through a single endpoint.

		// Discover the endpoint.
		endpoint, err := oc.AdminKubeClient().CoreV1().Endpoints("openshift-ingress").Get(context.Background(), "router-internal-default", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(endpoint.Subsets).NotTo(o.BeEmpty())
		subset := endpoint.Subsets[0]
		o.Expect(subset.Addresses).NotTo(o.BeEmpty())

		// Extract the metrics port by name.
		for _, port := range subset.Ports {
			if port.Name == "metrics" {
				metricsPort = port.Port
				break
			}
		}
		o.Expect(metricsPort).NotTo(o.BeZero())

		// Extract the IP of a single router pod.
		host = subset.Addresses[0].IP

		// Extract the router pod's stats credentials.
		statsSecret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-ingress").Get(context.Background(), "router-stats-default", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		username, password = string(statsSecret.Data["statsUsername"]), string(statsSecret.Data["statsPassword"])

		token, err := oc.AdminKubeClient().
			CoreV1().
			ServiceAccounts("openshift-monitoring").
			CreateToken(
				context.TODO(),
				"prometheus-k8s",
				&authenticationv1.TokenRequest{
					Spec: authenticationv1.TokenRequestSpec{},
				},
				metav1.CreateOptions{},
			)
		o.Expect(err).NotTo(o.HaveOccurred())
		bearerToken = token.Status.Token

		ns = oc.KubeFramework().Namespace.Name
	})

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWithInNamespace("router", "openshift-ingress", oc.AsAdmin())
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should expose a health check on the metrics port", g.Label("Size:S"), func() {
			execPodName = exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod").Name
			defer func() {
				oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPodName, *metav1.NewDeleteOptions(1))
			}()

			g.By("listening on the health port")
			err := expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s/healthz", net.JoinHostPort(host, strconv.Itoa(int(metricsPort)))), 200)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should expose prometheus metrics for a route [apigroup:route.openshift.io]", g.Label("Size:M"), func() {
			g.By("when a route exists")
			configPath := exutil.FixturePath("testdata", "router", "router-metrics.yaml")
			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the route to be admitted")
			routeHost, err := waitForAdmittedRoute(2*time.Minute, oc.AdminRouteClient().RouteV1(), ns, "weightedroute", "default", true)
			o.Expect(err).NotTo(o.HaveOccurred(), "route was not admitted")

			execPodName = exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod").Name
			defer func() {
				oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPodName, *metav1.NewDeleteOptions(1))
			}()

			g.By("preventing access without a username and password")
			err = expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s/metrics", net.JoinHostPort(host, strconv.Itoa(int(metricsPort)))), 401, 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("validate access using username and password")
			_, err = getAuthenticatedURLViaPod(ns, execPodName, fmt.Sprintf("http://%s/metrics", net.JoinHostPort(host, strconv.Itoa(int(metricsPort)))), username, password)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking for the expected metrics")
			routeLabels := promLabels{"backend": "http", "namespace": ns, "route": "weightedroute"}
			serverLabels := promLabels{"namespace": ns, "route": "weightedroute"}
			var metrics map[string]*dto.MetricFamily
			var results string
			defer func() { e2e.Logf("initial metrics:\n%s", results) }()
			times := 10
			p := expfmt.TextParser{}

			err = wait.PollImmediate(2*time.Second, 240*time.Second, func() (bool, error) {
				results, err = prometheus.GetBearerTokenURLViaPod(oc, execPodName, fmt.Sprintf("http://%s/metrics", net.JoinHostPort(host, strconv.Itoa(int(metricsPort)))), bearerToken)
				o.Expect(err).NotTo(o.HaveOccurred())
				metrics, err = p.TextToMetricFamilies(bytes.NewBufferString(results))
				o.Expect(err).NotTo(o.HaveOccurred())

				if len(findNonZeroGaugesWithLabels(metrics["haproxy_server_up"], serverLabels)) == 2 {
					if g := findGaugesWithLabels(metrics["haproxy_backend_connections_total"], routeLabels); len(g) > 0 {
						// stop retrying if the route got expected number of connections.
						if g[0] >= float64(times) {
							return true, nil
						}
					}
					// send a burst of traffic to the router
					g.By("sending traffic to a weighted route")
					err = expectRouteStatusCodeRepeatedExec(ns, execPodName, fmt.Sprintf("http://%s", exutil.IPUrl(host)), routeHost, http.StatusOK, times, proxyProtocol)
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				g.By("retrying metrics until all backend servers appear")
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			allEndpoints := sets.NewString()
			services := []string{"weightedendpoints1", "weightedendpoints2"}
			for _, name := range services {
				epts, err := oc.AdminKubeClient().CoreV1().Endpoints(ns).Get(context.Background(), name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				for _, s := range epts.Subsets {
					for _, a := range s.Addresses {
						allEndpoints.Insert(a.IP + ":8080")
					}
				}
			}
			foundEndpoints := sets.NewString(findMetricLabels(metrics["haproxy_server_http_responses_total"], serverLabels, "server")...)
			// There can be more server slots than endpoints if DCM is enabled (dynamic ones).
			o.Expect(foundEndpoints.List()).To(o.ContainElements(allEndpoints.List()))
			foundServices := sets.NewString(findMetricLabels(metrics["haproxy_server_http_responses_total"], serverLabels, "service")...)
			// Dynamic servers (_dynamic_pods-N) have empty value for service label.
			// "foundServices" may be a bigger set than expected endpoints if DCM is enabled.
			o.Expect(foundServices.List()).To(o.ContainElements(services))
			foundPods := sets.NewString(findMetricLabels(metrics["haproxy_server_http_responses_total"], serverLabels, "pod")...)
			// Dynamic servers (_dynamic_pods-N) have empty value for pod label.
			// "foundPods" may be a bigger set than expected endpoints is DCM is enabled.
			o.Expect(foundPods.List()).To(o.ContainElements([]string{"endpoint-1", "endpoint-2"}))

			// route specific metrics from server and backend
			o.Expect(findNonZeroGaugesWithLabels(metrics["haproxy_server_http_responses_total"], serverLabels.With("code", "2xx"))).To(o.ConsistOf(o.BeNumerically(">", 0), o.BeNumerically(">", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_http_responses_total"], serverLabels.With("code", "5xx"))).To(o.HaveEach(float64(0)))
			// backends started returning response counts in https://github.com/openshift/router/pull/132
			o.Expect(findGaugesWithLabels(metrics["haproxy_backend_http_responses_total"], routeLabels.With("code", "2xx"))).ToNot(o.BeZero())
			o.Expect(findNonZeroGaugesWithLabels(metrics["haproxy_server_connections_total"], serverLabels)).To(o.ConsistOf(o.BeNumerically(">=", 0), o.BeNumerically(">=", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_backend_connections_total"], routeLabels)).To(o.ConsistOf(o.BeNumerically(">=", times)))
			o.Expect(findNonZeroGaugesWithLabels(metrics["haproxy_server_up"], serverLabels)).To(o.Equal([]float64{1, 1}))
			o.Expect(findGaugesWithLabels(metrics["haproxy_backend_up"], routeLabels)).To(o.Equal([]float64{1}))
			o.Expect(findNonZeroGaugesWithLabels(metrics["haproxy_server_bytes_in_total"], serverLabels)).To(o.ConsistOf(o.BeNumerically(">=", 0), o.BeNumerically(">=", 0)))
			o.Expect(findNonZeroGaugesWithLabels(metrics["haproxy_server_bytes_out_total"], serverLabels)).To(o.ConsistOf(o.BeNumerically(">=", 0), o.BeNumerically(">=", 0)))

			// generic metrics
			o.Expect(findGaugesWithLabels(metrics["haproxy_up"], nil)).To(o.Equal([]float64{1}))
			o.Expect(findGaugesWithLabels(metrics["haproxy_exporter_scrape_interval"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))
			o.Expect(findCountersWithLabels(metrics["haproxy_exporter_total_scrapes"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))
			o.Expect(findCountersWithLabels(metrics["haproxy_exporter_csv_parse_failures"], nil)).To(o.Equal([]float64{0}))
			o.Expect(findGaugesWithLabels(metrics["haproxy_process_resident_memory_bytes"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_process_max_fds"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))

			// router metrics
			o.Expect(findMetricsWithLabels(metrics["template_router_reload_seconds"], nil)[0].Summary.GetSampleSum()).To(o.BeNumerically(">", 0))
			o.Expect(findMetricsWithLabels(metrics["template_router_write_config_seconds"], nil)[0].Summary.GetSampleSum()).To(o.BeNumerically(">", 0))

			// verify that across a reload metrics are preserved
			g.By("forcing a router restart after a pod deletion")

			// delete the pod
			err = oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), "endpoint-2", metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the router to reload")
			time.Sleep(15 * time.Second)

			g.By("checking that some metrics are not reset to 0 after router restart")
			updatedResults, err := prometheus.GetBearerTokenURLViaPod(oc, execPodName, fmt.Sprintf("http://%s/metrics", net.JoinHostPort(host, strconv.Itoa(int(metricsPort)))), bearerToken)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() { e2e.Logf("final metrics:\n%s", updatedResults) }()

			updatedMetrics, err := p.TextToMetricFamilies(bytes.NewBufferString(updatedResults))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(findGaugesWithLabels(updatedMetrics["haproxy_backend_connections_total"], routeLabels)[0]).To(o.BeNumerically(">=", findGaugesWithLabels(metrics["haproxy_backend_connections_total"], routeLabels)[0]))
			o.Expect(findGaugesWithLabels(updatedMetrics["haproxy_server_bytes_in_total"], serverLabels)[0]).To(o.BeNumerically(">=", findGaugesWithLabels(metrics["haproxy_server_bytes_in_total"], serverLabels)[0]))
			// max_sessions should reset after a reload, it is not possible to deterministically ensure max sessions is captured due to the
			// 30s scrape interval of router + the likelihood the router is being reloaded is high. Just verify that the value is reset
			// because no one else should be hitting this server.
			o.Expect(findGaugesWithLabels(updatedMetrics["haproxy_server_max_sessions"], serverLabels)[0]).To(o.Equal(float64(0)))
		})

		g.It("should expose the profiling endpoints", g.Label("Size:S"), func() {
			execPodName = exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod").Name
			defer func() {
				oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPodName, *metav1.NewDeleteOptions(1))
			}()

			g.By("preventing access without a username and password")
			err := expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s/debug/pprof/heap", net.JoinHostPort(host, strconv.Itoa(int(metricsPort)))), 401, 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("at /debug/pprof")
			results, err := getAuthenticatedURLViaPod(ns, execPodName, fmt.Sprintf("http://%s/debug/pprof/heap?debug=1", net.JoinHostPort(host, strconv.Itoa(int(metricsPort)))), username, password)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(results).To(o.ContainSubstring("# runtime.MemStats"))
		})

		g.It("should enable openshift-monitoring to pull metrics", g.Label("Size:M"), func(ctx g.SpecContext) {
			url, err := prometheus.PrometheusServiceURL(ctx, oc)
			if errors.IsNotFound(err) {
				g.Skip("prometheus not found on this cluster")
			}
			o.Expect(err).NotTo(o.HaveOccurred(), "Get url of prometheus service")
			token, err := prometheus.RequestPrometheusServiceAccountAPIToken(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "Request prometheus service account API token")
			execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
			defer func() {
				_ = oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
			}()

			o.Expect(wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
				contents, err := prometheus.GetBearerTokenURLViaPod(oc, execPod.Name, fmt.Sprintf("%s/api/v1/targets?state=active", url), token)
				o.Expect(err).NotTo(o.HaveOccurred())

				targets := &promTargets{}
				err = json.Unmarshal([]byte(contents), targets)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying router-internal-default job has a working target")
				err = targets.Expect(promLabels{"job": "router-internal-default"}, "up", "^https://.*/metrics$")
				if err != nil {
					e2e.Logf("missing router-internal-default target: %v", err)
					return false, nil
				}
				return true, nil
			})).NotTo(o.HaveOccurred())
		})
	})
})

type promLabels map[string]string

func (l promLabels) With(name, value string) promLabels {
	n := make(promLabels)
	for k, v := range l {
		n[k] = v
	}
	n[name] = value
	return n
}

type promTargets struct {
	Data struct {
		ActiveTargets []struct {
			Labels    map[string]string
			Health    string
			ScrapeUrl string
		}
	}
	Status string
}

func (t *promTargets) Expect(l promLabels, health, scrapeURLPattern string) error {
	for _, target := range t.Data.ActiveTargets {
		match := true
		for k, v := range l {
			if target.Labels[k] != v {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if health != target.Health {
			continue
		}
		if !regexp.MustCompile(scrapeURLPattern).MatchString(target.ScrapeUrl) {
			continue
		}
		return nil
	}
	return fmt.Errorf("no match for %v with health %s and scrape URL %s", l, health, scrapeURLPattern)
}

func findMetricsWithLabels(f *dto.MetricFamily, promLabels map[string]string) []*dto.Metric {
	var result []*dto.Metric
	if f == nil {
		return result
	}
	for _, m := range f.Metric {
		matched := map[string]struct{}{}
		for _, l := range m.Label {
			if expect, ok := promLabels[l.GetName()]; ok {
				if expect != l.GetValue() {
					break
				}
				matched[l.GetName()] = struct{}{}
			}
		}
		if len(matched) != len(promLabels) {
			continue
		}
		result = append(result, m)
	}
	return result
}

func findCountersWithLabels(f *dto.MetricFamily, promLabels map[string]string) []float64 {
	var result []float64
	for _, m := range findMetricsWithLabels(f, promLabels) {
		result = append(result, m.Counter.GetValue())
	}
	return result
}

func findGaugesWithLabels(f *dto.MetricFamily, promLabels map[string]string) []float64 {
	return findGaugesWithLabelsCond(f, promLabels, nil)
}

func findGaugesWithLabelsCond(f *dto.MetricFamily, promLabels map[string]string, cond func(v float64) bool) []float64 {
	var result []float64
	for _, m := range findMetricsWithLabels(f, promLabels) {
		if cond == nil || cond(m.Gauge.GetValue()) {
			result = append(result, m.Gauge.GetValue())
		}
	}
	return result
}

// findNonZeroGaugesWithLabels skips gauges with 0 value to avoid false negatives
// from the server slots which are DOWN.
// The Dynamic Configuration Manager (DCM) adds dynamic server slots and
// uses both these slots and the servers added during HAProxy config rendering.
// When DCM is enabled, not all server slots are always in the UP state,
// unlike the behavior without DCM where all slots are UP by default.
// As a result, we may want to filter out servers where the gauge is set to 0 (== DOWN).
// TODO: metrics for the slots which are DOWN should not be exposed to avoid
// the ambiguity of the zero value (no value or zero value) when DCM is enabled.
func findNonZeroGaugesWithLabels(f *dto.MetricFamily, promLabels map[string]string) []float64 {
	return findGaugesWithLabelsCond(f, promLabels, nonZeroCond)
}

func nonZeroCond(v float64) bool {
	return v != float64(0)
}

func findMetricLabels(f *dto.MetricFamily, promLabels map[string]string, match string) []string {
	var result []string
	for _, m := range findMetricsWithLabels(f, promLabels) {
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
	output, err := e2eoutput.RunHostCmd(ns, execPodName, cmd)
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
	output, err := e2eoutput.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func waitForAdmittedRoute(maxInterval time.Duration, client routev1client.RouteV1Interface, ns, name, ingressName string, errorOnRejection bool) (string, error) {
	var routeHost string
	err := wait.PollImmediate(time.Second, maxInterval, func() (bool, error) {
		route, err := client.Routes(ns).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		ingress := findIngress(route, ingressName)
		if ingress == nil {
			return false, nil
		}
		if len(ingress.Conditions) == 0 || ingress.Conditions[0].Type != routev1.RouteAdmitted {
			return false, nil
		}
		if errorOnRejection && ingress.Conditions[0].Status == corev1.ConditionFalse {
			return false, fmt.Errorf("router rejected route: %#v", ingress)
		}
		if ingress.Conditions[0].Status != corev1.ConditionTrue {
			return false, nil
		}
		routeHost = ingress.Host
		return true, nil
	})
	return routeHost, err
}
