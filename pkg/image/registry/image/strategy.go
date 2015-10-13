package image

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"
	errs "k8s.io/kubernetes/pkg/util/fielderrors"

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
func (imageStrategy) PrepareForCreate(obj runtime.Object) {
	image := obj.(*api.Image)
	image.Status = api.ImageStatus{
		Phase: api.ImageAvailable,
	}
}

// Validate validates a new image.
func (imageStrategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
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

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (imageStrategy) PrepareForUpdate(obj, old runtime.Object) {
	newImage := obj.(*api.Image)
	oldImage := old.(*api.Image)
	// image metadata cannot be altered
	newImage.DockerImageMetadata = oldImage.DockerImageMetadata
	newImage.DockerImageManifest = oldImage.DockerImageManifest
	newImage.DockerImageMetadataVersion = oldImage.DockerImageMetadataVersion
	newImage.Finalizers = oldImage.Finalizers
	newImage.Status = newImage.Status
}

// ValidateUpdate is the default update validation for an end user.
func (imageStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) errs.ValidationErrorList {
	return validation.ValidateImageUpdate(old.(*api.Image), obj.(*api.Image))
}

type imageStatusStrategy struct {
	imageStrategy
}

var StatusStrategy = imageStatusStrategy{Strategy}

func (imageStatusStrategy) PrepareForUpdate(obj, old runtime.Object) {
	newImage := obj.(*api.Image)
	oldImage := old.(*api.Image)
	newImage.DockerImageReference = oldImage.DockerImageReference
	newImage.DockerImageMetadata = oldImage.DockerImageMetadata
	newImage.DockerImageMetadataVersion = oldImage.DockerImageMetadataVersion
	newImage.DockerImageManifest = oldImage.DockerImageManifest
	newImage.Finalizers = oldImage.Finalizers
}

func (imageStatusStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateImageStatusUpdate(obj.(*api.Image), old.(*api.Image))
}

type imageFinalizeStrategy struct {
	imageStrategy
}

var FinalizeStrategy = imageFinalizeStrategy{Strategy}

func (imageFinalizeStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateImageFinalizeUpdate(obj.(*api.Image), old.(*api.Image))
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (imageFinalizeStrategy) PrepareForUpdate(obj, old runtime.Object) {
	newImage := obj.(*api.Image)
	oldImage := old.(*api.Image)
	newImage.DockerImageReference = oldImage.DockerImageReference
	newImage.DockerImageMetadata = oldImage.DockerImageMetadata
	newImage.DockerImageMetadataVersion = oldImage.DockerImageMetadataVersion
	newImage.DockerImageManifest = oldImage.DockerImageManifest
	newImage.Status = oldImage.Status
}

// MatchImage returns a generic matcher for a given label and field selector.
func MatchImage(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		image, ok := obj.(*api.Image)
		if !ok {
			return false, fmt.Errorf("not an image")
		}
		fields := ImageToSelectableFields(image)
		return label.Matches(labels.Set(image.Labels)) && field.Matches(fields), nil
	})
}

// ImageToSelectableFields returns a label set that represents the object.
func ImageToSelectableFields(image *api.Image) labels.Set {
	return labels.Set{
		"name":         image.Name,
		"status.phase": image.Status.Phase,
	}
}
