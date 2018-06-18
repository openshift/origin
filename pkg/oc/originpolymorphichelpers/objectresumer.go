package originpolymorphichelpers

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func NewObjectResumerFn(delegate polymorphichelpers.ObjectResumerFunc) polymorphichelpers.ObjectResumerFunc {
	return func(obj runtime.Object) ([]byte, error) {
		switch t := obj.(type) {
		case *appsapi.DeploymentConfig:
			if !t.Spec.Paused {
				return nil, errors.New("is not paused")
			}
			t.Spec.Paused = false
			// TODO: Resume the deployer containers.
			return runtime.Encode(internalVersionJSONEncoder(), obj)

		default:
			return delegate(obj)
		}
	}
}
