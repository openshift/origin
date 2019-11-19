package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	v1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/kubernetes/pkg/client/conditions"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

const waitForPrometheusStartSeconds = 240

type prometheusResponse struct {
	Status string                 `json:"status"`
	Data   prometheusResponseData `json:"data"`
}

type prometheusResponseData struct {
	ResultType string       `json:"resultType"`
	Result     model.Vector `json:"result"`
}

func getBearerTokenURLViaPod(ns, execPodName, url, bearer string) (string, error) {
	cmd := fmt.Sprintf("curl -s -k -H 'Authorization: Bearer %s' %q", bearer, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func runQueries(promQueries map[string]bool, oc *exutil.CLI, ns, execPodName, baseURL, bearerToken string) {
	// expect all correct metrics within a reasonable time period
	errsMap := map[string]error{}
	for i := 0; i < waitForPrometheusStartSeconds; i++ {
		for query, expected := range promQueries {
			//TODO when the http/query apis discussed at https://github.com/prometheus/client_golang#client-for-the-prometheus-http-api
			// and introduced at https://github.com/prometheus/client_golang/blob/master/api/prometheus/v1/api.go are vendored into
			// openshift/origin, look to replace this homegrown http request / query param with that API
			g.By("perform prometheus metric query " + query)
			contents, err := getBearerTokenURLViaPod(ns, execPodName, fmt.Sprintf("%s/api/v1/query?%s", baseURL, (url.Values{"query": []string{query}}).Encode()), bearerToken)
			o.Expect(err).NotTo(o.HaveOccurred())
			result := prometheusResponse{}
			json.Unmarshal([]byte(contents), &result)
			metrics := result.Data.Result

			delete(errsMap, query) // clear out any prior failures
			if (len(metrics) > 0 && !expected) || (len(metrics) == 0 && expected) {
				dbg := fmt.Sprintf("promQL query: %s had reported incorrect results: %v", query, metrics)
				fmt.Fprintf(g.GinkgoWriter, dbg)
				errsMap[query] = fmt.Errorf(dbg)
			}

		}

		if len(errsMap) == 0 {
			break
		}
		time.Sleep(time.Second)
	}

	if len(errsMap) != 0 {
		exutil.DumpPodLogsStartingWith("prometheus-0", oc)
	}
	o.Expect(errsMap).To(o.BeEmpty())
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
