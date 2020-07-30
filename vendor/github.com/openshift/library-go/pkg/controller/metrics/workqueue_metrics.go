package metrics

import (
	"k8s.io/client-go/util/workqueue"
	k8smetrics "k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"

	"github.com/prometheus/client_golang/prometheus"
)

// Package prometheus sets the workqueue DefaultMetricsFactory to produce
// prometheus metrics. To use this package, you just have to import it.

func init() {
	workqueue.SetProvider(prometheusMetricsProvider{})
}

// Package prometheus sets the workqueue DefaultMetricsFactory to produce
// prometheus metrics. To use this package, you just have to import it.

// Metrics subsystem and keys used by the workqueue.
const (
	WorkQueueSubsystem         = "workqueue"
	DepthKey                   = "depth"
	AddsKey                    = "adds_total"
	QueueLatencyKey            = "queue_duration_seconds"
	WorkDurationKey            = "work_duration_seconds"
	UnfinishedWorkKey          = "unfinished_work_seconds"
	LongestRunningProcessorKey = "longest_running_processor_seconds"
	RetriesKey                 = "retries_total"
)

func init() {
	workqueue.SetProvider(prometheusMetricsProvider{})
}

type prometheusMetricsProvider struct{}

func (prometheusMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	depth := k8smetrics.NewGauge(&k8smetrics.GaugeOpts{
		Subsystem:   WorkQueueSubsystem,
		Name:        DepthKey,
		Help:        "Current depth of workqueue",
		ConstLabels: prometheus.Labels{"name": name},
	})
	legacyregistry.Register(depth)
	return depth
}

func (prometheusMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	adds := k8smetrics.NewCounter(&k8smetrics.CounterOpts{
		Subsystem:   WorkQueueSubsystem,
		Name:        AddsKey,
		Help:        "Total number of adds handled by workqueue",
		ConstLabels: prometheus.Labels{"name": name},
	})
	legacyregistry.Register(adds)
	return adds
}

func (prometheusMetricsProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	latency := k8smetrics.NewHistogram(&k8smetrics.HistogramOpts{
		Subsystem:   WorkQueueSubsystem,
		Name:        QueueLatencyKey,
		Help:        "How long in seconds an item stays in workqueue before being requested.",
		ConstLabels: prometheus.Labels{"name": name},
		Buckets:     prometheus.ExponentialBuckets(10e-9, 10, 10),
	})
	legacyregistry.Register(latency)
	return latency
}

func (prometheusMetricsProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	workDuration := k8smetrics.NewHistogram(&k8smetrics.HistogramOpts{
		Subsystem:   WorkQueueSubsystem,
		Name:        WorkDurationKey,
		Help:        "How long in seconds processing an item from workqueue takes.",
		ConstLabels: prometheus.Labels{"name": name},
		Buckets:     prometheus.ExponentialBuckets(10e-9, 10, 10),
	})
	legacyregistry.Register(workDuration)
	return workDuration
}

func (prometheusMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	unfinished := k8smetrics.NewGauge(&k8smetrics.GaugeOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      UnfinishedWorkKey,
		Help: "How many seconds of work has done that " +
			"is in progress and hasn't been observed by work_duration. Large " +
			"values indicate stuck threads. One can deduce the number of stuck " +
			"threads by observing the rate at which this increases.",
		ConstLabels: prometheus.Labels{"name": name},
	})
	legacyregistry.Register(unfinished)
	return unfinished
}

func (prometheusMetricsProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	unfinished := k8smetrics.NewGauge(&k8smetrics.GaugeOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      LongestRunningProcessorKey,
		Help: "How many seconds has the longest running " +
			"processor for workqueue been running.",
		ConstLabels: prometheus.Labels{"name": name},
	})
	legacyregistry.Register(unfinished)
	return unfinished
}

func (prometheusMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	retries := k8smetrics.NewCounter(&k8smetrics.CounterOpts{
		Subsystem:   WorkQueueSubsystem,
		Name:        RetriesKey,
		Help:        "Total number of retries handled by workqueue",
		ConstLabels: prometheus.Labels{"name": name},
	})
	legacyregistry.Register(retries)
	return retries
}

