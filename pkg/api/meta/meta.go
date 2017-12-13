package meta

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
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
// such mutator is defined. Only references that are different between obj and old will
// be returned unless old is nil.
func GetImageReferenceMutator(obj, old runtime.Object) (ImageReferenceMutator, error) {
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
	default:
		if spec, path, err := GetPodSpec(obj); err == nil {
			if old == nil {
				return &podSpecMutator{spec: spec, path: path}, nil
			}
			oldSpec, _, err := GetPodSpec(old)
			if err != nil {
				return nil, fmt.Errorf("old and new pod spec objects were not of the same type %T != %T: %v", obj, old, err)
			}
			return &podSpecMutator{spec: spec, oldSpec: oldSpec, path: path}, nil
		}
		if spec, path, err := GetPodSpecV1(obj); err == nil {
			if old == nil {
				return &podSpecV1Mutator{spec: spec, path: path}, nil
			}
			oldSpec, _, err := GetPodSpecV1(old)
			if err != nil {
				return nil, fmt.Errorf("old and new pod spec objects were not of the same type %T != %T: %v", obj, old, err)
			}
			return &podSpecV1Mutator{spec: spec, oldSpec: oldSpec, path: path}, nil
		}
		return nil, errNoImageMutator
	}
}

type pairwiseMutator struct {
	newer ImageReferenceMutator
	older ImageReferenceMutator
}

type AnnotationAccessor interface {
	// Annotations returns a map representing annotations. Not mutable.
	Annotations() map[string]string
	// SetAnnotations sets representing annotations onto the object.
	SetAnnotations(map[string]string)
	// TemplateAnnotations returns a map representing annotations on a nested template in the object. Not mutable.
	// If no template is present bool will be false.
	TemplateAnnotations() (map[string]string, bool)
	// SetTemplateAnnotations sets annotations on a nested template in the object.
	// If no template is present bool will be false.
	SetTemplateAnnotations(map[string]string) bool
}

type annotationsAccessor struct {
	object   metav1.Object
	template metav1.Object
}

func (a annotationsAccessor) Annotations() map[string]string {
	return a.object.GetAnnotations()
}

func (a annotationsAccessor) TemplateAnnotations() (map[string]string, bool) {
	if a.template == nil {
		return nil, false
	}
	return a.template.GetAnnotations(), true
}

func (a annotationsAccessor) SetAnnotations(annotations map[string]string) {
	a.object.SetAnnotations(annotations)
}

func (a annotationsAccessor) SetTemplateAnnotations(annotations map[string]string) bool {
	if a.template == nil {
		return false
	}
	a.template.SetAnnotations(annotations)
	return true
}

// GetAnnotationAccessor returns an accessor for the provided object or false if the object
// does not support accessing annotations.
func GetAnnotationAccessor(obj runtime.Object) (AnnotationAccessor, bool) {
	switch t := obj.(type) {
	case metav1.Object:
		templateObject, _ := GetTemplateMetaObject(obj)
		return annotationsAccessor{object: t, template: templateObject}, true
	default:
		return nil, false
	}
}
