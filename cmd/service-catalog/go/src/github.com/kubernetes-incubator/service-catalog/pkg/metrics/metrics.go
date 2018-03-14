/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package metrics creates and registers metrics objects with Prometheus
// and sets the Prometheus HTTP handler for /metrics
package metrics

import (
	"net/http"
	"sync"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registerMetrics sync.Once

const (
	catalogNamespace = "servicecatalog" // Prometheus namespace (nothing to do with k8s namespace)
)

var (
	// Metrics are identified in Prometheus by concatinating Namespace,
	// Subsystem and Name while omitting any nulls and separating each key with
	// an underscore.  Note that in this context, Namespace is the Prometheus
	// Namespace and there is no correlation with Kubernetes Namespace.

	// BrokerServiceClassCount exposes the number of Service Classes registered
	// per broker.
	BrokerServiceClassCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: catalogNamespace,
			Name:      "broker_service_class_count",
			Help:      "Number of services classes by Broker.",
		},
		[]string{"broker"},
	)

	// BrokerServicePlanCount exposes the number of Service Plans registered
	// per broker.
	BrokerServicePlanCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: catalogNamespace,
			Name:      "broker_service_plan_count",
			Help:      "Number of services classes by Broker.",
		},
		[]string{"broker"},
	)

	// OSBRequestCount exposes the number of HTTP requests made to Open Service
	// Brokers.  The metric is broken out by broker name and response status
	// group (1xx/2xx/3xx/4xx/5xx or 'client-error')
	OSBRequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: catalogNamespace,
			Name:      "osb_request_count",
			Help:      "Cumulative number of HTTP requests from the OSB Client to the specified Service Broker grouped by broker name, broker method, and response status.",
		},
		[]string{"broker", "method", "status"},
	)
)

func register(registry *prometheus.Registry) {
	registerMetrics.Do(func() {
		registry.MustRegister(BrokerServiceClassCount)
		registry.MustRegister(BrokerServicePlanCount)
		registry.MustRegister(OSBRequestCount)
	})
}

// RegisterMetricsAndInstallHandler registers the Service Catalog metrics
// objects with Prometheus and installs the Prometheus http handler at the
// default context.
func RegisterMetricsAndInstallHandler(m *http.ServeMux) {
	registry := prometheus.NewRegistry()
	register(registry)
	m.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError}))
	glog.V(4).Info("Registered /metrics with prometheus")
}
