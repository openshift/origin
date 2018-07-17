package originpolymorphichelpers

import (
	oapps "github.com/openshift/api/apps"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"
)

func NewCanBeExposedFn(delegate polymorphichelpers.CanBeExposedFunc) polymorphichelpers.CanBeExposedFunc {
	return func(kind schema.GroupKind) error {
		if oapps.Kind("DeploymentConfig") == kind {
			return nil
		}
		return delegate(kind)
	}
}
