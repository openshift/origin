package validation

import (
	"fmt"
	"strings"

	"github.com/docker/distribution/registry/api/v2"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/image/api"
)

var qualifiedNameErrorMsg string = fmt.Sprintf(`must be a qualified name (at most %d characters, matching regex %s), with an optional DNS subdomain prefix (at most %d characters, matching regex %s) and slash (/): e.g. "MyName" or "example.com/MyName"`, util.QualifiedNameMaxLength, util.QualifiedNameFmt, util.DNS1123SubdomainMaxLength, util.DNS1123SubdomainFmt)

func ValidateImageStreamName(name string, prefix bool) (bool, string) {
	if ok, reason := oapi.MinimalNameRequirements(name, prefix); !ok {
		return ok, reason
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
	if image.Status.Phase != api.ImageAvailable && image.Status.Phase != api.ImagePurging {
		result = append(result, fielderrors.NewFieldInvalid("status.phase", image.Status.Phase, fmt.Sprintf("phase is not one of valid phases (%s, %s)", api.ImageAvailable, api.ImagePurging)))
	}
	if image.DeletionTimestamp == nil && image.Status.Phase == api.ImagePurging {
		result = append(result, fielderrors.NewFieldInvalid("status.phase", image.Status.Phase, fmt.Sprintf("%s phase is valid  only when DeletionTimestamp is set", api.ImagePurging)))

	}

	return result
}

// ValidateImageUpdate validates an update of image
func ValidateImageUpdate(newImage, oldImage *api.Image) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	result = append(result, validation.ValidateObjectMetaUpdate(&newImage.ObjectMeta, &oldImage.ObjectMeta).Prefix("metadata")...)
	result = append(result, ValidateImage(newImage)...)

	return result
}

// ValidateImageStatusUpdate tests required fields for an Image status update.
func ValidateImageStatusUpdate(newImage, oldImage *api.Image) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	result = append(result, validation.ValidateObjectMetaUpdate(&newImage.ObjectMeta, &oldImage.ObjectMeta).Prefix("metadata")...)
	if newImage.Status.Phase != api.ImageAvailable && newImage.Status.Phase != api.ImagePurging {
		result = append(result, fielderrors.NewFieldInvalid("status.phase", newImage.Status.Phase, fmt.Sprintf("phase is not one of valid phases (%s, %s)", api.ImageAvailable, api.ImagePurging)))
	}
	if newImage.DeletionTimestamp == nil && newImage.Status.Phase == api.ImagePurging {
		result = append(result, fielderrors.NewFieldInvalid("status.phase", newImage.Status.Phase, fmt.Sprintf("%s phase is valid only when DeletionTimestamp is set", api.ImagePurging)))

	}

	newImage.DockerImageReference = oldImage.DockerImageReference
	newImage.DockerImageMetadata = oldImage.DockerImageMetadata
	newImage.DockerImageMetadataVersion = oldImage.DockerImageMetadataVersion
	newImage.DockerImageManifest = oldImage.DockerImageManifest
	newImage.Finalizers = oldImage.Finalizers
	return result
}

// ValidateImageFinalizeUpdate tests to see if the update is legal for an end user to make.
// newImage is updated with fields that cannot be changed.
func ValidateImageFinalizeUpdate(newImage, oldImage *api.Image) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&newImage.ObjectMeta, &oldImage.ObjectMeta).Prefix("metadata")...)
	for i := range newImage.Finalizers {
		allErrs = append(allErrs, validateFinalizerName(string(newImage.Finalizers[i]))...)
	}
	newImage.DockerImageReference = oldImage.DockerImageReference
	newImage.DockerImageMetadata = oldImage.DockerImageMetadata
	newImage.DockerImageMetadataVersion = oldImage.DockerImageMetadataVersion
	newImage.DockerImageManifest = oldImage.DockerImageManifest
	newImage.Status = oldImage.Status
	return allErrs
}

