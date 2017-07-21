package prometheus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/client/conditions"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Prometheus][Conformance] Prometheus", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("prometheus", exutil.KubeConfigPath())

		execPodName, ns, host, bearerToken string
		statsPort                          int
	)

	g.BeforeEach(func() {
		ns = oc.KubeFramework().Namespace.Name
		host = "prometheus.kube-system.svc"
		statsPort = 443
		mustCreate := false
		if _, err := oc.AdminKubeClient().Extensions().Deployments("kube-system").Get("prometheus", metav1.GetOptions{}); err != nil {
			if !kapierrs.IsNotFound(err) {
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			mustCreate = true
		}

		if mustCreate {
			e2e.Logf("Installing Prometheus onto the cluster for testing")
			configPath := exutil.FixturePath("..", "..", "examples", "prometheus", "prometheus.yaml")
			stdout, _, err := oc.WithoutNamespace().Run("process").Args("-f", configPath).Outputs()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.WithoutNamespace().AsAdmin().Run("create").Args("-f", "-").InputString(stdout).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.WaitForDeploymentStatus(oc.AdminKubeClient(), &extensions.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "prometheus", Namespace: "kube-system"}})
		}

		waitForServiceAccountInNamespace(oc.AdminKubeClient(), "kube-system", "prometheus", 2*time.Minute)
		secrets, err := oc.AdminKubeClient().Core().Secrets("kube-system").List(metav1.ListOptions{})
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
		o.Expect(bearerToken).ToNot(o.BeEmpty())
	})

	g.Describe("when installed to the cluster", func() {
		g.It("should start and expose a secured proxy and unsecured metrics", func() {
			execPodName = e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) {
				pod.Spec.Containers[0].Image = "centos:7"
			})
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By("checking the unsecured metrics path")
			success := false
			var metrics map[string]*dto.MetricFamily
			for i := 0; i < 30; i++ {
				results, err := getInsecureURLViaPod(ns, execPodName, fmt.Sprintf("https://%s:%d/metrics", host, statsPort))
				if err != nil {
					e2e.Logf("unable to get unsecured metrics: %v", err)
					continue
				}
				//e2e.Logf("Metrics:\n%s", results)

				p := expfmt.TextParser{}
				metrics, err = p.TextToMetricFamilies(bytes.NewBufferString(results))
				o.Expect(err).NotTo(o.HaveOccurred())

				counts := findCountersWithLabels(metrics["tsdb_samples_appended_total"], labels{})
				if len(counts) == 0 || counts[0] == 0 {
					time.Sleep(time.Second)
					continue
				}
				success = true
				break
			}
			o.Expect(success).To(o.BeTrue(), fmt.Sprintf("Did not find tsdb_samples_appended_total in:\n%#v,", metrics))

			g.By("verifying the oauth-proxy reports a 403 on the root URL")
			err := expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("https://%s:%d", host, statsPort), 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying a service account token is able to authenticate")
			err = expectBearerTokenURLStatusCodeExec(ns, execPodName, fmt.Sprintf("https://%s:%d/graph", host, statsPort), bearerToken, 200)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying a service account token is able to access the Prometheus API")
			// expect all endpoints within 60 seconds
			var lastErrs []error
			for i := 0; i < 60; i++ {
				contents, err := getBearerTokenURLViaPod(ns, execPodName, fmt.Sprintf("https://%s:%d/api/v1/targets", host, statsPort), bearerToken)
				o.Expect(err).NotTo(o.HaveOccurred())

				targets := &prometheusTargets{}
				err = json.Unmarshal([]byte(contents), targets)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying all expected jobs have a working target")
				lastErrs = all(
					targets.Expect(labels{"job": "kubernetes-apiservers"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "kubernetes-service-endpoints", "kubernetes_name": "prometheus", "kubernetes_namespace": "kube-system", "name": "prometheus"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "kubernetes-nodes"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "kubernetes-cadvisor"}, "up", "^https://.*/metrics/cadvisor$"),
				)
				if len(lastErrs) == 0 {
					break
				}
				time.Sleep(time.Second)
			}
			o.Expect(lastErrs).To(o.BeEmpty())
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
		o.Expect(target.ScrapeUrl).To(o.MatchRegexp(scrapeURLPattern))
		return nil
	}
	return fmt.Errorf("no match for %v with health %s", l, health)
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
	w, err := c.Core().ServiceAccounts(ns).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: serviceAccountName}))
	if err != nil {
		return err
	}
	_, err = watch.Until(timeout, w, conditions.ServiceAccountHasSecrets)
	return err
}
