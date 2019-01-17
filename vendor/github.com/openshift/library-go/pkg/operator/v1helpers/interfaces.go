package v1helpers

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/client-go/tools/cache"
)

type OperatorClient interface {
	Informer() cache.SharedIndexInformer
	GetOperatorState() (spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, resourceVersion string, err error)
	UpdateOperatorSpec(string, *operatorv1.OperatorSpec) (spec *operatorv1.OperatorSpec, resourceVersion string, err error)
	UpdateOperatorStatus(string, *operatorv1.OperatorStatus) (status *operatorv1.OperatorStatus, err error)
}

type StaticPodOperatorClient interface {
	OperatorClient
	GetStaticPodOperatorState() (*operatorv1.OperatorSpec, *operatorv1.StaticPodOperatorStatus, string, error)
	UpdateStaticPodOperatorStatus(string, *operatorv1.StaticPodOperatorStatus) (*operatorv1.StaticPodOperatorStatus, error)
}
