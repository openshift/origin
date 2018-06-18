package originpolymorphichelpers

import (
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func NewObjectPauserFn(delegate polymorphichelpers.ObjectPauserFunc) polymorphichelpers.ObjectPauserFunc {
	return func(obj runtime.Object) ([]byte, error) {
		switch t := obj.(type) {
		case *appsapi.DeploymentConfig:
			if t.Spec.Paused {
				return nil, errors.New("is already paused")
			}
			t.Spec.Paused = true
			// TODO: Pause the deployer containers.
			return runtime.Encode(internalVersionJSONEncoder(), obj)

		default:
			return delegate(obj)
		}
	}
}

func internalVersionJSONEncoder() runtime.Encoder {
	encoder := legacyscheme.Codecs.LegacyCodec(legacyscheme.Scheme.PrioritizedVersionsAllGroups()...)
	return unstructured.JSONFallbackEncoder{Encoder: encoder}
}
