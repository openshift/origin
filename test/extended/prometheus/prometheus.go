package prometheus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	v1 "k8s.io/api/core/v1"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	clientset "k8s.io/client-go/kubernetes"
	watchtools "k8s.io/client-go/tools/watch"

	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/conditions"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"

	operatorv1 "github.com/openshift/api/operator/v1"
)

const waitForPrometheusStartSeconds = 240

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

	g.Describe("when installed on the cluster", func() {
		g.It("should report telemetry if a cloud.openshift.com token is present", func() {
			if !hasPullSecret(oc.AdminKubeClient(), "cloud.openshift.com") {
				e2e.Skipf("Telemetry is disabled")
			}
			oc.SetupProject()
			ns := oc.Namespace()

			execPodName := e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) { pod.Spec.Containers[0].Image = "centos:7" })
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			tests := map[string][]metricTest{
				// should have successfully sent at least once to remote
				`metricsclient_request_send{client="federate_to",job="telemeter-client",status_code="200"}`: {metricTest{greaterThanEqual: true, value: 1}},
				// should have scraped some metrics from prometheus
				`federate_samples{job="telemeter-client"}`: {metricTest{greaterThanEqual: true, value: 10}},
			}
			runQueries(tests, oc, ns, execPodName, url, bearerToken)

			e2e.Logf("Telemetry is enabled: %s", bearerToken)
		})

		g.It("should start and expose a secured proxy and unsecured metrics", func() {
			oc.SetupProject()
			ns := oc.Namespace()
			execPodName := e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) { pod.Spec.Containers[0].Image = "centos:7" })
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By("checking the unsecured metrics path")
			var metrics map[string]*dto.MetricFamily
			o.Expect(wait.PollImmediate(10*time.Second, waitForPrometheusStartSeconds*time.Second, func() (bool, error) {
				results, err := getInsecureURLViaPod(ns, execPodName, fmt.Sprintf("%s/metrics", url))
				if err != nil {
					e2e.Logf("unable to get unsecured metrics: %v", err)
					return false, nil
				}

				p := expfmt.TextParser{}
				metrics, err = p.TextToMetricFamilies(bytes.NewBufferString(results))
				o.Expect(err).NotTo(o.HaveOccurred())
				// original field in 2.0.0-beta
				counts := findCountersWithLabels(metrics["tsdb_samples_appended_total"], labels{})
				if len(counts) != 0 && counts[0] > 0 {
					return true, nil
				}
				// 2.0.0-rc.0
				counts = findCountersWithLabels(metrics["tsdb_head_samples_appended_total"], labels{})
				if len(counts) != 0 && counts[0] > 0 {
					return true, nil
				}
				// 2.0.0-rc.2
				counts = findCountersWithLabels(metrics["prometheus_tsdb_head_samples_appended_total"], labels{})
				if len(counts) != 0 && counts[0] > 0 {
					return true, nil
				}
				return false, nil
			})).NotTo(o.HaveOccurred(), fmt.Sprintf("Did not find tsdb_samples_appended_total, tsdb_head_samples_appended_total, or prometheus_tsdb_head_samples_appended_total"))

			g.By("verifying the oauth-proxy reports a 403 on the root URL")
			err := expectURLStatusCodeExec(ns, execPodName, url, 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying a service account token is able to authenticate")
			err = expectBearerTokenURLStatusCodeExec(ns, execPodName, fmt.Sprintf("%s/graph", url), bearerToken, 200)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying a service account token is able to access the Prometheus API")
			// expect all endpoints within 60 seconds
			var lastErrs []error
			o.Expect(wait.PollImmediate(10*time.Second, 2*time.Minute, func() (bool, error) {
				contents, err := getBearerTokenURLViaPod(ns, execPodName, fmt.Sprintf("%s/api/v1/targets", url), bearerToken)
				o.Expect(err).NotTo(o.HaveOccurred())

				targets := &prometheusTargets{}
				err = json.Unmarshal([]byte(contents), targets)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying all expected jobs have a working target")
				lastErrs = all(
					// The OpenShift control plane
					targets.Expect(labels{"job": "api"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "controller-manager"}, "up", "^https://.*/metrics$"),

					// The kube control plane
					targets.Expect(labels{"job": "etcd"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "apiserver"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "kube-controller-manager"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "scheduler"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "kube-state-metrics"}, "up", "^https://.*/metrics$"),

					// TODO: should probably be https
					targets.Expect(labels{"job": "cluster-version-operator"}, "up", "^http://.*/metrics$"),
					targets.Expect(labels{"job": "prometheus-k8s", "namespace": "openshift-monitoring", "pod": "prometheus-k8s-0"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "kubelet"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "kubelet"}, "up", "^https://.*/metrics/cadvisor$"),
					targets.Expect(labels{"job": "node-exporter"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "prometheus-operator"}, "up", "^http://.*/metrics$"),
					targets.Expect(labels{"job": "alertmanager-main"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "crio"}, "up", "^http://.*/metrics$"),
					targets.Expect(labels{"job": "telemeter-client"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "sdn"}, "up", "^http://.*/metrics$"),
				)
				if len(lastErrs) > 0 {
					e2e.Logf("missing some targets: %v", lastErrs)
					return false, nil
				}
				return true, nil
			})).NotTo(o.HaveOccurred())
		})
		g.It("should have a Watchdog alert in firing state", func() {
			oc.SetupProject()
			ns := oc.Namespace()
			execPodName := e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) { pod.Spec.Containers[0].Image = "centos:7" })
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			tests := map[string][]metricTest{
				// should have a constantly firing watchdog alert
				`ALERTS{alertstate="firing",alertname="Watchdog"}`: {metricTest{greaterThanEqual: true, value: 1}},
				// should be only one watchdog alert (this is a workaround as metricTest doesn't offer equality operator)
				`ALERTS{alertstate="firing",alertname="Watchdog",severity="none"}`: {metricTest{greaterThanEqual: false, value: 2}},
			}
			runQueries(tests, oc, ns, execPodName, url, bearerToken)

			e2e.Logf("Watchdog alert is firing")
		})
		g.It("should have non-Pod host cAdvisor metrics", func() {
			oc.SetupProject()
			ns := oc.Namespace()
			execPodName := e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) { pod.Spec.Containers[0].Image = "centos:7" })
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			tests := map[string][]metricTest{
				// should have constantly firing a watchdog alert
				`container_cpu_usage_seconds_total{id!~"/kubepods.slice/.*"}`: {metricTest{greaterThanEqual: true, value: 1}},
			}
			runQueries(tests, oc, ns, execPodName, url, bearerToken)
		})
		g.It("should be able to get the sdn ovs flows", func() {
			oc.SetupProject()
			ns := oc.Namespace()
			execPodName := e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) { pod.Spec.Containers[0].Image = "centos:7" })
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			tests := map[string][]metricTest{
				//something
				`openshift_sdn_ovs_flows`: {metricTest{greaterThanEqual: true, value: 1}},
			}
			runQueries(tests, oc, ns, execPodName, url, bearerToken)
		})
		g.It("should report less than two alerts in firing or pending state", func() {
			oc.SetupProject()
			ns := oc.Namespace()
			execPodName := e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) { pod.Spec.Containers[0].Image = "centos:7" })
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			// needed for cluster to settle and have metrics and alerts usable
			time.Sleep(5 * time.Minute)

			tests := map[string][]metricTest{
				// should be checking there is no more than 1 alerts firing.
				// Checking for specific alert is done in "should have a Watchdog alert in firing state".
				`sum(ALERTS{alertstate="firing"})`: {metricTest{greaterThanEqual: false, value: 2}},
			}
			runQueries(tests, oc, ns, execPodName, url, bearerToken)
		})
		g.It("should have machine api operator metrics", func() {
			oc.SetupProject()
			ns := oc.Namespace()
			execPodName := e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) { pod.Spec.Containers[0].Image = "centos:7" })
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			tests := map[string][]metricTest{
				`mapi_machine_set_status_replicas`: {metricTest{greaterThanEqual: true, value: 1}},
			}
			runQueries(tests, oc, ns, execPodName, url, bearerToken)
		})
		g.It("should provide ingress metrics", func() {
			oc.SetupProject()
			ns := oc.Namespace()

			execPodName := e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) { pod.Spec.Containers[0].Image = "centos:7" })
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By("creating a non-default ingresscontroller")
			replicas := int32(1)
			ingress := &operatorv1.IngressController{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "openshift-ingress-operator",
					Name:      "prometheus",
				},
				Spec: operatorv1.IngressControllerSpec{
					Domain:   "prometheus.e2e.openshift.example.com",
					Replicas: &replicas,
					EndpointPublishingStrategy: &operatorv1.EndpointPublishingStrategy{
						Type: operatorv1.PrivateStrategyType,
					},
					RouteSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"e2e-test": ns,
						},
					},
				},
			}
			_, err := oc.AdminOperatorClient().OperatorV1().IngressControllers(ingress.Namespace).Create(ingress)
			o.Expect(err).NotTo(o.HaveOccurred())

			defer func() {
				if err := oc.AdminOperatorClient().OperatorV1().IngressControllers(ingress.Namespace).Delete(ingress.Name, metav1.NewDeleteOptions(1)); err != nil {
					e2e.Logf("WARNING: failed to delete ingresscontroller '%s/%s' created during test cleanup: %v", ingress.Namespace, ingress.Name, err)
				} else {
					e2e.Logf("deleted test ingresscontroller '%s/%s'", ingress.Namespace, ingress.Name)
				}
			}()

			var lastErrs []error
			o.Expect(wait.PollImmediate(10*time.Second, 4*time.Minute, func() (bool, error) {
				contents, err := getBearerTokenURLViaPod(ns, execPodName, fmt.Sprintf("%s/api/v1/targets", url), bearerToken)
				o.Expect(err).NotTo(o.HaveOccurred())

				targets := &prometheusTargets{}
				err = json.Unmarshal([]byte(contents), targets)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying all expected jobs have a working target")
				lastErrs = all(
					// Is there a good way to discover the name and thereby avoid leaking the naming algorithm?
					targets.Expect(labels{"job": "router-internal-default"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "router-internal-prometheus"}, "up", "^https://.*/metrics$"),
				)
				if len(lastErrs) > 0 {
					e2e.Logf("missing some targets: %v", lastErrs)
					return false, nil
				}
				return true, nil
			})).NotTo(o.HaveOccurred())

			g.By("verifying standard metrics keys")
			queries := map[string][]metricTest{
				`template_router_reload_seconds_count{job="router-internal-default"}`:    {metricTest{greaterThanEqual: true, value: 1}},
				`haproxy_server_up{job="router-internal-default"}`:                       {metricTest{greaterThanEqual: true, value: 1}},
				`template_router_reload_seconds_count{job="router-internal-prometheus"}`: {metricTest{greaterThanEqual: true, value: 1}},
				`haproxy_server_up{job="router-internal-prometheus"}`:                    {metricTest{greaterThanEqual: true, value: 1}},
			}
			runQueries(queries, oc, ns, execPodName, url, bearerToken)
		})
	})
})

