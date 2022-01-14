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

const (
	apiserverReadVerbs  = "GET|LIST"
	apiserverWriteVerbs = "POST|PUT|PATCH|DELETE"
)

var _ = g.Describe("[sig-api-machinery][Late] API Server latency", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("apiserver-latency")

	// Latency thresholds defined below are based on the following CI jobs:
	// - https://prow.ci.openshift.org/view/gs/origin-ci-test/pr-logs/pull/26739/pull-ci-openshift-origin-master-e2e-gcp/1481905976666755072
	// - https://prow.ci.openshift.org/view/gs/origin-ci-test/pr-logs/pull/26739/pull-ci-openshift-origin-master-e2e-gcp/1481879797410828288
	// - https://prow.ci.openshift.org/view/gs/origin-ci-test/pr-logs/pull/26739/pull-ci-openshift-origin-master-e2e-aws-fips/1481879797377273856
	// - https://prow.ci.openshift.org/view/gs/origin-ci-test/pr-logs/pull/26739/pull-ci-openshift-origin-master-e2e-aws-fips/1481905976616423424
	// Latency thresholds should be calculed based on the following queries:
	// - histogram_quantile(0.99,sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",verb=~"GET|LIST"}[$testDuration])) by (le,scope))
	// - histogram_quantile(0.99,sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",verb=~"POST|PUT|PATCH|DELETE"}[$testDuration])) by (le,scope))
	g.It("should verify that the apiserver requests latency are within expected thresholds", func() {
		promClient := oc.NewPrometheusClient(context.TODO())

		// 99th percentile of resource-scoped requests latency
		scope := "resource|"
		expectRequestLatency(promClient, apiserverReadVerbs, scope, parseDuration("200ms"))    // got 130ms
		expectRequestLatency(promClient, apiserverWriteVerbs, scope, parseDuration("1s250ms")) // got 850ms

		// 99th percentile of namespace-scoped requests latency
		scope = "namespace"
		expectRequestLatency(promClient, apiserverReadVerbs, scope, parseDuration("150ms"))    // got 100ms
		expectRequestLatency(promClient, apiserverWriteVerbs, scope, parseDuration("1s350ms")) // got 900ms

		// 99th percentile of cluster-scoped requests latency
		scope = "cluster"
		expectRequestLatency(promClient, apiserverReadVerbs, scope, parseDuration("1s350ms")) // got 900ms
		expectRequestLatency(promClient, apiserverWriteVerbs, scope, parseDuration("300ms"))  // got 200ms
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
			`histogram_quantile(0.99, sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",verb=~"%s",scope=~"%s"}[%s])) by (le))`,
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
