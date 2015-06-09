package validation

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	"github.com/docker/distribution/registry/api/v2"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/image/api"
)

func ValidateImageStreamName(name string, prefix bool) (bool, string) {
	if ok, reason := oapi.MinimalNameRequirements(name, prefix); !ok {
		return ok, reason
	}

	if len(name) < v2.RepositoryNameComponentMinLength {
		return false, fmt.Sprintf("must be at least %d characters long", v2.RepositoryNameComponentMinLength)
	}
	if !v2.RepositoryNameComponentAnchoredRegexp.MatchString(name) {
		return false, fmt.Sprintf("must match %q", v2.RepositoryNameComponentRegexp.String())
	}
	return true, ""
}

// ValidateImage tests required fields for an Image.
func ValidateImage(image *api.Image) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	result = append(result, validation.ValidateObjectMeta(&image.ObjectMeta, false, oapi.MinimalNameRequirements).Prefix("metadata")...)

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
	result = append(result, validation.ValidateObjectMeta(&stream.ObjectMeta, true, ValidateImageStreamName).Prefix("metadata")...)

	// Ensure we can generate a valid docker image repository from namespace/name
	if len(stream.Namespace) > 0 && len(stream.Namespace) < v2.RepositoryNameComponentMinLength {
		result = append(result, fielderrors.NewFieldInvalid("metadata.namespace", stream.Namespace, fmt.Sprintf("must be at least %d characters long", v2.RepositoryNameComponentMinLength)))
	}
	if len(stream.Namespace+"/"+stream.Name) > v2.RepositoryNameTotalLengthMax {
		result = append(result, fielderrors.NewFieldInvalid("metadata.name", stream.Name, fmt.Sprintf("'namespace/name' cannot be longer than %d characters", v2.RepositoryNameTotalLengthMax)))
	}

	if stream.Spec.Tags == nil {
		stream.Spec.Tags = make(map[string]api.TagReference)
	}

	if len(stream.Spec.DockerImageRepository) != 0 {
		if _, err := api.ParseDockerImageReference(stream.Spec.DockerImageRepository); err != nil {
			result = append(result, fielderrors.NewFieldInvalid("spec.dockerImageRepository", stream.Spec.DockerImageRepository, err.Error()))
		}
	}
	for tag, tagRef := range stream.Spec.Tags {
		if tagRef.From != nil {
			switch tagRef.From.Kind {
			case "DockerImage", "ImageStreamImage", "ImageStreamTag":
			default:
				result = append(result, fielderrors.NewFieldInvalid(fmt.Sprintf("spec.tags[%s].from.kind", tag), tagRef.From.Kind, "valid values are 'DockerImage', 'ImageStreamImage', 'ImageStreamTag'"))
			}
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

	if ok, msg := validation.ValidateNamespaceName(mapping.Namespace, false); !ok {
		result = append(result, fielderrors.NewFieldInvalid("namespace", mapping.Namespace, msg))
	}
	if len(mapping.Tag) == 0 {
		result = append(result, fielderrors.NewFieldRequired("tag"))
	}
	if errs := ValidateImage(&mapping.Image).Prefix("image"); len(errs) != 0 {
		result = append(result, errs...)
	}
	return result
}
