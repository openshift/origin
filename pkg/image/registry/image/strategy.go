package image

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation"
	"github.com/openshift/origin/pkg/image/util"
)

// managedSignatureAnnotation used to be set by image signature import controller as a signature annotation.
const managedSignatureAnnotation = "image.openshift.io/managed-signature"

// imageStrategy implements behavior for Images.
type imageStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that applies when creating and updating
// Image objects via the REST API.
var Strategy = imageStrategy{legacyscheme.Scheme, names.SimpleNameGenerator}

var _ rest.GarbageCollectionDeleteStrategy = imageStrategy{}

func (imageStrategy) DefaultGarbageCollectionPolicy(ctx apirequest.Context) rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

// NamespaceScoped is false for images.
func (imageStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
// It extracts the latest information from the manifest (if available) and sets that onto the object.
func (s imageStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	newImage := obj.(*imageapi.Image)
	// ignore errors, change in place
	if err := util.ImageWithMetadata(newImage); err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to update image metadata for %q: %v", newImage.Name, err))
	}
	if newImage.Annotations[imageapi.ImageManifestBlobStoredAnnotation] == "true" {
		newImage.DockerImageManifest = ""
		newImage.DockerImageConfig = ""
	}

	removeManagedSignatureAnnotation(newImage)
}

// Validate validates a new image.
func (imageStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	image := obj.(*imageapi.Image)
	return validation.ValidateImage(image)
}

// AllowCreateOnUpdate is false for images.
func (imageStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (imageStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// Canonicalize normalizes the object after validation.
func (imageStrategy) Canonicalize(obj runtime.Object) {
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
// It extracts the latest info from the manifest and sets that on the object. It allows a user
// to update the manifest so that it matches the digest (in case an older server stored a manifest
// that was malformed, it can always be corrected).
func (s imageStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
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
			ok, err = util.ManifestMatchesImage(oldImage, []byte(newImage.DockerImageManifest))
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
			ok, err = util.ImageConfigMatchesImage(newImage, []byte(newImage.DockerImageConfig))
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("attempted to validate that a new config for %q mentioned in the manifest, but failed: %v", oldImage.Name, err))
			}
		}
		if !ok {
			newImage.DockerImageConfig = oldImage.DockerImageConfig
		}
	}

	if err = util.ImageWithMetadata(newImage); err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to update image metadata for %q: %v", newImage.Name, err))
	}

	if newImage.Annotations[imageapi.ImageManifestBlobStoredAnnotation] == "true" {
		newImage.DockerImageManifest = ""
		newImage.DockerImageConfig = ""
	}

	removeManagedSignatureAnnotation(newImage)
}

// ValidateUpdate is the default update validation for an end user.
func (imageStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateImageUpdate(obj.(*imageapi.Image), old.(*imageapi.Image))
}

// removeManagedSignatureAnnotation removes deprecated annotation from image signatures. A bug in image update
// logic allowed to set arbitrary annotations that would otherwise be rejected by validation.
// Resolves rhbz#1557607
func removeManagedSignatureAnnotation(img *imageapi.Image) {
	for i := range img.Signatures {
		delete(img.Signatures[i].Annotations, managedSignatureAnnotation)
	}
}
