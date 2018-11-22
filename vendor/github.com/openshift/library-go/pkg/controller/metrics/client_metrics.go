package metrics

import (
	"net/url"
	"time"

	"k8s.io/client-go/tools/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// requestLatency is a Prometheus Summary metric type partitioned by
	// "verb" and "url" labels. It is used for the rest client latency metrics.
	requestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rest_client_request_latency_seconds",
			Help:    "Request latency in seconds. Broken down by verb and URL.",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"verb", "url"},
	)

	requestResult = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rest_client_requests_total",
			Help: "Number of HTTP requests, partitioned by status code, method, and host.",
		},
		[]string{"code", "method", "host"},
	)
)

func init() {
	prometheus.MustRegister(requestLatency)
	prometheus.MustRegister(requestResult)
	metrics.Register(&latencyAdapter{requestLatency}, &resultAdapter{requestResult})
}

type latencyAdapter struct {
	m *prometheus.HistogramVec
}

func (l *latencyAdapter) Observe(verb string, u url.URL, latency time.Duration) {
	l.m.WithLabelValues(verb, u.String()).Observe(latency.Seconds())
}

type resultAdapter struct {
	m *prometheus.CounterVec
}

func (r *resultAdapter) Increment(code, method, host string) {
	r.m.WithLabelValues(code, method, host).Inc()
}
