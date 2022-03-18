package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	v1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/kubernetes/pkg/client/conditions"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	maxPrometheusQueryAttempts = 5
	prometheusQueryRetrySleep  = 10 * time.Second
)

// PrometheusResponse is used to contain prometheus query results
type PrometheusResponse struct {
	Data prometheusResponseData `json:"data"`
}

type prometheusResponseData struct {
	Result model.Vector `json:"result"`
}

// GetBearerTokenURLViaPod makes http request through given pod
func GetBearerTokenURLViaPod(ns, execPodName, url, bearer string) (string, error) {
	cmd := fmt.Sprintf("curl --retry 15 --max-time 2 --retry-delay 1 -s -k -H 'Authorization: Bearer %s' %q", bearer, url)
	output, err := framework.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func waitForServiceAccountInNamespace(c clientset.Interface, ns, serviceAccountName string, timeout time.Duration) error {
	w, err := c.CoreV1().ServiceAccounts(ns).Watch(context.Background(), metav1.SingleObject(metav1.ObjectMeta{Name: serviceAccountName}))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err = watchtools.UntilWithoutRetry(ctx, w, conditions.ServiceAccountHasSecrets)
	return err
}

// LocatePrometheus uses an exisitng CLI to return information used to make http requests to Prometheus.
func LocatePrometheus(oc *exutil.CLI) (queryURL, prometheusURL, bearerToken string, ok bool) {
	_, err := oc.AdminKubeClient().CoreV1().Services("openshift-monitoring").Get(context.Background(), "prometheus-k8s", metav1.GetOptions{})
	if kapierrs.IsNotFound(err) {
		return "", "", "", false
	}

	waitForServiceAccountInNamespace(oc.AdminKubeClient(), "openshift-monitoring", "prometheus-k8s", 2*time.Minute)
	for i := 0; i < 30; i++ {
		secrets, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-monitoring").List(context.Background(), metav1.ListOptions{})
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
			framework.Logf("Waiting for prometheus service account secret to show up")
			time.Sleep(time.Second)
			continue
		}
	}
	o.Expect(bearerToken).ToNot(o.BeEmpty())

	return "https://thanos-querier.openshift-monitoring.svc:9091", "https://prometheus-k8s.openshift-monitoring.svc:9091", bearerToken, true
}

type MetricCondition struct {
	Selector map[string]string
	Text     string
	Matches  func(sample *model.Sample) bool
}

type MetricConditions []MetricCondition

func (c MetricConditions) Matches(sample *model.Sample) *MetricCondition {
	for i, condition := range c {
		matches := true
		for name, value := range condition.Selector {
			if sample.Metric[model.LabelName(name)] != model.LabelValue(value) {
				matches = false
				break
			}
		}
		if matches && (condition.Matches == nil || condition.Matches(sample)) {
			return &c[i]
		}
	}
	return nil
}

func LabelsAsSelector(l model.LabelSet) string {
	return l.String()
}

func StripLabels(m model.Metric, names ...string) model.LabelSet {
	labels := make(model.LabelSet)
	for k := range m {
		labels[k] = m[k]
	}
	for _, name := range names {
		delete(labels, model.LabelName(name))
	}
	return labels
}

func RunQuery(ctx context.Context, prometheusClient prometheusv1.API, query string) (*PrometheusResponse, error) {
	return RunQueryAtTime(ctx, prometheusClient, query, time.Now())
}

func RunQueryAtTime(ctx context.Context, prometheusClient prometheusv1.API, query string, evaluationTime time.Time) (*PrometheusResponse, error) {
	result, warnings, err := prometheusClient.Query(ctx, query, evaluationTime)
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		errs := []error{}
		for _, warning := range warnings {
			errs = append(errs, fmt.Errorf("%s", warning))
		}
		return nil, errors.NewAggregate(errs)
	}
	if result.Type() != model.ValVector {
		return nil, fmt.Errorf("result type is not the vector: %v", result.Type())
	}
	return &PrometheusResponse{
		Data: prometheusResponseData{
			Result: result.(model.Vector),
		},
	}, nil
}

