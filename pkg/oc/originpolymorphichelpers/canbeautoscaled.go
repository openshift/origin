package originpolymorphichelpers

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func NewCanBeAutoscaledFn(delegate polymorphichelpers.CanBeAutoscaledFunc) polymorphichelpers.CanBeAutoscaledFunc {
	return func(kind schema.GroupKind) error {
		if appsapi.Kind("DeploymentConfig") == kind {
			return nil
		}
		return delegate(kind)
	}
}
