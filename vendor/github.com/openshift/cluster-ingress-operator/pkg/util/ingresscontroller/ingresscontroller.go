package ingresscontroller

import (
	operatorv1 "github.com/openshift/api/operator/v1"
)

const (
	// ingressControllerAdmittedConditionType is the type for the
	// IngressController status condition that indicates that the operator
	// has validated and admitted the IngressController.
	//
	// TODO: Move the definition in this package and the one in the
	// "github.com/openshift/cluster-ingress-operator/pkg/operator/controller/ingress"
	// package into a third package that the first two can import.
	ingressControllerAdmittedConditionType = "Admitted"
)

// IsAdmitted returns a Boolean value indicating whether the given
// ingresscontroller has been admitted, as indicated by its "Admitted" status
// condition.
func IsAdmitted(ic *operatorv1.IngressController) bool {
	for _, cond := range ic.Status.Conditions {
		if cond.Type == ingressControllerAdmittedConditionType && cond.Status == operatorv1.ConditionTrue {
			return true
		}
	}
	return false
}
