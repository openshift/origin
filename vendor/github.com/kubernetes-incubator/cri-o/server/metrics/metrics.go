package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// CRIOOperationsKey is the key for CRI-O operation metrics.
	CRIOOperationsKey = "crio_operations"
	// CRIOOperationsLatencyKey is the key for the operation latency metrics.
	CRIOOperationsLatencyKey = "crio_operations_latency_microseconds"
	// CRIOOperationsErrorsKey is the key for the operation error metrics.
	CRIOOperationsErrorsKey = "crio_operations_errors"

	// TODO(runcom):
	// timeouts

	subsystem = "container_runtime"
)

var (
	// CRIOOperations collects operation counts by operation type.
	CRIOOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      CRIOOperationsKey,
			Help:      "Cumulative number of CRI-O operations by operation type.",
		},
		[]string{"operation_type"},
	)
	// CRIOOperationsLatency collects operation latency numbers by operation
	// type.
	CRIOOperationsLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Subsystem: subsystem,
			Name:      CRIOOperationsLatencyKey,
			Help:      "Latency in microseconds of CRI-O operations. Broken down by operation type.",
		},
		[]string{"operation_type"},
	)
	// CRIOOperationsErrors collects operation errors by operation
	// type.
	CRIOOperationsErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      CRIOOperationsErrorsKey,
			Help:      "Cumulative number of CRI-O operation errors by operation type.",
		},
		[]string{"operation_type"},
	)
)

var registerMetrics sync.Once

// Register all metrics
func Register() {
	registerMetrics.Do(func() {
		prometheus.MustRegister(CRIOOperations)
		prometheus.MustRegister(CRIOOperationsLatency)
		prometheus.MustRegister(CRIOOperationsErrors)
	})
}

// SinceInMicroseconds gets the time since the specified start in microseconds.
func SinceInMicroseconds(start time.Time) float64 {
	return float64(time.Since(start).Nanoseconds() / time.Microsecond.Nanoseconds())
}
