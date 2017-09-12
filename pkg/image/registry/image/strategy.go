package image

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	kapi "k8s.io/kubernetes/pkg/api"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation"
)

type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator

	registryHostnameRetriever imageapi.RegistryHostnameRetriever
}

func NewStrategy(registryHostname imageapi.RegistryHostnameRetriever) Strategy {
	return Strategy{
		ObjectTyper:   kapi.Scheme,
		NameGenerator: names.SimpleNameGenerator,

		registryHostnameRetriever: registryHostname,
	}
}

func (s Strategy) DefaultGarbageCollectionPolicy() rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

// NamespaceScoped is false for images.
func (Strategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
// It extracts the latest information from the manifest (if available) and sets that onto the object.
func (s Strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	newImage := obj.(*imageapi.Image)
	// ignore errors, change in place
	if err := imageapi.ImageWithMetadata(newImage); err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to update image metadata for %q: %v", newImage.Name, err))
	}
	err := imageapi.UpdateWithRegistryHostnames(s.registryHostnameRetriever, newImage)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to set dockerImageRepository for %s: %v", newImage.Name, err))
	}
}

// Validate validates a new image.
func (Strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	image := obj.(*imageapi.Image)
	return validation.ValidateImage(image)
}

// AllowCreateOnUpdate is false for images.
func (Strategy) AllowCreateOnUpdate() bool {
	return false
}

func (Strategy) AllowUnconditionalUpdate() bool {
	return false
}

// Canonicalize normalizes the object after validation.
func (Strategy) Canonicalize(obj runtime.Object) {
}

func (s Strategy) Decorate(obj runtime.Object) error {
	return imageapi.UpdateWithRegistryHostnames(s.registryHostnameRetriever, obj)
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
// It extracts the latest info from the manifest and sets that on the object. It allows a user
// to update the manifest so that it matches the digest (in case an older server stored a manifest
// that was malformed, it can always be corrected).
func (s Strategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	newImage := obj.(*imageapi.Image)
	oldImage := old.(*imageapi.Image)

	// image metadata cannot be altered
	newImage.DockerImageMetadata = oldImage.DockerImageMetadata
	newImage.DockerImageMetadataVersion = oldImage.DockerImageMetadataVersion
	newImage.DockerImageLayers = oldImage.DockerImageLayers

	if oldImage.DockerImageSignatures != nil {
		newImage.DockerImageSignatures = nil
		for _, v := range oldImage.DockerImageSignatures {
			newImage.DockerImageSignatures = append(newImage.DockerImageSignatures, v)
		}
	}

	var err error

	// allow an image update that results in the manifest matching the digest (the name)
	if newImage.DockerImageManifest != oldImage.DockerImageManifest {
		ok := true
		if len(newImage.DockerImageManifest) > 0 {
			ok, err = imageapi.ManifestMatchesImage(oldImage, []byte(newImage.DockerImageManifest))
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("attempted to validate that a manifest change to %q matched the signature, but failed: %v", oldImage.Name, err))
			}
		}
		if !ok {
			newImage.DockerImageManifest = oldImage.DockerImageManifest
		}
	}

	if newImage.DockerImageConfig != oldImage.DockerImageConfig {
		ok := true
		if len(newImage.DockerImageConfig) > 0 {
			ok, err = imageapi.ImageConfigMatchesImage(newImage, []byte(newImage.DockerImageConfig))
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("attempted to validate that a new config for %q mentioned in the manifest, but failed: %v", oldImage.Name, err))
			}
		}
		if !ok {
			newImage.DockerImageConfig = oldImage.DockerImageConfig
		}
	}

	if err = imageapi.ImageWithMetadata(newImage); err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to update image metadata for %q: %v", newImage.Name, err))
	}

	if err = imageapi.UpdateWithRegistryHostnames(s.registryHostnameRetriever, newImage); err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to set dockerImageRepository for %s: %v", newImage.Name, err))
	}
}

// ValidateUpdate is the default update validation for an end user.
func (Strategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateImageUpdate(old.(*imageapi.Image), obj.(*imageapi.Image))
}
