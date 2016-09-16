package imagestreammapping

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
)

// Strategy implements behavior for image stream mappings.
type Strategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator

	defaultRegistry api.DefaultRegistry
}

// Strategy is the default logic that applies when creating ImageStreamMapping
// objects via the REST API.
func NewStrategy(defaultRegistry api.DefaultRegistry) Strategy {
	return Strategy{
		kapi.Scheme,
		kapi.SimpleNameGenerator,
		defaultRegistry,
	}
}

// NamespaceScoped is true for image stream mappings.
func (s Strategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (s Strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	ism := obj.(*api.ImageStreamMapping)
	if len(ism.Image.DockerImageReference) == 0 {
		internalRegistry, ok := s.defaultRegistry.DefaultRegistry()
		if ok {
			ism.Image.DockerImageReference = api.DockerImageReference{
				Registry:  internalRegistry,
				Namespace: ism.Namespace,
				Name:      ism.Name,
				ID:        ism.Image.Name,
			}.Exact()
		}
	}

	// signatures can be added using "images" or "imagesignatures" resources
	ism.Image.Signatures = nil
}

// Canonicalize normalizes the object after validation.
func (s Strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new ImageStreamMapping.
func (s Strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	mapping := obj.(*api.ImageStreamMapping)
	return validation.ValidateImageStreamMapping(mapping)
}
