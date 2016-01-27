package validation

import (
	"fmt"
	"regexp"

	"github.com/docker/distribution/reference"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/image/api"
)

// RepositoryNameComponentRegexp restricts registry path component names to
// start with at least one letter or number, with following parts able to
// be separated by one period, dash or underscore.
// Copied from github.com/docker/distribution/registry/api/v2/names.go v2.1.1
var RepositoryNameComponentRegexp = regexp.MustCompile(`[a-z0-9]+(?:[._-][a-z0-9]+)*`)

// RepositoryNameComponentAnchoredRegexp is the version of
// RepositoryNameComponentRegexp which must completely match the content
// Copied from github.com/docker/distribution/registry/api/v2/names.go v2.1.1
var RepositoryNameComponentAnchoredRegexp = regexp.MustCompile(`^` + RepositoryNameComponentRegexp.String() + `$`)

// RepositoryNameRegexp builds on RepositoryNameComponentRegexp to allow
// multiple path components, separated by a forward slash.
// Copied from github.com/docker/distribution/registry/api/v2/names.go v2.1.1
var RepositoryNameRegexp = regexp.MustCompile(`(?:` + RepositoryNameComponentRegexp.String() + `/)*` + RepositoryNameComponentRegexp.String())

func ValidateImageStreamName(name string, prefix bool) (bool, string) {
	if ok, reason := oapi.MinimalNameRequirements(name, prefix); !ok {
		return ok, reason
	}

	if !RepositoryNameComponentAnchoredRegexp.MatchString(name) {
		return false, fmt.Sprintf("must match %q", RepositoryNameComponentRegexp.String())
	}
	return true, ""
}

// ValidateImage tests required fields for an Image.
func ValidateImage(image *api.Image) field.ErrorList {
	return validateImage(image, nil)
}

func validateImage(image *api.Image, fldPath *field.Path) field.ErrorList {
	result := validation.ValidateObjectMeta(&image.ObjectMeta, false, oapi.MinimalNameRequirements, fldPath.Child("metadata"))

	if len(image.DockerImageReference) == 0 {
		result = append(result, field.Required(fldPath.Child("dockerImageReference")))
	} else {
		if _, err := api.ParseDockerImageReference(image.DockerImageReference); err != nil {
			result = append(result, field.Invalid(fldPath.Child("dockerImageReference"), image.DockerImageReference, err.Error()))
		}
	}

	return result
}

func ValidateImageUpdate(newImage, oldImage *api.Image) field.ErrorList {
	result := validation.ValidateObjectMetaUpdate(&newImage.ObjectMeta, &oldImage.ObjectMeta, field.NewPath("metadata"))
	result = append(result, ValidateImage(newImage)...)

	return result
}

// ValidateImageStream tests required fields for an ImageStream.
func ValidateImageStream(stream *api.ImageStream) field.ErrorList {
	result := validation.ValidateObjectMeta(&stream.ObjectMeta, true, ValidateImageStreamName, field.NewPath("metadata"))

	// Ensure we can generate a valid docker image repository from namespace/name
	if len(stream.Namespace+"/"+stream.Name) > reference.NameTotalLengthMax {
		result = append(result, field.Invalid(field.NewPath("metadata", "name"), stream.Name, fmt.Sprintf("'namespace/name' cannot be longer than %d characters", reference.NameTotalLengthMax)))
	}

	if stream.Spec.Tags == nil {
		stream.Spec.Tags = make(map[string]api.TagReference)
	}

	if len(stream.Spec.DockerImageRepository) != 0 {
		dockerImageRepositoryPath := field.NewPath("spec", "dockerImageRepository")
		if ref, err := api.ParseDockerImageReference(stream.Spec.DockerImageRepository); err != nil {
			result = append(result, field.Invalid(dockerImageRepositoryPath, stream.Spec.DockerImageRepository, err.Error()))
		} else {
			if len(ref.Tag) > 0 {
				result = append(result, field.Invalid(dockerImageRepositoryPath, stream.Spec.DockerImageRepository, "the repository name may not contain a tag"))
			}
			if len(ref.ID) > 0 {
				result = append(result, field.Invalid(dockerImageRepositoryPath, stream.Spec.DockerImageRepository, "the repository name may not contain an ID"))
			}
		}
	}
	for tag, tagRef := range stream.Spec.Tags {
		if tagRef.From != nil {
			switch tagRef.From.Kind {
			case "DockerImage", "ImageStreamImage", "ImageStreamTag":
			default:
				result = append(result, field.Invalid(field.NewPath("spec", "tags").Key(tag).Child("from", "kind"), tagRef.From.Kind, "valid values are 'DockerImage', 'ImageStreamImage', 'ImageStreamTag'"))
			}
		}
	}
	for tag, history := range stream.Status.Tags {
		for i, tagEvent := range history.Items {
			if len(tagEvent.DockerImageReference) == 0 {
				result = append(result, field.Required(field.NewPath("status", "tags").Key(tag).Child("items").Index(i).Child("dockerImageReference")))
			}
		}
	}

	return result
}

