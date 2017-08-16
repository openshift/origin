package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	authSubsystem = "openshift_auth"
)

var (
	authCounterTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: authSubsystem,
			Name:      "auth_count_total",
			Help:      "Counts total authentication attempts",
		}, []string{},
	)
	authCounterResult = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: authSubsystem,
			Name:      "auth_count_result",
			Help:      "Counts authentication attempts by result",
		}, []string{"result"},
	)
)

func init() {
	prometheus.MustRegister(authCounterTotal)
	prometheus.MustRegister(authCounterResult)
}

func Record(result string) {
	authCounterTotal.WithLabelValues().Inc()
	authCounterResult.WithLabelValues(result).Inc()
}
