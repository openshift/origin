package controller

import (
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

var templateInstancesTotal = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "openshift_template_instance_total",
		Help: "Counts TemplateInstance objects",
	},
	nil,
)

var templateInstanceStatusCondition = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "openshift_template_instance_status_condition_total",
		Help: "Counts TemplateInstance objects by condition type and status",
	},
	[]string{"type", "status"},
)

var templateInstancesActiveStartTime = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "openshift_template_instance_active_start_time_seconds",
		Help: "Show the start time in unix epoch form of active TemplateInstance objects by namespace and name",
	},
	[]string{"namespace", "name"},
)

func (c *TemplateInstanceController) Describe(ch chan<- *prometheus.Desc) {
	templateInstancesTotal.Describe(ch)
	templateInstanceStatusCondition.Describe(ch)
	templateInstancesActiveStartTime.Describe(ch)
}

func (c *TemplateInstanceController) Collect(ch chan<- prometheus.Metric) {
	templateInstances, err := c.lister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	templateInstancesTotal.Reset()
	templateInstanceStatusCondition.Reset()
	templateInstancesActiveStartTime.Reset()

	templateInstancesTotal.WithLabelValues().Set(0)

	for _, templateInstance := range templateInstances {
		waiting := true

		templateInstancesTotal.WithLabelValues().Inc()

		for _, cond := range templateInstance.Status.Conditions {
			templateInstanceStatusCondition.WithLabelValues(string(cond.Type), string(cond.Status)).Inc()

			if cond.Status == kapi.ConditionTrue &&
				(cond.Type == templateapi.TemplateInstanceInstantiateFailure || cond.Type == templateapi.TemplateInstanceReady) {
				waiting = false
			}
		}

		if waiting {
			templateInstancesActiveStartTime.WithLabelValues(templateInstance.Namespace, templateInstance.Name).Set(float64(templateInstance.CreationTimestamp.Unix()))
		}
	}

	templateInstancesTotal.Collect(ch)
	templateInstanceStatusCondition.Collect(ch)
	templateInstancesActiveStartTime.Collect(ch)
}
