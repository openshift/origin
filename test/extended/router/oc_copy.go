package router

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

func podConditionStatus(pod *corev1.Pod, t corev1.PodConditionType) corev1.ConditionStatus {
	for _, condition := range pod.Status.Conditions {
		if t != condition.Type {
			continue
		}
		return condition.Status
	}
	return corev1.ConditionUnknown
}
