package originpolymorphichelpers

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appsv1 "github.com/openshift/api/apps/v1"
)

func NewObjectResumerFn(delegate polymorphichelpers.ObjectResumerFunc) polymorphichelpers.ObjectResumerFunc {
	return func(obj runtime.Object) ([]byte, error) {
		switch t := obj.(type) {
		case *appsv1.DeploymentConfig:
			if !t.Spec.Paused {
				return nil, errors.New("is not paused")
			}
			t.Spec.Paused = false
			return runtime.Encode(scheme.DefaultJSONEncoder(), obj)

		default:
			return delegate(obj)
		}
	}
}
