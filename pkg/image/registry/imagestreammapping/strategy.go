package imagestreammapping

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage/names"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
)

// Strategy implements behavior for image stream mappings.
type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator

	defaultRegistry api.DefaultRegistry
}

// Strategy is the default logic that applies when creating ImageStreamMapping
// objects via the REST API.
func NewStrategy(defaultRegistry api.DefaultRegistry) Strategy {
	return Strategy{
		kapi.Scheme,
		names.SimpleNameGenerator,
		defaultRegistry,
	}
}

// NamespaceScoped is true for image stream mappings.
func (s Strategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (s Strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
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
func (s Strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	mapping := obj.(*api.ImageStreamMapping)
	return validation.ValidateImageStreamMapping(mapping)
}
