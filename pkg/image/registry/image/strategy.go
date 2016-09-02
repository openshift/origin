package image

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
)

// imageStrategy implements behavior for Images.
type imageStrategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

// Strategy is the default logic that applies when creating and updating
// Image objects via the REST API.
var Strategy = imageStrategy{kapi.Scheme, kapi.SimpleNameGenerator}

// NamespaceScoped is false for images.
func (imageStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
// It extracts the latest information from the manifest (if available) and sets that onto the object.
func (s imageStrategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	newImage := obj.(*api.Image)
	// ignore errors, change in place
	if err := api.ImageWithMetadata(newImage); err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to update image metadata for %q: %v", newImage.Name, err))
	}

	// clear signature fields that will be later set by server once it's able to parse the content
	s.clearSignatureDetails(newImage)
}

// Validate validates a new image.
func (imageStrategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	image := obj.(*api.Image)
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
func (s imageStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	newImage := obj.(*api.Image)
	oldImage := old.(*api.Image)

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

	// allow an image update that results in the manifest matching the digest (the name)
	newManifest := newImage.DockerImageManifest
	newImage.DockerImageManifest = oldImage.DockerImageManifest
	if newManifest != oldImage.DockerImageManifest && len(newManifest) > 0 {
		ok, err := api.ManifestMatchesImage(oldImage, []byte(newManifest))
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("attempted to validate that a manifest change to %q matched the signature, but failed: %v", oldImage.Name, err))
		} else if ok {
			newImage.DockerImageManifest = newManifest
		}
	}

	newImageConfig := newImage.DockerImageConfig
	newImage.DockerImageConfig = oldImage.DockerImageConfig
	if newImageConfig != oldImage.DockerImageConfig && len(newImageConfig) > 0 {
		ok, err := api.ImageConfigMatchesImage(newImage, []byte(newImageConfig))
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("attempted to validate that a new config for %q mentioned in the manifest, but failed: %v", oldImage.Name, err))
		} else if ok {
			newImage.DockerImageConfig = newImageConfig
		}
	}

	if err := api.ImageWithMetadata(newImage); err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to update image metadata for %q: %v", newImage.Name, err))
	}

	// clear signature fields that will be later set by server once it's able to parse the content
	s.clearSignatureDetails(newImage)
}

// ValidateUpdate is the default update validation for an end user.
func (imageStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateImageUpdate(old.(*api.Image), obj.(*api.Image))
}

// clearSignatureDetails removes signature details from all the signatures of given image. It also clear all
// the validation data. These data will be set by the server once the signature parsing support is added.
func (imageStrategy) clearSignatureDetails(image *api.Image) {
	for i := range image.Signatures {
		signature := &image.Signatures[i]
		signature.Conditions = nil
		signature.ImageIdentity = ""
		signature.SignedClaims = nil
		signature.Created = nil
		signature.IssuedBy = nil
		signature.IssuedTo = nil
	}
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(o runtime.Object) (labels.Set, fields.Set, error) {
			obj, ok := o.(*api.Image)
			if !ok {
				return nil, nil, fmt.Errorf("not an image")
			}
			return labels.Set(obj.Labels), SelectableFields(obj), nil
		},
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *api.Image) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}
