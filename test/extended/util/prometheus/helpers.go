package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/errors"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/prometheus/common/model"

	v1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Status string                 `json:"status"`
	Data   prometheusResponseData `json:"data"`
}

type prometheusResponseData struct {
	ResultType string       `json:"resultType"`
	Result     model.Vector `json:"result"`
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

// LocatePrometheus uses an exisitng CLI to return information used to make http requests to Prometheus
func LocatePrometheus(oc *exutil.CLI) (url, bearerToken string, ok bool) {
	_, err := oc.AdminKubeClient().CoreV1().Services("openshift-monitoring").Get(context.Background(), "prometheus-k8s", metav1.GetOptions{})
	if kapierrs.IsNotFound(err) {
		return "", "", false
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

	return "https://prometheus-k8s.openshift-monitoring.svc:9091", bearerToken, true
}

// ExpectPrometheus uses an existing framework to return information used to make http requests to Prometheus
func ExpectPrometheus(f *framework.Framework) (url, bearerToken string, oc *exutil.CLI, ok bool) {

	// Must use version that's can run within Ginkgo It
	oc = exutil.NewCLIWithFramework(f)

	url, bearerToken, ok = LocatePrometheus(oc)

	return url, bearerToken, oc, ok
}

// RunQueries executes Prometheus queries and checks provided expected result.
func RunQueries(promQueries map[string]bool, oc *exutil.CLI, ns, execPodName, baseURL, bearerToken string) error {
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
			url := fmt.Sprintf("%s/api/v1/query?%s", baseURL, (url.Values{"query": []string{query}}).Encode())
			contents, err := GetBearerTokenURLViaPod(ns, execPodName, url, bearerToken)
			if err != nil {
				return err
			}

			// check query result, if this is a new error log it, otherwise remain silent
			var result PrometheusResponse
			if err := json.Unmarshal([]byte(contents), &result); err != nil {
				framework.Logf("unable to parse query response for %s: %v", query, err)
				continue
			}
			metrics := result.Data.Result
			if result.Status != "success" {
				data, _ := json.Marshal(metrics)
				msg := fmt.Sprintf("promQL query: %s had reported incorrect status:\n%s", query, data)
				if prev, ok := queryErrors[query]; !ok || prev.Error() != msg {
					framework.Logf("%s", msg)
				}
				queryErrors[query] = fmt.Errorf(msg)
				continue
			}
			if (len(metrics) > 0 && !expected) || (len(metrics) == 0 && expected) {
				data, _ := json.Marshal(metrics)
				msg := fmt.Sprintf("promQL query: %s had reported incorrect results:\n%s", query, data)
				if prev, ok := queryErrors[query]; !ok || prev.Error() != msg {
					framework.Logf("%s", msg)
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
		exutil.DumpPodLogsStartingWith("prometheus-0", oc)
	}
	var errs []error
	for query, err := range queryErrors {
		errs = append(errs, fmt.Errorf("query failed: %s: %s", query, err))
	}
	return errors.NewAggregate(errs)
}

// ExpectURLStatusCodeExec attempts connection to url returning an error
// upon failure or if status return code is not equal to statusCode.
func ExpectURLStatusCodeExec(ns, execPodName, url string, statusCode int) error {
	cmd := fmt.Sprintf("curl -k -s -o /dev/null -w '%%{http_code}' %q", url)
	output, err := framework.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	if output != strconv.Itoa(statusCode) {
		return fmt.Errorf("last response from server was not %d: %s", statusCode, output)
	}
	return nil
}

// ExpectPrometheusEndpoint attempts to connect to the metrics endpoint with
// delayed retries upon failure.
func ExpectPrometheusEndpoint(namespace, podName, url string) {
	var err error
	for i := 0; i < maxPrometheusQueryAttempts; i++ {
		err = ExpectURLStatusCodeExec(namespace, podName, url, 403)
		if err == nil {
			break
		}
		time.Sleep(prometheusQueryRetrySleep)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}