func ValidateImageStreamUpdate(newStream, oldStream *api.ImageStream) field.ErrorList {
	result := validation.ValidateObjectMetaUpdate(&newStream.ObjectMeta, &oldStream.ObjectMeta, field.NewPath("metadata"))
	result = append(result, ValidateImageStream(newStream)...)

	return result
}

// ValidateImageStreamStatusUpdate tests required fields for an ImageStream status update.
func ValidateImageStreamStatusUpdate(newStream, oldStream *api.ImageStream) field.ErrorList {
	result := validation.ValidateObjectMetaUpdate(&newStream.ObjectMeta, &oldStream.ObjectMeta, field.NewPath("metadata"))
	return result
}

// ValidateImageStreamMapping tests required fields for an ImageStreamMapping.
func ValidateImageStreamMapping(mapping *api.ImageStreamMapping) field.ErrorList {
	result := validation.ValidateObjectMeta(&mapping.ObjectMeta, true, oapi.MinimalNameRequirements, field.NewPath("metadata"))

	hasRepository := len(mapping.DockerImageRepository) != 0
	hasName := len(mapping.Name) != 0
	switch {
	case hasRepository:
		if _, err := api.ParseDockerImageReference(mapping.DockerImageRepository); err != nil {
			result = append(result, field.Invalid(field.NewPath("dockerImageRepository"), mapping.DockerImageRepository, err.Error()))
		}
	case hasName:
	default:
		result = append(result, field.Required(field.NewPath("name")))
		result = append(result, field.Required(field.NewPath("dockerImageRepository")))
	}

	if ok, msg := validation.ValidateNamespaceName(mapping.Namespace, false); !ok {
		result = append(result, field.Invalid(field.NewPath("metadata", "namespace"), mapping.Namespace, msg))
	}
	if len(mapping.Tag) == 0 {
		result = append(result, field.Required(field.NewPath("tag")))
	}
	if errs := validateImage(&mapping.Image, field.NewPath("image")); len(errs) != 0 {
		result = append(result, errs...)
	}
	return result
}

// ValidateImageStreamTag is essentially a no-op.  We don't allow direct creation of istags
func ValidateImageStreamTag(ist *api.ImageStreamTag) field.ErrorList {
	return validation.ValidateObjectMeta(&ist.ObjectMeta, true, oapi.MinimalNameRequirements, field.NewPath("metadata"))
}

// ValidateImageStreamTagUpdate ensures that only the annotations of the IST have changed
func ValidateImageStreamTagUpdate(newIST, oldIST *api.ImageStreamTag) field.ErrorList {
	result := validation.ValidateObjectMetaUpdate(&newIST.ObjectMeta, &oldIST.ObjectMeta, field.NewPath("metadata"))

	// ensure that only annotations have changed
	newISTCopy := *newIST
	oldISTCopy := *oldIST
	newISTCopy.Annotations = nil
	oldISTCopy.Annotations = nil
	if !kapi.Semantic.Equalities.DeepEqual(&newISTCopy, &oldISTCopy) {
		result = append(result, field.Invalid(field.NewPath("metadata"), "", "may not update fields other than metadata.annotations"))
	}

	return result
}

func ValidateImageStreamImport(isi *api.ImageStreamImport) field.ErrorList {
	specPath := field.NewPath("spec")
	imagesPath := specPath.Child("images")
	repoPath := specPath.Child("repository")

	errs := field.ErrorList{}
	for i, spec := range isi.Spec.Images {
		from := spec.From
		switch from.Kind {
		case "DockerImage":
			if spec.To != nil && len(spec.To.Name) == 0 {
				errs = append(errs, field.Invalid(imagesPath.Index(i).Child("to", "name"), spec.To.Name, "the name of the target tag must be specified"))
			}
			if len(spec.From.Name) == 0 {
				errs = append(errs, field.Required(imagesPath.Index(i).Child("from", "name")))
			} else {
				if _, err := api.ParseDockerImageReference(spec.From.Name); err != nil {
					errs = append(errs, field.Invalid(imagesPath.Index(i).Child("from", "name"), spec.From.Name, err.Error()))
				}
			}
		default:
			errs = append(errs, field.Invalid(imagesPath.Index(i).Child("from", "kind"), from.Kind, "only DockerImage is supported"))
		}
	}

	if spec := isi.Spec.Repository; spec != nil {
		from := spec.From
		switch from.Kind {
		case "DockerImage":
			if len(spec.From.Name) == 0 {
				errs = append(errs, field.Required(repoPath.Child("from", "name")))
			} else {
				if _, err := api.ParseDockerImageReference(spec.From.Name); err != nil {
					errs = append(errs, field.Invalid(repoPath.Child("from", "name"), spec.From.Name, err.Error()))
				}
			}
		default:
			errs = append(errs, field.Invalid(repoPath.Child("from", "kind"), from.Kind, "only DockerImage is supported"))
		}
	}
	if len(isi.Spec.Images) == 0 && isi.Spec.Repository == nil {
		errs = append(errs, field.Invalid(imagesPath, nil, "you must specify at least one image or a repository import"))
	}

	errs = append(errs, validation.ValidateObjectMeta(&isi.ObjectMeta, true, ValidateImageStreamName, field.NewPath("metadata"))...)
	return errs
}
