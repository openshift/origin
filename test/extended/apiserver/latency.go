package apiserver

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"

	testresult "github.com/openshift/origin/pkg/test/ginkgo/result"
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

	// Latency thresholds defined below are based on the following search.ci graphs:
	// - https://search.ci.openshift.org/graph/metrics?metric=cluster%3Aapi%3Aread%3Arequests%3Alatency%3Atotal%3Aavg&job=periodic-ci-openshift-release-master-ci-4.10-e2e-aws&job=periodic-ci-openshift-release-master-ci-4.10-e2e-azure&job=periodic-ci-openshift-release-master-ci-4.10-e2e-gcp&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-aws-upgrade&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-azure-upgrade&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-gcp-upgrade
	// - https://search.ci.openshift.org/graph/metrics?metric=cluster%3Aapi%3Aread%3Arequests%3Alatency%3Atotal%3Aquantile&job=periodic-ci-openshift-release-master-ci-4.10-e2e-aws&job=periodic-ci-openshift-release-master-ci-4.10-e2e-azure&job=periodic-ci-openshift-release-master-ci-4.10-e2e-gcp&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-aws-upgrade&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-azure-upgrade&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-gcp-upgrade
	// - https://search.ci.openshift.org/graph/metrics?metric=cluster%3Aapi%3Awrite%3Arequests%3Alatency%3Atotal%3Aavg&job=periodic-ci-openshift-release-master-ci-4.10-e2e-aws&job=periodic-ci-openshift-release-master-ci-4.10-e2e-azure&job=periodic-ci-openshift-release-master-ci-4.10-e2e-gcp&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-aws-upgrade&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-azure-upgrade&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-gcp-upgrade
	// - https://search.ci.openshift.org/graph/metrics?metric=cluster%3Aapi%3Awrite%3Arequests%3Alatency%3Atotal%3Aquantile&job=periodic-ci-openshift-release-master-ci-4.10-e2e-aws&job=periodic-ci-openshift-release-master-ci-4.10-e2e-azure&job=periodic-ci-openshift-release-master-ci-4.10-e2e-gcp&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-aws-upgrade&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-azure-upgrade&job=periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-gcp-upgrade
	// Latency thresholds should be calculed based on the following queries:
	// - histogram_quantile(0.99,sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",verb=~"GET|LIST"}[$testDuration])) by (le,scope))
	// - histogram_quantile(0.99,sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",verb=~"POST|PUT|PATCH|DELETE"}[$testDuration])) by (le,scope))
	g.It("should verify that the apiserver requests latency are within expected thresholds", func() {
		promClient := oc.NewPrometheusClient(context.TODO())

		// resource-scoped requests latency
		scope := "resource|"
		expectAverageRequestLatency(promClient, apiserverReadVerbs, scope, parseDuration("275ms"))         // got 200ms in gcp
		expectAverageRequestLatency(promClient, apiserverWriteVerbs, scope, parseDuration("120ms"))        // got 90ms in gcp
		expect99PercentileRequestLatency(promClient, apiserverReadVerbs, scope, parseDuration("1s"))       // got 750ms in azure
		expect99PercentileRequestLatency(promClient, apiserverWriteVerbs, scope, parseDuration("1s600ms")) // got 1s200ms in azure

		// namespace-scoped requests latency
		scope = "namespace"
		expectAverageRequestLatency(promClient, apiserverReadVerbs, scope, parseDuration("50ms"))          // got 35ms in azure
		expectAverageRequestLatency(promClient, apiserverWriteVerbs, scope, parseDuration("100ms"))        // got 75ms in azure
		expect99PercentileRequestLatency(promClient, apiserverReadVerbs, scope, parseDuration("1s200ms"))  // got 900ms in azure
		expect99PercentileRequestLatency(promClient, apiserverWriteVerbs, scope, parseDuration("1s300ms")) // got 1s in azure

		// cluster-scoped requests latency
		scope = "cluster"
		expectAverageRequestLatency(promClient, apiserverReadVerbs, scope, parseDuration("250ms"))         // got 180ms in azure
		expectAverageRequestLatency(promClient, apiserverWriteVerbs, scope, parseDuration("120ms"))        // got 90ms in azure
		expect99PercentileRequestLatency(promClient, apiserverReadVerbs, scope, parseDuration("6s"))       // got 4s in gcp
		expect99PercentileRequestLatency(promClient, apiserverWriteVerbs, scope, parseDuration("1s300ms")) // got 1s in gcp
	})
})

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	o.Expect(err).NotTo(o.HaveOccurred())
	return d
}

func expectAverageRequestLatency(prometheusClient prometheusv1.API, verb string, scope string, threshold time.Duration) {
	resp, err := promhelper.RunQuery(
		context.TODO(),
		prometheusClient,
		fmt.Sprintf(
			`sum(rate(apiserver_request_duration_seconds_sum{job="apiserver",verb=~"%s",scope=~"%s"}[%s])) / sum(rate(apiserver_request_duration_seconds_count{job="apiserver",verb=~"%s",scope=~"%s"}[%s]))`,
			verb, scope, exutil.DurationSinceStartInSeconds().String(), verb, scope, exutil.DurationSinceStartInSeconds(),
		),
	)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(resp.Data.Result).NotTo(o.BeEmpty())

	latency := time.Duration(float64(resp.Data.Result[0].Value) * float64(time.Second))
	if latency.Seconds() < threshold.Seconds() {
		// Flake to gather more insights on the impact of the test.
		testresult.Flakef("expected average apiserver request latency with verb=%q and scope=%q to be less than %s, got %s", verb, scope, threshold, latency)
	}
}

func expect99PercentileRequestLatency(prometheusClient prometheusv1.API, verb string, scope string, threshold time.Duration) {
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
	if latency.Seconds() > threshold.Seconds() {
		// Flake to gather more insights on the impact of the test.
		testresult.Flakef("expected 99th percentile of apiserver request latency with verb=%q and scope=%q to be less than %s, got %s", verb, scope, threshold, latency)
	}
}
