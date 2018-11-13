package originpolymorphichelpers

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func NewMapBasedSelectorForObjectFn(delegate polymorphichelpers.MapBasedSelectorForObjectFunc) polymorphichelpers.MapBasedSelectorForObjectFunc {
	return func(object runtime.Object) (string, error) {
		switch t := object.(type) {
		case *appsapi.DeploymentConfig:
			return kubectl.MakeLabels(t.Spec.Selector), nil
		case *appsv1.DeploymentConfig:
			return kubectl.MakeLabels(t.Spec.Selector), nil

		default:
			return delegate(object)
		}
	}
}
