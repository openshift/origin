package metrics

import (
	"net/url"
	"time"

	"github.com/blang/semver"
	k8smetrics "k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// requestLatency is a Prometheus Summary metric type partitioned by
	// "verb" and "url" labels. It is used for the rest client latency metrics.
	requestLatency = k8smetrics.NewHistogramVec(
		&k8smetrics.HistogramOpts{
			Name:    "rest_client_request_latency_seconds",
			Help:    "Request latency in seconds. Broken down by verb and URL.",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"verb", "url"},
	)

	requestResult = k8smetrics.NewCounterVec(
		&k8smetrics.CounterOpts{
			Name: "rest_client_requests_total",
			Help: "Number of HTTP requests, partitioned by status code, method, and host.",
		},
		[]string{"code", "method", "host"},
	)
)

func init() {
	legacyregistry.MustRegister(requestLatency)
	legacyregistry.MustRegister(requestResult)

	legacyregistry.Register(&latencyAdapter{requestLatency})
	legacyregistry.Register(&resultAdapter{requestResult})
}

type latencyAdapter struct {
	m *k8smetrics.HistogramVec
}

func (l *latencyAdapter) Describe(c chan<- *prometheus.Desc) {
	l.m.Describe(c)
}

func (l *latencyAdapter) Collect(c chan<- prometheus.Metric) {
	l.m.Collect(c)
}

func (l *latencyAdapter) Create(version *semver.Version) bool {
	return l.m.Create(version)
}

func (l *latencyAdapter) Observe(verb string, u url.URL, latency time.Duration) {
	l.m.WithLabelValues(verb, u.String()).Observe(latency.Seconds())
}

func (l *latencyAdapter) ClearState() {
	l.m.Reset()
}

func (l *latencyAdapter) FQName() string {
	return l.m.FQName()
}

type resultAdapter struct {
	m *k8smetrics.CounterVec
}

func (r *resultAdapter) Describe(c chan<- *prometheus.Desc) {
	r.m.Describe(c)
}

func (r *resultAdapter) Collect(c chan<- prometheus.Metric) {
	r.m.Collect(c)
}

func (r *resultAdapter) Create(version *semver.Version) bool {
	return r.m.Create(version)
}

func (r *resultAdapter) Increment(code, method, host string) {
	r.m.WithLabelValues(code, method, host).Inc()
}

func (r *resultAdapter) ClearState() {
	r.m.Reset()
}

func (r *resultAdapter) FQName() string {
	return r.m.FQName()
}
