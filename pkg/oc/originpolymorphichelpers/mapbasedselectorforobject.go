package originpolymorphichelpers

import (
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"
)

func NewMapBasedSelectorForObjectFn(delegate polymorphichelpers.MapBasedSelectorForObjectFunc) polymorphichelpers.MapBasedSelectorForObjectFunc {
	return func(object runtime.Object) (string, error) {
		switch t := object.(type) {
		case *appsapi.DeploymentConfig:
			return kubectl.MakeLabels(t.Spec.Selector), nil

		default:
			return delegate(object)
		}
	}
}
