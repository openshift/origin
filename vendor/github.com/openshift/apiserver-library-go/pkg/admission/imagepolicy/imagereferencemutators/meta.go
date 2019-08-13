package imagereferencemutators

import (
	"fmt"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

type KubeImageMutators struct{}

var errNoImageMutator = fmt.Errorf("No list of images available for this object")

// GetImageReferenceMutator returns a mutator for the provided object, or an error if no
// such mutator is defined. Only references that are different between obj and old will
// be returned unless old is nil.
func (KubeImageMutators) GetImageReferenceMutator(obj, old runtime.Object) (ImageReferenceMutator, error) {
	if spec, path, err := GetPodSpec(obj); err == nil {
		var oldSpec *kapi.PodSpec
		if old != nil {
			oldSpec, _, err = GetPodSpec(old)
			if err != nil {
				return nil, fmt.Errorf("old and new pod spec objects were not of the same type %T != %T: %v", obj, old, err)
			}
		}
		return NewPodSpecMutator(spec, oldSpec, path), nil
	}
	if spec, path, err := GetPodSpecV1(obj); err == nil {
		var oldSpec *kapiv1.PodSpec
		if old != nil {
			oldSpec, _, err = GetPodSpecV1(old)
			if err != nil {
				return nil, fmt.Errorf("old and new pod spec objects were not of the same type %T != %T: %v", obj, old, err)
			}
		}
		return NewPodSpecV1Mutator(spec, oldSpec, path), nil
	}
	return nil, errNoImageMutator
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
func (KubeImageMutators) GetAnnotationAccessor(obj runtime.Object) (AnnotationAccessor, bool) {
	switch t := obj.(type) {
	case metav1.Object:
		templateObject, _ := GetTemplateMetaObject(obj)
		return annotationsAccessor{object: t, template: templateObject}, true
	default:
		return nil, false
	}
}
