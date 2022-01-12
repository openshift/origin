package apiserver

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	promhelper "github.com/openshift/origin/test/extended/util/prometheus"
)

var _ = g.Describe("[sig-api-machinery][Late] API Server latency", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("apiserver-latency")

	// TODO: refine latency thresholds based on actual CI run results
	//
	// Latency thresholds should be calculed based on the following queries:
	// - histogram_quantile(0.99,sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",scope=~"resource|"}[$testDuration])) by (le,verb))
	// - histogram_quantile(0.99,sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",scope="namespace"}[$testDuration])) by (le,verb))
	// - histogram_quantile(0.99,sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",scope="cluster"}[$testDuration])) by (le,verb))
	g.It("should verify that the apiserver requests latency are within expected thresholds", func() {
		promClient := oc.NewPrometheusClient(context.TODO())

		// 99th percentile of resource-scoped requests latency
		scope := "resource|"
		expectRequestLatency(promClient, "GET", scope, parseDuration("100ms"))
		expectRequestLatency(promClient, "POST", scope, parseDuration("100ms"))
		expectRequestLatency(promClient, "PUT", scope, parseDuration("100ms"))
		expectRequestLatency(promClient, "PATCH", scope, parseDuration("100ms"))
		expectRequestLatency(promClient, "DELETE", scope, parseDuration("100ms"))

		// 99th percentile of namespace-scoped requests latency
		scope = "namespace"
		expectRequestLatency(promClient, "LIST", scope, parseDuration("500ms"))
		expectRequestLatency(promClient, "POST", scope, parseDuration("500ms"))
		expectRequestLatency(promClient, "DELETE", scope, parseDuration("500ms"))

		// 99th percentile of cluster-scoped requests latency
		scope = "cluster"
		expectRequestLatency(promClient, "LIST", scope, parseDuration("5s"))
		expectRequestLatency(promClient, "POST", scope, parseDuration("5s"))
	})
})

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	o.Expect(err).NotTo(o.HaveOccurred())
	return d
}

func expectRequestLatency(prometheusClient prometheusv1.API, verb string, scope string, threshold time.Duration) {
	resp, err := promhelper.RunQuery(
		context.TODO(),
		prometheusClient,
		fmt.Sprintf(
			`histogram_quantile(0.99, sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",verb="%s",scope=~"%s"}[%s])) by (le))`,
			verb, scope, exutil.DurationSinceStartInSeconds().String(),
		),
	)

	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(resp.Data.Result).NotTo(o.BeEmpty())

	latency := time.Duration(float64(resp.Data.Result[0].Value) * float64(time.Second))
	o.Expect(latency.Seconds()).Should(o.BeNumerically("<", threshold.Seconds()),
		fmt.Sprintf("expected 99th percentile of apiserver request latency with verb=%q and scope=%q to be less than %s, got %s", verb, scope, threshold, latency),
	)
}
