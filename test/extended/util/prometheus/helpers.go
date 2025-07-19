package prometheus

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
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

// GetURLWithToken makes an HTTP request with a bearer token.
func GetURLWithToken(url, bearerToken string) (string, error) {
	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			// Use the HTTP proxy configured in the environment variables.
			Proxy: http.ProxyFromEnvironment,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("%s: %w", url, err)
	}

	req.Header.Add("Authorization", "Bearer "+bearerToken)

	var (
		body    []byte
		lastErr error
	)
	condition := func(ctx context.Context) (bool, error) {
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%s: request failed: %w", url, err)
			return false, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("%s: unexpected status code: %d", url, resp.StatusCode)
			return false, nil
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("%s: failed to read response: %w", url, err)
			return false, nil
		}

		return true, nil
	}
	if err = wait.PollUntilContextTimeout(context.Background(), time.Second, time.Minute, true, condition); err != nil {
		return "", fmt.Errorf("%w: %w", err, lastErr)
	}

	return string(body), nil
}

const (
	namespace      = "openshift-monitoring"
	prometheusName = "prometheus-k8s"
	thanosName     = "thanos-querier"
	serviceAccount = prometheusName
)

// PrometheusServiceURL returns the url of the cluster prometheus service or an error if the service is not found.
func PrometheusServiceURL(ctx context.Context, oc *exutil.CLI) (string, error) {
	svc, err := oc.AdminKubeClient().CoreV1().Services(namespace).Get(ctx, prometheusName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get the %s service in the %s namespace: %w", prometheusName, namespace, err)
	}
	i := slices.IndexFunc(svc.Spec.Ports, func(port corev1.ServicePort) bool { return port.Name == "web" })
	return fmt.Sprintf("https://%s.%s.svc:%d", svc.Name, svc.Namespace, svc.Spec.Ports[i].Port), nil
}

// ThanosQuerierServiceURL returns the url of the thanos querier service or an error if the service is not found.
func ThanosQuerierServiceURL(ctx context.Context, oc *exutil.CLI) (string, error) {
	svc, err := oc.AdminKubeClient().CoreV1().Services(namespace).Get(ctx, thanosName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get the %s service in the %s namespace: %w", thanosName, namespace, err)
	}
	i := slices.IndexFunc(svc.Spec.Ports, func(port corev1.ServicePort) bool { return port.Name == "web" })
	return fmt.Sprintf("https://%s.%s.svc:%d", svc.Name, svc.Namespace, svc.Spec.Ports[i].Port), nil
}

// PrometheusRouteURL returns the public url of the cluster prometheus service or an error if the route is not found.
func PrometheusRouteURL(ctx context.Context, oc *exutil.CLI) (string, error) {
	rte, err := oc.AsAdmin().RouteClient().RouteV1().Routes(namespace).Get(ctx, prometheusName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get the %s route in the %s namespace: %w", prometheusName, namespace, err)
	}
	return "https://" + rte.Status.Ingress[0].Host, nil
}

// ThanosQuerierRouteURL returns the public url of the thanos querier service or an error if the route is not found.
func ThanosQuerierRouteURL(ctx context.Context, oc *exutil.CLI) (string, error) {
	rte, err := oc.AsAdmin().RouteClient().RouteV1().Routes(namespace).Get(ctx, thanosName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get the %s route in the %s namespace: %w", thanosName, namespace, err)
	}
	return "https://" + rte.Status.Ingress[0].Host, nil
}

// RequestPrometheusServiceAccountAPIToken returns a time-bound (24hr) API token for the prometheus service account.
func RequestPrometheusServiceAccountAPIToken(ctx context.Context, oc *exutil.CLI) (string, error) {
	expirationSeconds := int64(24 * time.Hour / time.Second)
	req, err := oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace).CreateToken(ctx, serviceAccount,
		&authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{ExpirationSeconds: &expirationSeconds},
		}, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get an API token for the %s service account in the %s namespace: %w", serviceAccount, namespace, err)
	}
	return req.Status.Token, nil
}

func RunQuery(ctx context.Context, prometheusClient prometheusv1.API, query string) (*PrometheusResponse, error) {
	return RunQueryAtTime(ctx, prometheusClient, query, time.Now())
}

