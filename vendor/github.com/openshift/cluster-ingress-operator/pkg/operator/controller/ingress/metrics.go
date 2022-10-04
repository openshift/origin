package ingress

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/cluster-ingress-operator/pkg/manifests"

	operatorv1 "github.com/openshift/api/operator/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	// ingressControllerConditions reports the status conditions of each
	// IngressController using the ingress_controller_conditions metric.
	ingressControllerConditions = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ingress_controller_conditions",
		Help: "Report the conditions for ingress controllers. 0 is False and 1 is True.",
	}, []string{"name", "condition"})

	activeNLBs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ingress_controller_aws_nlb_active",
		Help: "Report the number of active NLBs on AWS clusters.",
	}, []string{"name"})

	// metricsList is a list of metrics for this package.
	metricsList = []prometheus.Collector{
		ingressControllerConditions,
		activeNLBs,
	}
)

// reportedConditions is the set of ingresscontroller status conditions that are
// reported in the ingress_controller_conditions metric.
var reportedConditions = sets.NewString("Available", "Degraded")

// SetIngressControllerConditionsMetric updates the
// ingress_controller_conditions metric values for the given IngressController.
func SetIngressControllerConditionsMetric(ic *operatorv1.IngressController) {
	for _, c := range ic.Status.Conditions {
		if !reportedConditions.Has(c.Type) {
			continue
		}
		switch c.Status {
		case operatorv1.ConditionFalse, operatorv1.ConditionTrue:
		default:
			log.V(4).Info("skipping metrics for IngressController condition because it is neither True nor False", "ingresscontroller", ic.Name, "condition_type", c.Type, "condition_status", c.Status)
			continue
		}
		var v float64 = 0
		if c.Status == operatorv1.ConditionTrue {
			v = 1
		}
		ingressControllerConditions.WithLabelValues(ic.Name, string(c.Type)).Set(v)
	}
}

// DeleteIngressControllerConditionsMetric deletes ingress_controller_conditions metrics which belong to the given ingresscontroller
func DeleteIngressControllerConditionsMetric(ic *operatorv1.IngressController) {
	for _, c := range ic.Status.Conditions {
		if !reportedConditions.Has(c.Type) {
			continue
		}
		deleted := ingressControllerConditions.DeleteLabelValues(ic.Name, string(c.Type))
		log.V(4).Info("deleted metric for IngressController that is being deleted", "ingresscontroller", ic.Name, "condition_type", c.Type, "deleted", deleted)
	}
}

func DeleteActiveNLBMetrics(ic *operatorv1.IngressController) {
	activeNLBs.DeleteLabelValues(ic.Name)
}

func SetIngressControllerNLBMetric(ci *operatorv1.IngressController) {
	labelVal := 0
	if ci.Status.EndpointPublishingStrategy != nil &&
		ci.Status.EndpointPublishingStrategy.LoadBalancer != nil &&
		ci.Status.EndpointPublishingStrategy.LoadBalancer.ProviderParameters != nil &&
		ci.Status.EndpointPublishingStrategy.LoadBalancer.ProviderParameters.Type == operatorv1.AWSLoadBalancerProvider &&
		ci.Status.EndpointPublishingStrategy.LoadBalancer.ProviderParameters.AWS != nil &&
		ci.Status.EndpointPublishingStrategy.LoadBalancer.ProviderParameters.AWS.Type == operatorv1.AWSNetworkLoadBalancer {
		labelVal = 1
	}
	activeNLBs.WithLabelValues(ci.Name).Set(float64(labelVal))
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

// ensureMetricsIntegration ensures that router prometheus metrics is integrated with openshift-monitoring for the given ingresscontroller.
func (r *reconciler) ensureMetricsIntegration(ci *operatorv1.IngressController, svc *corev1.Service, deploymentRef metav1.OwnerReference) error {
	statsSecret := manifests.RouterStatsSecret(ci)
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: statsSecret.Namespace, Name: statsSecret.Name}, statsSecret); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get router stats secret %s/%s, %v", statsSecret.Namespace, statsSecret.Name, err)
		}

		statsSecret.SetOwnerReferences([]metav1.OwnerReference{deploymentRef})
		if err := r.client.Create(context.TODO(), statsSecret); err != nil {
			return fmt.Errorf("failed to create router stats secret %s/%s: %v", statsSecret.Namespace, statsSecret.Name, err)
		}
		log.Info("created router stats secret", "namespace", statsSecret.Namespace, "name", statsSecret.Name)
	}

	cr := manifests.MetricsClusterRole()
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: cr.Name}, cr); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get router metrics cluster role %s: %v", cr.Name, err)
		}
		if err := r.client.Create(context.TODO(), cr); err != nil {
			return fmt.Errorf("failed to create router metrics cluster role %s: %v", cr.Name, err)
		}
		log.Info("created router metrics cluster role", "name", cr.Name)
	}

	crb := manifests.MetricsClusterRoleBinding()
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: crb.Name}, crb); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get router metrics cluster role binding %s: %v", crb.Name, err)
		}
		if err := r.client.Create(context.TODO(), crb); err != nil {
			return fmt.Errorf("failed to create router metrics cluster role binding %s: %v", crb.Name, err)
		}
		log.Info("created router metrics cluster role binding", "name", crb.Name)
	}

	mr := manifests.MetricsRole()
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: mr.Namespace, Name: mr.Name}, mr); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get router metrics role %s: %v", mr.Name, err)
		}
		if err := r.client.Create(context.TODO(), mr); err != nil {
			return fmt.Errorf("failed to create router metrics role %s: %v", mr.Name, err)
		}
		log.Info("created router metrics role", "name", mr.Name)
	}

	mrb := manifests.MetricsRoleBinding()
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: mrb.Namespace, Name: mrb.Name}, mrb); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get router metrics role binding %s: %v", mrb.Name, err)
		}
		if err := r.client.Create(context.TODO(), mrb); err != nil {
			return fmt.Errorf("failed to create router metrics role binding %s: %v", mrb.Name, err)
		}
		log.Info("created router metrics role binding", "name", mrb.Name)
	}

	if _, _, err := r.ensureServiceMonitor(ci, svc, deploymentRef); err != nil {
		return fmt.Errorf("failed to ensure servicemonitor for %s: %v", ci.Name, err)
	}

	return nil
}
