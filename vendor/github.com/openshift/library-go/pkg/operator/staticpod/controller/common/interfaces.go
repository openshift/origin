package common

import (
	"k8s.io/client-go/tools/cache"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
)

type OperatorClient interface {
	Informer() cache.SharedIndexInformer
	Get() (*operatorv1alpha1.OperatorSpec, *operatorv1alpha1.StaticPodOperatorStatus, string, error)
	UpdateStatus(string, *operatorv1alpha1.StaticPodOperatorStatus) (*operatorv1alpha1.StaticPodOperatorStatus, error)
}