func RunQueryAtTime(ctx context.Context, prometheusClient prometheusv1.API, query string, evaluationTime time.Time) (*PrometheusResponse, error) {
	var lastErr error
	var result model.Value
	var warnings prometheusv1.Warnings
	for i := 0; i < 5; i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		result, warnings, lastErr = prometheusClient.Query(ctx, query, evaluationTime)
		if lastErr == nil {
			break
		}

		select {
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
			break
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}

	if len(warnings) > 0 {
		framework.Logf("#### warnings \n\t%v\n", strings.Join(warnings, "\n\t"))
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
				queryErrors[query] = fmt.Errorf("%s", msg)
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

// ExpectHTTPStatusCode sends a GET request to the URL and checks the expected status code.
// If bearerToken isn't empty, it will be used to authenticate against the endpoint.
// It returns an error if the request fails or the status return code is not
// equal to any of the statusCodes.
func ExpectHTTPStatusCode(url, bearerToken string, statusCodes ...int) error {
	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			// Use the HTTP proxy configured in the environment variables.
			Proxy: http.ProxyFromEnvironment,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", url, err)
	}

	if len(bearerToken) > 0 {
		req.Header.Add("Authorization", "Bearer "+bearerToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: request failed: %w", url, err)
	}
	defer resp.Body.Close()

	for _, statusCode := range statusCodes {
		if resp.StatusCode == statusCode {
			return nil
		}
	}
	return fmt.Errorf("%s: last response from server was not in %v: %d", url, statusCodes, resp.StatusCode)
}

// ExpectURLStatusCodeExecViaPod attempts connection to url via exec pod and returns an error
// upon failure or if status return code is not equal to any of the statusCodes.
func ExpectURLStatusCodeExecViaPod(ns, execPodName, url string, statusCodes ...int) error {
	cmd := fmt.Sprintf("curl -k -s -o /dev/null -w '%%{http_code}' %q", url)
	output, err := e2eoutput.RunHostCmd(ns, execPodName, cmd)
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
func ExpectPrometheusEndpoint(url string) {
	var err error
	for i := 0; i < maxPrometheusQueryAttempts; i++ {
		err = ExpectHTTPStatusCode(url, "", 401, 403)
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
func FetchAlertingRules(promURL, bearerToken string) (map[string][]prometheusv1.AlertingRule, error) {
	url := fmt.Sprintf("%s/api/v1/rules", promURL)
	contents, err := GetURLWithToken(url, bearerToken)
	if err != nil {
		return nil, fmt.Errorf("unable to query %s: %v", url, err)
	}

	var result struct {
		Status string                   `json:"status"`
		Data   prometheusv1.RulesResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(contents), &result); err != nil {
		return nil, fmt.Errorf("unable to parse response %q from %s: %v", contents, url, err)
	}

	alertingRules := make(map[string][]prometheusv1.AlertingRule)

	for _, rg := range result.Data.Groups {
		for _, r := range rg.Rules {
			switch v := r.(type) {
			case prometheusv1.RecordingRule:
				continue
			case prometheusv1.AlertingRule:
				alertingRules[rg.Name] = append(alertingRules[rg.Name], v)
			default:
				return nil, fmt.Errorf("unexpected rule of type %T", r)
			}
		}
	}

	return alertingRules, nil
}

func ValidateURL(rawURL string) error {
	var u *url.URL
	var err error
	if u, err = url.Parse(rawURL); err != nil {
		return err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		errstr := fmt.Sprintf("%q: URL scheme is invalid, it should be 'http' or 'https'", rawURL)
		return fmt.Errorf("%s", errstr)
	}
	if u.Host == "" {
		errstr := fmt.Sprintf("%q: host should not be empty", rawURL)
		return fmt.Errorf("%s", errstr)
	}
	return nil
}

// QueryURL takes a URL as a string and a timeout.  The URL is
// parsed, then fetched until a 200 response is received or the timeout
// is reached.
func QueryURL(rawURL string, timeout time.Duration) error {
	if _, err := url.Parse(rawURL); err != nil {
		return err
	}

	return wait.PollImmediate(1*time.Second, timeout, func() (done bool, err error) {
		resp, err := http.Get(rawURL)
		if err != nil {
			framework.Logf("QueryURL(%q) error: %v", rawURL, err)
			return false, nil
		}

		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			return true, nil
		}

		framework.Logf("QueryURL(%q) got non-200 response: %d", rawURL, resp.StatusCode)
		return false, nil
	})
}

// ForEachAlertingRule takes a map of rule group names to a list of
// alerting rules, and for each rule in each group runs the given
// function.  The function takes the alerting rule, and returns a set of
// violations, which maye be empty or nil.  If after all rules are
// checked, there are any violations, an error is returned.
func ForEachAlertingRule(rules map[string][]prometheusv1.AlertingRule, f func(a prometheusv1.AlertingRule) sets.String) error {
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

// MustJoinUrlPath behaves like url.JoinPath but it will panic in case of error.
func MustJoinUrlPath(base string, paths ...string) string {
	path, err := url.JoinPath(base, paths...)
	if err != nil {
		panic(err)
	}
	return path
}

func GetBearerTokenURLViaPod(oc *exutil.CLI, execPodName, url, bearer string) (string, error) {
	auth := fmt.Sprintf("Authorization: Bearer %s", bearer)
	stdout, stderr, err := oc.AsAdmin().Run("exec").Args(execPodName, "--", "curl", "-s", "-k", "-H", auth, url).Outputs()
	if err != nil {
		return "", fmt.Errorf("command failed: %v\nstderr: %s\nstdout:%s", exutil.RedactBearerToken(err.Error()), exutil.RedactBearerToken(stderr), exutil.RedactBearerToken(stdout))
	}
	// Terminate stdout with a newline to avoid an unexpected end of stream error.
	if len(stdout) > 0 {
		stdout = stdout + "\n"
	}
	return stdout, err
}
