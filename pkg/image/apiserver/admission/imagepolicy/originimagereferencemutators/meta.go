package originimagereferencemutators

import (
	"fmt"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/imagepolicy/imagereferencemutators"
)

type OriginImageMutators struct {
	imagereferencemutators.KubeImageMutators
}

// GetImageReferenceMutator returns a mutator for the provided object, or an error if no
// such mutator is defined. Only references that are different between obj and old will
// be returned unless old is nil.
func (o OriginImageMutators) GetImageReferenceMutator(obj, old runtime.Object) (imagereferencemutators.ImageReferenceMutator, error) {
	switch t := obj.(type) {
	case *buildapi.Build:
		if oldT, ok := old.(*buildapi.Build); ok && oldT != nil {
			return &buildSpecMutator{spec: &t.Spec.CommonSpec, oldSpec: &oldT.Spec.CommonSpec, path: field.NewPath("spec")}, nil
		}
		return &buildSpecMutator{spec: &t.Spec.CommonSpec, path: field.NewPath("spec")}, nil
	case *buildapi.BuildConfig:
		if oldT, ok := old.(*buildapi.BuildConfig); ok && oldT != nil {
			return &buildSpecMutator{spec: &t.Spec.CommonSpec, oldSpec: &oldT.Spec.CommonSpec, path: field.NewPath("spec")}, nil
		}
		return &buildSpecMutator{spec: &t.Spec.CommonSpec, path: field.NewPath("spec")}, nil
	}
	if spec, path, err := getPodSpec(obj); err == nil {
		var oldSpec *kapi.PodSpec
		if old != nil {
			oldSpec, _, err = getPodSpec(old)
			if err != nil {
				return nil, fmt.Errorf("old and new pod spec objects were not of the same type %T != %T: %v", obj, old, err)
			}
		}
		return imagereferencemutators.NewPodSpecMutator(spec, oldSpec, path), nil
	}
	if spec, path, err := getPodSpecV1(obj); err == nil {
		var oldSpec *kapiv1.PodSpec
		if old != nil {
			oldSpec, _, err = getPodSpecV1(old)
			if err != nil {
				return nil, fmt.Errorf("old and new pod spec objects were not of the same type %T != %T: %v", obj, old, err)
			}
		}
		return imagereferencemutators.NewPodSpecV1Mutator(spec, oldSpec, path), nil
	}
	return o.KubeImageMutators.GetImageReferenceMutator(obj, old)
}