// RunQueries executes Prometheus queries and checks provided expected result.
func RunQueries(ctx context.Context, prometheusClient prometheusv1.API, promQueries map[string]bool, oc *exutil.CLI) error {
	// expect all correct metrics within a reasonable time period
	queryErrors := make(map[string]error)
	passed := make(map[string]struct{})
	for i := 0; i < maxPrometheusQueryAttempts; i++ {
		for query, expected := range promQueries {
			if _, ok := passed[query]; ok {
				continue
			}
			//TODO when the http/query apis discussed at https://github.com/prometheus/client_golang#client-for-the-prometheus-http-api
			// and introduced at https://github.com/prometheus/client_golang/blob/master/api/prometheus/v1/api.go are vendored into
			// openshift/origin, look to replace this homegrown http request / query param with that API
			g.By("perform prometheus metric query " + query)
			result, err := RunQuery(ctx, prometheusClient, query)
			if err != nil {
				msg := err.Error()
				if prev, ok := queryErrors[query]; ok && prev.Error() != msg {
					framework.Logf("%s", prev.Error())
				}
				queryErrors[query] = err
				continue
			}
			metrics := result.Data.Result
			if (len(metrics) > 0 && !expected) || (len(metrics) == 0 && expected) {
				data, _ := json.MarshalIndent(result.Data.Result, "", "  ")
				msg := fmt.Sprintf("promQL query returned unexpected results:\n%s\n%s", query, data)
				if prev, ok := queryErrors[query]; ok && prev.Error() != msg {
					framework.Logf("%s", prev.Error())
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
		time.Sleep(prometheusQueryRetrySleep)
	}

	if len(queryErrors) != 0 {
		exutil.DumpPodLogsStartingWith("prometheus-k8s-", oc)
	}
	var errs []error
	for _, err := range queryErrors {
		errs = append(errs, err)
	}
	return errors.NewAggregate(errs)
}

// ExpectURLStatusCodeExec attempts connection to url returning an error
// upon failure or if status return code is not equal to any of the statusCodes.
func ExpectURLStatusCodeExec(ns, execPodName, url string, statusCodes ...int) error {
	cmd := fmt.Sprintf("curl -k -s -o /dev/null -w '%%{http_code}' %q", url)
	output, err := framework.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	for _, statusCode := range statusCodes {
		if output == strconv.Itoa(statusCode) {
			return nil
		}
	}

	return fmt.Errorf("last response from server was not in %v: %s", statusCodes, output)
}

// ExpectPrometheusEndpoint attempts to connect to the metrics endpoint with
// delayed retries upon failure.
func ExpectPrometheusEndpoint(namespace, podName, url string) {
	var err error
	for i := 0; i < maxPrometheusQueryAttempts; i++ {
		err = ExpectURLStatusCodeExec(namespace, podName, url, 401, 403)
		if err == nil {
			break
		}
		time.Sleep(prometheusQueryRetrySleep)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

// FetchAlertingRules fetchs all alerting rules from the Prometheus at
// the given URL using the given bearer token.  The results are returned
// as a map of group names to lists of alerting rules.
func FetchAlertingRules(oc *exutil.CLI, promURL, bearerToken string) (map[string][]promv1.AlertingRule, error) {
	ns := oc.SetupNamespace()
	execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
	defer func() {
		oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
	}()

	url := fmt.Sprintf("%s/api/v1/rules", promURL)
	contents, err := GetBearerTokenURLViaPod(ns, execPod.Name, url, bearerToken)
	if err != nil {
		return nil, fmt.Errorf("unable to query %s: %v", url, err)
	}

	var result struct {
		Status string             `json:"status"`
		Data   promv1.RulesResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(contents), &result); err != nil {
		return nil, fmt.Errorf("unable to parse response %q from %s: %v", contents, url, err)
	}

	alertingRules := make(map[string][]promv1.AlertingRule)

	for _, rg := range result.Data.Groups {
		for _, r := range rg.Rules {
			switch v := r.(type) {
			case promv1.RecordingRule:
				continue
			case promv1.AlertingRule:
				alertingRules[rg.Name] = append(alertingRules[rg.Name], v)
			default:
				return nil, fmt.Errorf("unexpected rule of type %T", r)
			}
		}
	}

	return alertingRules, nil
}

// ValidateURL takes a URL as a string and a timeout.  The URL is
// parsed, then fetched until a 200 response is received or the timeout
// is reached.
func ValidateURL(rawURL string, timeout time.Duration) error {
	if _, err := url.Parse(rawURL); err != nil {
		return err
	}

	return wait.PollImmediate(1*time.Second, timeout, func() (done bool, err error) {
		resp, err := http.Get(rawURL)
		if err != nil {
			framework.Logf("validateURL(%q) error: %v", err)
			return false, nil
		}

		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			return true, nil
		}

		framework.Logf("validateURL(%q) got non-200 response: %d", rawURL, resp.StatusCode)
		return false, nil
	})
}

// ForEachAlertingRule takes a map of rule group names to a list of
// alerting rules, and for each rule in each group runs the given
// function.  The function takes the alerting rule, and returns a set of
// violations, which maye be empty or nil.  If after all rules are
// checked, there are any violations, an error is returned.
func ForEachAlertingRule(rules map[string][]promv1.AlertingRule, f func(a promv1.AlertingRule) sets.String) error {
	allViolations := sets.NewString()

	for group, alerts := range rules {
		for _, alert := range alerts {
			// The Watchdog alert is special because it is only there to
			// test the end-to-end alert flow and it isn't meant to be
			// routed to cluster admins.
			if alert.Name == "Watchdog" {
				continue
			}

			if violations := f(alert); violations != nil {
				for _, v := range violations.List() {
					allViolations.Insert(
						fmt.Sprintf("Alerting rule %q (group: %s) %s", alert.Name, group, v),
					)
				}
			}
		}
	}

	if len(allViolations) == 0 {
		return nil // No violations
	}

	return fmt.Errorf("Incompliant rules detected:\n\n%s", strings.Join(allViolations.List(), "\n"))
}
