package validation

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/image/api"
)

// ValidateImage tests required fields for an Image.
func ValidateImage(image *api.Image) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	if len(image.Name) == 0 {
		result = append(result, fielderrors.NewFieldRequired("name"))
	}
	if len(image.DockerImageReference) == 0 {
		result = append(result, fielderrors.NewFieldRequired("dockerImageReference"))
	} else {
		if _, err := api.ParseDockerImageReference(image.DockerImageReference); err != nil {
			result = append(result, fielderrors.NewFieldInvalid("dockerImageReference", image.DockerImageReference, err.Error()))
		}
	}

	return result
}

// ValidateImageStream tests required fields for an ImageStream.
func ValidateImageStream(stream *api.ImageStream) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	if stream.Spec.Tags == nil {
		stream.Spec.Tags = make(map[string]api.TagReference)
	}
	if len(stream.Name) == 0 {
		result = append(result, fielderrors.NewFieldRequired("name"))
	}
	if !util.IsDNS1123Subdomain(stream.Namespace) {
		result = append(result, fielderrors.NewFieldInvalid("namespace", stream.Namespace, ""))
	}
	if len(stream.Spec.DockerImageRepository) != 0 {
		if _, err := api.ParseDockerImageReference(stream.Spec.DockerImageRepository); err != nil {
			result = append(result, fielderrors.NewFieldInvalid("spec.dockerImageRepository", stream.Spec.DockerImageRepository, err.Error()))
		}
	}
	for tag, history := range stream.Status.Tags {
		for i, tagEvent := range history.Items {
			if len(tagEvent.DockerImageReference) == 0 {
				result = append(result, fielderrors.NewFieldRequired(fmt.Sprintf("status.tags[%s].Items[%d].dockerImageReference", tag, i)))
			}
		}
	}

	return result
}

func ValidateImageStreamUpdate(newStream, oldStream *api.ImageStream) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	result = append(result, validation.ValidateObjectMetaUpdate(&oldStream.ObjectMeta, &newStream.ObjectMeta).Prefix("metadata")...)
	result = append(result, ValidateImageStream(newStream)...)

	return result
}

// ValidateImageStreamStatusUpdate tests required fields for an ImageStream status update.
func ValidateImageStreamStatusUpdate(newStream, oldStream *api.ImageStream) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	result = append(result, validation.ValidateObjectMetaUpdate(&oldStream.ObjectMeta, &newStream.ObjectMeta).Prefix("metadata")...)
	newStream.Spec.Tags = oldStream.Spec.Tags
	newStream.Spec.DockerImageRepository = oldStream.Spec.DockerImageRepository
	return result
}

// ValidateImageStreamMapping tests required fields for an ImageStreamMapping.
func ValidateImageStreamMapping(mapping *api.ImageStreamMapping) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	hasRepository := len(mapping.DockerImageRepository) != 0
	hasName := len(mapping.Name) != 0
	switch {
	case hasRepository:
		if _, err := api.ParseDockerImageReference(mapping.DockerImageRepository); err != nil {
			result = append(result, fielderrors.NewFieldInvalid("dockerImageRepository", mapping.DockerImageRepository, err.Error()))
		}
	case hasName:
	default:
		result = append(result, fielderrors.NewFieldRequired("name"))
		result = append(result, fielderrors.NewFieldRequired("dockerImageRepository"))
	}

	if !util.IsDNS1123Subdomain(mapping.Namespace) {
		result = append(result, fielderrors.NewFieldInvalid("namespace", mapping.Namespace, ""))
	}
	if len(mapping.Tag) == 0 {
		result = append(result, fielderrors.NewFieldRequired("tag"))
	}
	if errs := ValidateImage(&mapping.Image).Prefix("image"); len(errs) != 0 {
		result = append(result, errs...)
	}
	return result
}
