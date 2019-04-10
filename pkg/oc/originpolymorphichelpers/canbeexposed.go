package originpolymorphichelpers

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsv1 "github.com/openshift/api/apps/v1"
)

func NewCanBeExposedFn(delegate polymorphichelpers.CanBeExposedFunc) polymorphichelpers.CanBeExposedFunc {
	return func(kind schema.GroupKind) error {
		if appsv1.SchemeGroupVersion.WithKind("DeploymentConfig").GroupKind() == kind {
			return nil
		}
		return delegate(kind)
	}
}
