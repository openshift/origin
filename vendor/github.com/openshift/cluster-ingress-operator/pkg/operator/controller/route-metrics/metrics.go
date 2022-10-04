package routemetrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// routeMetricsControllerRoutesPerShard reports the number of routes belonging to each
	// Shard (IngressController) using the route_metrics_controller_routes_per_shard metric.
	routeMetricsControllerRoutesPerShard = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "route_metrics_controller_routes_per_shard",
		Help: "Report the number of routes for shards (ingress controllers).",
	}, []string{"shard_name"})

	// metricsList is a list of metrics for this package.
	metricsList = []prometheus.Collector{
		routeMetricsControllerRoutesPerShard,
	}
)

func SetRouteMetricsControllerRoutesPerShardMetric(shardName string, value float64) {
	routeMetricsControllerRoutesPerShard.WithLabelValues(shardName).Set(value)
}

func DeleteRouteMetricsControllerRoutesPerShardMetric(shardName string) {
	routeMetricsControllerRoutesPerShard.DeleteLabelValues(shardName)
}

// RegisterMetrics calls prometheus.Register on each metric in metricsList, and
// returns on errors.
func RegisterMetrics() error {
	for _, metric := range metricsList {
		if err := prometheus.Register(metric); err != nil {
			return err
		}
	}
	return nil
}
