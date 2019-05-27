package routedisplayhelpers

import (
	corev1 "k8s.io/api/core/v1"

	routev1 "github.com/openshift/api/route/v1"
)

func IngressConditionStatus(ingress *routev1.RouteIngress, t routev1.RouteIngressConditionType) (corev1.ConditionStatus, routev1.RouteIngressCondition) {
	for _, condition := range ingress.Conditions {
		if t != condition.Type {
			continue
		}
		return condition.Status, condition
	}
	return corev1.ConditionUnknown, routev1.RouteIngressCondition{}
}
