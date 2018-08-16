package routedisplayhelpers

import (
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

func IngressConditionStatus(ingress *routeapi.RouteIngress, t routeapi.RouteIngressConditionType) (kapi.ConditionStatus, routeapi.RouteIngressCondition) {
	for _, condition := range ingress.Conditions {
		if t != condition.Type {
			continue
		}
		return condition.Status, condition
	}
	return kapi.ConditionUnknown, routeapi.RouteIngressCondition{}
}
