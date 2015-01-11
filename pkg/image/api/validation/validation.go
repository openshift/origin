package validation

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/image/api"
)

// ValidateImage tests required fields for an Image.
func ValidateImage(image *api.Image) errors.ValidationErrorList {
	result := errors.ValidationErrorList{}

	if len(image.Name) == 0 {
		result = append(result, errors.NewFieldRequired("name", image.Name))
	}
	if !util.IsDNSSubdomain(image.Namespace) {
		result = append(result, errors.NewFieldInvalid("namespace", image.Namespace, ""))
	}
	if len(image.DockerImageReference) == 0 {
		result = append(result, errors.NewFieldRequired("dockerImageReference", image.DockerImageReference))
	} else {
		_, _, _, _, err := api.SplitDockerPullSpec(image.DockerImageReference)
		if err != nil {
			result = append(result, errors.NewFieldInvalid("dockerImageReference", image.DockerImageReference, err.Error()))
		}
	}

	return result
}

// ValidateImageRepository tests required fields for an ImageRepository.
func ValidateImageRepository(repo *api.ImageRepository) errors.ValidationErrorList {
	result := errors.ValidationErrorList{}

	if repo.Tags == nil {
		repo.Tags = make(map[string]string)
	}
	if len(repo.Name) == 0 {
		result = append(result, errors.NewFieldRequired("name", repo.Name))
	}
	if !util.IsDNSSubdomain(repo.Namespace) {
		result = append(result, errors.NewFieldInvalid("namespace", repo.Namespace, ""))
	}
	if len(repo.DockerImageRepository) != 0 {
		_, _, _, _, err := api.SplitDockerPullSpec(repo.DockerImageRepository)
		if err != nil {
			result = append(result, errors.NewFieldInvalid("dockerImageRepository", repo.DockerImageRepository, err.Error()))
		}
	}

	return result
}

// ValidateImageRepositoryMapping tests required fields for an ImageRepositoryMapping.
func ValidateImageRepositoryMapping(mapping *api.ImageRepositoryMapping) errors.ValidationErrorList {
	result := errors.ValidationErrorList{}

	hasRepository := len(mapping.DockerImageRepository) != 0
	hasName := len(mapping.Name) != 0
	switch {
	case hasRepository:
		_, _, _, _, err := api.SplitDockerPullSpec(mapping.DockerImageRepository)
		if err != nil {
			result = append(result, errors.NewFieldInvalid("dockerImageRepository", mapping.DockerImageRepository, err.Error()))
		}
	case hasName:
	default:
		result = append(result, errors.NewFieldRequired("name", ""))
		result = append(result, errors.NewFieldRequired("dockerImageRepository", ""))
	}

	if !util.IsDNSSubdomain(mapping.Namespace) {
		result = append(result, errors.NewFieldInvalid("namespace", mapping.Namespace, ""))
	}
	if len(mapping.Tag) == 0 {
		result = append(result, errors.NewFieldRequired("tag", mapping.Tag))
	}
	if len(mapping.Image.Namespace) == 0 {
		mapping.Image.Namespace = mapping.Namespace
	}
	if errs := ValidateImage(&mapping.Image).Prefix("image"); len(errs) != 0 {
		result = append(result, errs...)
	}
	return result
}
