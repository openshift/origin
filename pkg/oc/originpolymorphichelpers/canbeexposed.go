package originpolymorphichelpers

import (
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"
)

func NewCanBeExposedFn(delegate polymorphichelpers.CanBeExposedFunc) polymorphichelpers.CanBeExposedFunc {
	return func(kind schema.GroupKind) error {
		if appsapi.Kind("DeploymentConfig") == kind {
			return nil
		}
		return delegate(kind)
	}
}
