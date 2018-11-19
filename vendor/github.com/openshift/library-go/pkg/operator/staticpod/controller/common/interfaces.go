package common

import (
	"k8s.io/client-go/tools/cache"

	operatorv1 "github.com/openshift/api/operator/v1"
)

type OperatorClient interface {
	Informer() cache.SharedIndexInformer
	Get() (*operatorv1.OperatorSpec, *operatorv1.StaticPodOperatorStatus, string, error)
	UpdateStatus(string, *operatorv1.StaticPodOperatorStatus) (*operatorv1.StaticPodOperatorStatus, error)
}