// validateFinalizerName validates finalizer names
func validateFinalizerName(stringValue string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if !util.IsQualifiedName(stringValue) {
		return append(allErrs, fielderrors.NewFieldInvalid("finalizers", stringValue, qualifiedNameErrorMsg))
	}

	if strings.Index(stringValue, "/") < 0 && !kapi.IsStandardFinalizerName(stringValue) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("finalizers", stringValue, fmt.Sprintf("finalizer name is neither a standard finalizer name nor is it fully qualified")))
	} else if strings.Index(stringValue, "/") >= 0 && stringValue != string(oapi.FinalizerOrigin) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("finalizers", stringValue, fmt.Sprintf("is fully qualified and doesn't match %s", oapi.FinalizerOrigin)))
	}

	return allErrs
}

// ValidateImageStream tests required fields for an ImageStream.
func ValidateImageStream(stream *api.ImageStream) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	result = append(result, validation.ValidateObjectMeta(&stream.ObjectMeta, true, ValidateImageStreamName).Prefix("metadata")...)

	// Ensure we can generate a valid docker image repository from namespace/name
	if len(stream.Namespace+"/"+stream.Name) > v2.RepositoryNameTotalLengthMax {
		result = append(result, fielderrors.NewFieldInvalid("metadata.name", stream.Name, fmt.Sprintf("'namespace/name' cannot be longer than %d characters", v2.RepositoryNameTotalLengthMax)))
	}

	if stream.Spec.Tags == nil {
		stream.Spec.Tags = make(map[string]api.TagReference)
	}

	if len(stream.Spec.DockerImageRepository) != 0 {
		if ref, err := api.ParseDockerImageReference(stream.Spec.DockerImageRepository); err != nil {
			result = append(result, fielderrors.NewFieldInvalid("spec.dockerImageRepository", stream.Spec.DockerImageRepository, err.Error()))
		} else {
			if len(ref.Tag) > 0 {
				result = append(result, fielderrors.NewFieldInvalid("spec.dockerImageRepository", stream.Spec.DockerImageRepository, "the repository name may not contain a tag"))
			}
			if len(ref.ID) > 0 {
				result = append(result, fielderrors.NewFieldInvalid("spec.dockerImageRepository", stream.Spec.DockerImageRepository, "the repository name may not contain an ID"))
			}
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

	result = append(result, validation.ValidateObjectMetaUpdate(&newStream.ObjectMeta, &oldStream.ObjectMeta).Prefix("metadata")...)
	result = append(result, ValidateImageStream(newStream)...)

	return result
}

// ValidateImageStreamStatusUpdate tests required fields for an ImageStream status update.
func ValidateImageStreamStatusUpdate(newStream, oldStream *api.ImageStream) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	result = append(result, validation.ValidateObjectMetaUpdate(&newStream.ObjectMeta, &oldStream.ObjectMeta).Prefix("metadata")...)
	newStream.Spec.Tags = oldStream.Spec.Tags
	newStream.Spec.DockerImageRepository = oldStream.Spec.DockerImageRepository
	return result
}

// ValidateImageStreamMapping tests required fields for an ImageStreamMapping.
func ValidateImageStreamMapping(mapping *api.ImageStreamMapping) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	result = append(result, validation.ValidateObjectMeta(&mapping.ObjectMeta, true, oapi.MinimalNameRequirements).Prefix("metadata")...)

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

// ValidateImageStreamDeletion tests required fields for an ImageStreamDeletion.
func ValidateImageStreamDeletion(isd *api.ImageStreamDeletion) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	result = append(result, validation.ValidateObjectMeta(&isd.ObjectMeta, false, oapi.MinimalNameRequirements).Prefix("metadata")...)
	if isd.Name != "" {
		nameParts := strings.SplitN(isd.Name, ":", 2)
		if len(nameParts) != 2 {
			result = append(result, fielderrors.NewFieldInvalid("name", isd.Name, "must have following format: namespace:name"))
		} else {
			if ok, msg := validation.ValidateNamespaceName(nameParts[0], false); !ok {
				result = append(result, fielderrors.NewFieldInvalid("name", isd.Name, fmt.Sprintf("part before ':' is not a valid namespace: %v", msg)))
			}
			if len(isd.Namespace+"/"+isd.Name) > v2.RepositoryNameTotalLengthMax {
				result = append(result, fielderrors.NewFieldInvalid("metadata.name", isd.Name, fmt.Sprintf("'namespace/name' cannot be longer than %d characters", v2.RepositoryNameTotalLengthMax)))
			}
		}
	}
	return result
}
