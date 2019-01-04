package v1helpers

import (
	operatorv1 "github.com/openshift/api/operator/v1"
)

type OperatorClient interface {
	GetOperatorState() (spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, resourceVersion string, err error)
	UpdateOperatorSpec(string, *operatorv1.OperatorSpec) (spec *operatorv1.OperatorSpec, resourceVersion string, err error)
	UpdateOperatorStatus(string, *operatorv1.OperatorStatus) (status *operatorv1.OperatorStatus, resourceVersion string, err error)
}
