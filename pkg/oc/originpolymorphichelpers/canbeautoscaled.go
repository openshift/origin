package originpolymorphichelpers

import (
	oapps "github.com/openshift/api/apps"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"
)

func NewCanBeAutoscaledFn(delegate polymorphichelpers.CanBeAutoscaledFunc) polymorphichelpers.CanBeAutoscaledFunc {
	return func(kind schema.GroupKind) error {
		if oapps.Kind("DeploymentConfig") == kind {
			return nil
		}
		return delegate(kind)
	}
}
