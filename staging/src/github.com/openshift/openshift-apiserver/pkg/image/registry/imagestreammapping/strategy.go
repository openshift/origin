package imagestreammapping

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
)

// Strategy implements behavior for image stream mappings.
type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator

	registryHostRetriever registryhostname.RegistryHostnameRetriever
}

// Strategy is the default logic that applies when creating ImageStreamMapping
// objects via the REST API.
func NewStrategy(registryHost registryhostname.RegistryHostnameRetriever) Strategy {
	return Strategy{
		ObjectTyper:           legacyscheme.Scheme,
		NameGenerator:         names.SimpleNameGenerator,
		registryHostRetriever: registryHost,
	}
}

// NamespaceScoped is true for image stream mappings.
func (s Strategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (s Strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	ism := obj.(*imageapi.ImageStreamMapping)
	if len(ism.Image.DockerImageReference) == 0 {
		internalRegistry, ok := s.registryHostRetriever.InternalRegistryHostname()
		if ok {
			ism.Image.DockerImageReference = imageapi.DockerImageReference{
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
func (s Strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	mapping := obj.(*imageapi.ImageStreamMapping)
	return validation.ValidateImageStreamMapping(mapping)
}
