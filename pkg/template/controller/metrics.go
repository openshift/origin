package controller

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	templatev1 "github.com/openshift/api/template/v1"
)

var templateInstanceCompleted = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "openshift_template_instance_completed_total",
		Help: "Counts completed TemplateInstance objects by condition",
	},
	[]string{"condition"},
)

func newTemplateInstanceActiveAge() prometheus.Histogram {
	// We recreate a new Histogram object every time Collect is called.  This is
	// because we are recording a series of point-in-time observations about the
	// population of "active" TemplateInstances.  Were we to use a singleton
	// Histogram, we would only be able to observe TemplateInstances as they
	// completed, which would add latency in reporting very long-running
	// TemplateInstances and completely prevent reporting of non-completing
	// TemplateInstances.
	//
	// Effectively, the resulting series is to Histogram what Gauge is to
	// Counter.  In the resulting series, _count and _sum are not monotonically
	// increasing (because TemplateInstances are no longer part of the
	// population once they terminate or are deleted), therefore it is not valid
	// to use counter functions such as rate() on this series.

	return prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "openshift_template_instance_active_age_seconds",
			Help:    "Shows the instantaneous age distribution of active TemplateInstance objects",
			Buckets: prometheus.LinearBuckets(600, 600, 7),
		},
	)
}

func (c *TemplateInstanceController) Describe(ch chan<- *prometheus.Desc) {
	templateInstanceActiveAge := newTemplateInstanceActiveAge()

	templateInstanceCompleted.Describe(ch)
	templateInstanceActiveAge.Describe(ch)
}

func (c *TemplateInstanceController) Collect(ch chan<- prometheus.Metric) {
	templateInstanceCompleted.Collect(ch)

	now := c.clock.Now()

	templateInstances, err := c.lister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	templateInstanceActiveAge := newTemplateInstanceActiveAge()

nextTemplateInstance:
	for _, templateInstance := range templateInstances {
		for _, cond := range templateInstance.Status.Conditions {
			if cond.Status == corev1.ConditionTrue &&
				(cond.Type == templatev1.TemplateInstanceInstantiateFailure ||
					cond.Type == templatev1.TemplateInstanceReady) {
				continue nextTemplateInstance
			}
		}

		templateInstanceActiveAge.Observe(float64(now.Sub(templateInstance.CreationTimestamp.Time) / time.Second))
	}

	templateInstanceActiveAge.Collect(ch)
}
