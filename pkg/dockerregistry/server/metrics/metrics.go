package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	registryNamespace = "openshift"
	registrySubsystem = "registry"
)

var (
	RegistryAPIRequests *prometheus.HistogramVec
	MasterAPIRequests   *prometheus.HistogramVec
)

// Register the metrics.
func Register() {
	RegistryAPIRequests = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: registryNamespace,
			Subsystem: registrySubsystem,
			Name:      "request_duration_seconds",
			Help:      "Request latency summary in microseconds for each operation",
		},
		[]string{"operation", "name"},
	)
	prometheus.MustRegister(RegistryAPIRequests)

	MasterAPIRequests = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: registryNamespace,
			Subsystem: registrySubsystem,
			Name:      "masterapi_request_duration_seconds",
			Help:      "Master API request latency summary in microseconds for each operation",
		},
		[]string{"operation"},
	)
	prometheus.MustRegister(MasterAPIRequests)
}

// NewTimer wraps the HistogramVec and used to track amount of time passed since the Timer was created.
func NewTimer(collector *prometheus.HistogramVec, labels []string) *metricTimer {
	return &metricTimer{
		collector: collector,
		labels:    labels,
		startTime: time.Now(),
	}
}

type metricTimer struct {
	collector *prometheus.HistogramVec
	labels    []string
	startTime time.Time
}

// Stop records the duration passed since the Timer was created with NewTimer.
func (m *metricTimer) Stop() {
	m.collector.WithLabelValues(m.labels...).Observe(float64(time.Since(m.startTime)) / float64(time.Second))
}
