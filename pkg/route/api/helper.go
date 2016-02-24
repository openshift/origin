package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// IngressConditionStatus returns the first status and condition matching the provided ingress condition type. Conditions
// prefer the first matching entry and clients are allowed to ignore later conditions of the same type.
func IngressConditionStatus(ingress *RouteIngress, t RouteIngressConditionType) (kapi.ConditionStatus, RouteIngressCondition) {
	for _, condition := range ingress.Conditions {
		if t != condition.Type {
			continue
		}
		return condition.Status, condition
	}
	return kapi.ConditionUnknown, RouteIngressCondition{}
}
