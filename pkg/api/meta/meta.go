package meta

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// ImageReferenceMutateFunc is passed a reference representing an image, and may alter
// the Name, Kind, and Namespace fields of the reference. If an error is returned the
// object may still be mutated under the covers.
type ImageReferenceMutateFunc func(ref *kapi.ObjectReference) error

type ImageReferenceMutator interface {
	// Mutate invokes fn on every image reference in the object. If fn returns an error,
	// a field.Error is added to the list to be returned. Mutate does not terminate early
	// if errors are detected.
	Mutate(fn ImageReferenceMutateFunc) field.ErrorList
}

var errNoImageMutator = fmt.Errorf("No list of images available for this object")

// GetImageReferenceMutator returns a mutator for the provided object, or an error if no
// such mutator is defined.
func GetImageReferenceMutator(obj runtime.Object) (ImageReferenceMutator, error) {
	switch t := obj.(type) {
	case *buildapi.Build:
		return &buildSpecMutator{spec: &t.Spec.CommonSpec, path: field.NewPath("spec")}, nil
	case *buildapi.BuildConfig:
		return &buildSpecMutator{spec: &t.Spec.CommonSpec, path: field.NewPath("spec")}, nil
	default:
		if spec, path, err := GetPodSpec(obj); err == nil {
			return &podSpecMutator{spec: spec, path: path}, nil
		}
		return nil, errNoImageMutator
	}
}