// TODO(danielqsj): Remove the following metrics, they are deprecated
func (prometheusMetricsProvider) NewDeprecatedDepthMetric(name string) workqueue.GaugeMetric {
	depth := k8smetrics.NewGauge(&k8smetrics.GaugeOpts{
		Subsystem: name,
		Name:      "depth",
		Help:      "(Deprecated) Current depth of workqueue: " + name,
	})
	if err := legacyregistry.Register(depth); err != nil {
		klog.Errorf("failed to register depth metric %v: %v", name, err)
	}
	return depth
}

func (prometheusMetricsProvider) NewDeprecatedAddsMetric(name string) workqueue.CounterMetric {
	adds := k8smetrics.NewCounter(&k8smetrics.CounterOpts{
		Subsystem: name,
		Name:      "adds",
		Help:      "(Deprecated) Total number of adds handled by workqueue: " + name,
	})
	if err := legacyregistry.Register(adds); err != nil {
		klog.Errorf("failed to register adds metric %v: %v", name, err)
	}
	return adds
}

func (prometheusMetricsProvider) NewDeprecatedLatencyMetric(name string) workqueue.SummaryMetric {
	latency := k8smetrics.NewSummary(&k8smetrics.SummaryOpts{
		Subsystem: name,
		Name:      "queue_latency",
		Help:      "(Deprecated) How long an item stays in workqueue" + name + " before being requested.",
	})
	if err := legacyregistry.Register(latency); err != nil {
		klog.Errorf("failed to register latency metric %v: %v", name, err)
	}
	return latency
}

func (prometheusMetricsProvider) NewDeprecatedWorkDurationMetric(name string) workqueue.SummaryMetric {
	workDuration := k8smetrics.NewSummary(&k8smetrics.SummaryOpts{
		Subsystem: name,
		Name:      "work_duration",
		Help:      "(Deprecated) How long processing an item from workqueue" + name + " takes.",
	})
	if err := legacyregistry.Register(workDuration); err != nil {
		klog.Errorf("failed to register work_duration metric %v: %v", name, err)
	}
	return workDuration
}

func (prometheusMetricsProvider) NewDeprecatedUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	unfinished := k8smetrics.NewGauge(&k8smetrics.GaugeOpts{
		Subsystem: name,
		Name:      "unfinished_work_seconds",
		Help: "(Deprecated) How many seconds of work " + name + " has done that " +
			"is in progress and hasn't been observed by work_duration. Large " +
			"values indicate stuck threads. One can deduce the number of stuck " +
			"threads by observing the rate at which this increases.",
	})
	if err := legacyregistry.Register(unfinished); err != nil {
		klog.Errorf("failed to register unfinished_work_seconds metric %v: %v", name, err)
	}
	return unfinished
}

func (prometheusMetricsProvider) NewDeprecatedLongestRunningProcessorMicrosecondsMetric(name string) workqueue.SettableGaugeMetric {
	unfinished := k8smetrics.NewGauge(&k8smetrics.GaugeOpts{
		Subsystem: name,
		Name:      "longest_running_processor_microseconds",
		Help: "(Deprecated) How many microseconds has the longest running " +
			"processor for " + name + " been running.",
	})
	if err := legacyregistry.Register(unfinished); err != nil {
		klog.Errorf("failed to register longest_running_processor_microseconds metric %v: %v", name, err)
	}
	return unfinished
}

func (prometheusMetricsProvider) NewDeprecatedRetriesMetric(name string) workqueue.CounterMetric {
	retries := k8smetrics.NewCounter(&k8smetrics.CounterOpts{
		Subsystem: name,
		Name:      "retries",
		Help:      "(Deprecated) Total number of retries handled by workqueue: " + name,
	})
	if err := legacyregistry.Register(retries); err != nil {
		klog.Errorf("failed to register retries metric %v: %v", name, err)
	}
	return retries
}