func all(errs ...error) []error {
	var result []error
	for _, err := range errs {
		if err != nil {
			result = append(result, err)
		}
	}
	return result
}

type prometheusTargets struct {
	Data struct {
		ActiveTargets []struct {
			Labels    map[string]string
			Health    string
			ScrapeUrl string
		}
	}
	Status string
}

func (t *prometheusTargets) Expect(l labels, health, scrapeURLPattern string) error {
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
	cmd := fmt.Sprintf("curl -k -s -o /dev/null -w '%%{http_code}' %q", url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	if output != strconv.Itoa(statusCode) {
		return fmt.Errorf("last response from server was not %d: %s", statusCode, output)
	}
	return nil
}

func expectBearerTokenURLStatusCodeExec(ns, execPodName, url, bearer string, statusCode int) error {
	cmd := fmt.Sprintf("curl -k -s -H 'Authorization: Bearer %s' -o /dev/null -w '%%{http_code}' %q", bearer, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	if output != strconv.Itoa(statusCode) {
		return fmt.Errorf("last response from server was not %d: %s", statusCode, output)
	}
	return nil
}

func getBearerTokenURLViaPod(ns, execPodName, url, bearer string) (string, error) {
	cmd := fmt.Sprintf("curl -s -k -H 'Authorization: Bearer %s' %q", bearer, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func getAuthenticatedURLViaPod(ns, execPodName, url, user, pass string) (string, error) {
	cmd := fmt.Sprintf("curl -s -u %s:%s %q", user, pass, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func getInsecureURLViaPod(ns, execPodName, url string) (string, error) {
	cmd := fmt.Sprintf("curl -s -k %q", url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func waitForServiceAccountInNamespace(c clientset.Interface, ns, serviceAccountName string, timeout time.Duration) error {
	w, err := c.CoreV1().ServiceAccounts(ns).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: serviceAccountName}))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err = watchtools.UntilWithoutRetry(ctx, w, conditions.ServiceAccountHasSecrets)
	return err
}

func locatePrometheus(oc *exutil.CLI) (url, bearerToken string, ok bool) {
	_, err := oc.AdminKubeClient().CoreV1().Services("openshift-monitoring").Get("prometheus-k8s", metav1.GetOptions{})
	if kapierrs.IsNotFound(err) {
		return "", "", false
	}

	waitForServiceAccountInNamespace(oc.AdminKubeClient(), "openshift-monitoring", "prometheus-k8s", 2*time.Minute)
	for i := 0; i < 30; i++ {
		secrets, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-monitoring").List(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, secret := range secrets.Items {
			if secret.Type != v1.SecretTypeServiceAccountToken {
				continue
			}
			if !strings.HasPrefix(secret.Name, "prometheus-") {
				continue
			}
			bearerToken = string(secret.Data[v1.ServiceAccountTokenKey])
			break
		}
		if len(bearerToken) == 0 {
			e2e.Logf("Waiting for prometheus service account secret to show up")
			time.Sleep(time.Second)
			continue
		}
	}
	o.Expect(bearerToken).ToNot(o.BeEmpty())

	return "https://prometheus-k8s.openshift-monitoring.svc:9091", bearerToken, true
}

func hasPullSecret(client clientset.Interface, name string) bool {
	scrt, err := client.CoreV1().Secrets("openshift-config").Get("pull-secret", metav1.GetOptions{})
	if err != nil {
		if kapierrs.IsNotFound(err) {
			return false
		}
		e2e.Failf("could not retrieve pull-secret: %v", err)
	}

	if scrt.Type != v1.SecretTypeDockerConfigJson {
		e2e.Failf("error expecting secret type %s got %s", v1.SecretTypeDockerConfigJson, scrt.Type)
	}

	ps := struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}{}

	if err := json.Unmarshal(scrt.Data[v1.DockerConfigJsonKey], &ps); err != nil {
		e2e.Failf("could not unmarshal pullSecret from openshift-config/pull-secret: %v", err)
	}
	return len(ps.Auths[name].Auth) > 0
}
