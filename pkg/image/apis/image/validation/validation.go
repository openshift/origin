package validation

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/docker/distribution/reference"

	"k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
	"k8s.io/kubernetes/pkg/api/validation"

	serverapi "github.com/openshift/origin/pkg/cmd/server/api"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	stringsutil "github.com/openshift/origin/pkg/util/strings"
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

func ValidateImageStreamName(name string, prefix bool) []string {
	if reasons := path.ValidatePathSegmentName(name, prefix); len(reasons) != 0 {
		return reasons
	}

	if !RepositoryNameComponentAnchoredRegexp.MatchString(name) {
		return []string{fmt.Sprintf("must match %q", RepositoryNameComponentRegexp.String())}
	}
	return nil
}

// ValidateImage tests required fields for an Image.
func ValidateImage(image *imageapi.Image) field.ErrorList {
	return validateImage(image, nil)
}

func validateImage(image *imageapi.Image, fldPath *field.Path) field.ErrorList {
	result := validation.ValidateObjectMeta(&image.ObjectMeta, false, path.ValidatePathSegmentName, fldPath.Child("metadata"))

	if len(image.DockerImageReference) == 0 {
		result = append(result, field.Required(fldPath.Child("dockerImageReference"), ""))
	} else {
		if _, err := imageapi.ParseDockerImageReference(image.DockerImageReference); err != nil {
			result = append(result, field.Invalid(fldPath.Child("dockerImageReference"), image.DockerImageReference, err.Error()))
		}
	}

	for i, sig := range image.Signatures {
		result = append(result, validateImageSignature(&sig, fldPath.Child("signatures").Index(i))...)
	}

	return result
}

func ValidateImageUpdate(newImage, oldImage *imageapi.Image) field.ErrorList {
	result := validation.ValidateObjectMetaUpdate(&newImage.ObjectMeta, &oldImage.ObjectMeta, field.NewPath("metadata"))
	result = append(result, ValidateImage(newImage)...)

	return result
}

// ValidateImageSignature ensures that given signatures is valid.
func ValidateImageSignature(signature *imageapi.ImageSignature) field.ErrorList {
	return validateImageSignature(signature, nil)
}

func validateImageSignature(signature *imageapi.ImageSignature, fldPath *field.Path) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&signature.ObjectMeta, false, path.ValidatePathSegmentName, fldPath.Child("metadata"))
	if len(signature.Labels) > 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("metadata").Child("labels"), "signature labels cannot be set"))
	}
	if len(signature.Annotations) > 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("metadata").Child("annotations"), "signature annotations cannot be set"))
	}

	if _, _, err := imageapi.SplitImageSignatureName(signature.Name); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("metadata").Child("name"), signature.Name, "name must be of format <imageName>@<signatureName>"))
	}
	if len(signature.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), ""))
	}
	if len(signature.Content) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("content"), ""))
	}

	var trustedCondition, forImageCondition *imageapi.SignatureCondition
	for i := range signature.Conditions {
		cond := &signature.Conditions[i]
		if cond.Type == imageapi.SignatureTrusted && (trustedCondition == nil || !cond.LastProbeTime.Before(trustedCondition.LastProbeTime)) {
			trustedCondition = cond
		} else if cond.Type == imageapi.SignatureForImage && forImageCondition == nil || !cond.LastProbeTime.Before(forImageCondition.LastProbeTime) {
			forImageCondition = cond
		}
	}

	if trustedCondition != nil && forImageCondition == nil {
		msg := fmt.Sprintf("missing %q condition type", imageapi.SignatureForImage)
		allErrs = append(allErrs, field.Invalid(fldPath.Child("conditions"), signature.Conditions, msg))
	} else if forImageCondition != nil && trustedCondition == nil {
		msg := fmt.Sprintf("missing %q condition type", imageapi.SignatureTrusted)
		allErrs = append(allErrs, field.Invalid(fldPath.Child("conditions"), signature.Conditions, msg))
	}

	if trustedCondition == nil || trustedCondition.Status == kapi.ConditionUnknown {
		if len(signature.ImageIdentity) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("imageIdentity"), signature.ImageIdentity, "must be unset for unknown signature state"))
		}
		if len(signature.SignedClaims) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("signedClaims"), signature.SignedClaims, "must be unset for unknown signature state"))
		}
		if signature.IssuedBy != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("issuedBy"), signature.IssuedBy, "must be unset for unknown signature state"))
		}
		if signature.IssuedTo != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("issuedTo"), signature.IssuedTo, "must be unset for unknown signature state"))
		}
	}

	return allErrs
}

// ValidateImageSignatureUpdate ensures that the new ImageSignature is valid.
func ValidateImageSignatureUpdate(newImageSignature, oldImageSignature *imageapi.ImageSignature) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newImageSignature.ObjectMeta, &oldImageSignature.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateImageSignature(newImageSignature)...)

	if newImageSignature.Type != oldImageSignature.Type {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("type"), "cannot change signature type"))
	}
	if !bytes.Equal(newImageSignature.Content, oldImageSignature.Content) {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("content"), "cannot change signature content"))
	}

	return allErrs
}

// ValidateImageStream tests required fields for an ImageStream.
func ValidateImageStream(stream *imageapi.ImageStream) field.ErrorList {
	result := validation.ValidateObjectMeta(&stream.ObjectMeta, true, ValidateImageStreamName, field.NewPath("metadata"))

	// Ensure we can generate a valid docker image repository from namespace/name
	if len(stream.Namespace+"/"+stream.Name) > reference.NameTotalLengthMax {
		result = append(result, field.Invalid(field.NewPath("metadata", "name"), stream.Name, fmt.Sprintf("'namespace/name' cannot be longer than %d characters", reference.NameTotalLengthMax)))
	}

	if len(stream.Spec.DockerImageRepository) != 0 {
		dockerImageRepositoryPath := field.NewPath("spec", "dockerImageRepository")
		if ref, err := imageapi.ParseDockerImageReference(stream.Spec.DockerImageRepository); err != nil {
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
		path := field.NewPath("spec", "tags").Key(tag)
		result = append(result, ValidateImageStreamTagReference(tagRef, path)...)
	}
	for tag, history := range stream.Status.Tags {
		for i, tagEvent := range history.Items {
			if len(tagEvent.DockerImageReference) == 0 {
				result = append(result, field.Required(field.NewPath("status", "tags").Key(tag).Child("items").Index(i).Child("dockerImageReference"), ""))
			}
		}
	}

	return result
}

// ValidateImageStreamTagReference ensures that a given tag reference is valid.
func ValidateImageStreamTagReference(tagRef imageapi.TagReference, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList
	if tagRef.From != nil {
		if len(tagRef.From.Name) == 0 {
			errs = append(errs, field.Required(fldPath.Child("from", "name"), ""))
		}
		switch tagRef.From.Kind {
		case "DockerImage":
			if ref, err := imageapi.ParseDockerImageReference(tagRef.From.Name); err != nil && len(tagRef.From.Name) > 0 {
				errs = append(errs, field.Invalid(fldPath.Child("from", "name"), tagRef.From.Name, err.Error()))
			} else if len(ref.ID) > 0 && tagRef.ImportPolicy.Scheduled {
				errs = append(errs, field.Invalid(fldPath.Child("from", "name"), tagRef.From.Name, "only tags can be scheduled for import"))
			}
		case "ImageStreamImage", "ImageStreamTag":
			if tagRef.ImportPolicy.Scheduled {
				errs = append(errs, field.Invalid(fldPath.Child("importPolicy", "scheduled"), tagRef.ImportPolicy.Scheduled, "only tags pointing to Docker repositories may be scheduled for background import"))
			}
		default:
			errs = append(errs, field.Required(fldPath.Child("from", "kind"), "valid values are 'DockerImage', 'ImageStreamImage', 'ImageStreamTag'"))
		}
	}
	switch tagRef.ReferencePolicy.Type {
	case imageapi.SourceTagReferencePolicy, imageapi.LocalTagReferencePolicy:
	default:
		errs = append(errs, field.Invalid(fldPath.Child("referencePolicy", "type"), tagRef.ReferencePolicy.Type, fmt.Sprintf("valid values are %q, %q", imageapi.SourceTagReferencePolicy, imageapi.LocalTagReferencePolicy)))
	}
	return errs
}

func ValidateImageStreamUpdate(newStream, oldStream *imageapi.ImageStream) field.ErrorList {
	result := validation.ValidateObjectMetaUpdate(&newStream.ObjectMeta, &oldStream.ObjectMeta, field.NewPath("metadata"))
	result = append(result, ValidateImageStream(newStream)...)

	return result
}

// ValidateImageStreamStatusUpdate tests required fields for an ImageStream status update.
func ValidateImageStreamStatusUpdate(newStream, oldStream *imageapi.ImageStream) field.ErrorList {
	result := validation.ValidateObjectMetaUpdate(&newStream.ObjectMeta, &oldStream.ObjectMeta, field.NewPath("metadata"))
	return result
}

// ValidateImageStreamMapping tests required fields for an ImageStreamMapping.
func ValidateImageStreamMapping(mapping *imageapi.ImageStreamMapping) field.ErrorList {
	result := validation.ValidateObjectMeta(&mapping.ObjectMeta, true, path.ValidatePathSegmentName, field.NewPath("metadata"))

	hasRepository := len(mapping.DockerImageRepository) != 0
	hasName := len(mapping.Name) != 0
	switch {
	case hasRepository:
		if _, err := imageapi.ParseDockerImageReference(mapping.DockerImageRepository); err != nil {
			result = append(result, field.Invalid(field.NewPath("dockerImageRepository"), mapping.DockerImageRepository, err.Error()))
		}
	case hasName:
	default:
		result = append(result, field.Required(field.NewPath("name"), ""))
		result = append(result, field.Required(field.NewPath("dockerImageRepository"), ""))
	}

	if reasons := validation.ValidateNamespaceName(mapping.Namespace, false); len(reasons) != 0 {
		result = append(result, field.Invalid(field.NewPath("metadata", "namespace"), mapping.Namespace, strings.Join(reasons, ", ")))
	}
	if len(mapping.Tag) == 0 {
		result = append(result, field.Required(field.NewPath("tag"), ""))
	}
	if errs := validateImage(&mapping.Image, field.NewPath("image")); len(errs) != 0 {
		result = append(result, errs...)
	}
	return result
}

// ValidateImageStreamTag validates a mutation of an image stream tag, which can happen on PUT
func ValidateImageStreamTag(ist *imageapi.ImageStreamTag) field.ErrorList {
	result := validation.ValidateObjectMeta(&ist.ObjectMeta, true, path.ValidatePathSegmentName, field.NewPath("metadata"))
	if ist.Tag != nil {
		result = append(result, ValidateImageStreamTagReference(*ist.Tag, field.NewPath("tag"))...)
		if ist.Tag.Annotations != nil && !kapihelper.Semantic.DeepEqual(ist.Tag.Annotations, ist.ObjectMeta.Annotations) {
			result = append(result, field.Invalid(field.NewPath("tag", "annotations"), "<map>", "tag annotations must not be provided or must be equal to the object meta annotations"))
		}
	}

	return result
}

// ValidateImageStreamTagUpdate ensures that only the annotations of the IST have changed
func ValidateImageStreamTagUpdate(newIST, oldIST *imageapi.ImageStreamTag) field.ErrorList {
	result := validation.ValidateObjectMetaUpdate(&newIST.ObjectMeta, &oldIST.ObjectMeta, field.NewPath("metadata"))

	if newIST.Tag != nil {
		result = append(result, ValidateImageStreamTagReference(*newIST.Tag, field.NewPath("tag"))...)
		if newIST.Tag.Annotations != nil && !kapihelper.Semantic.DeepEqual(newIST.Tag.Annotations, newIST.ObjectMeta.Annotations) {
			result = append(result, field.Invalid(field.NewPath("tag", "annotations"), "<map>", "tag annotations must not be provided or must be equal to the object meta annotations"))
		}
	}

	// ensure that only tag and annotations have changed
	newISTCopy := *newIST
	oldISTCopy := *oldIST
	newISTCopy.Annotations, oldISTCopy.Annotations = nil, nil
	newISTCopy.Tag, oldISTCopy.Tag = nil, nil
	newISTCopy.LookupPolicy = oldISTCopy.LookupPolicy
	newISTCopy.Generation = oldISTCopy.Generation
	if !kapihelper.Semantic.Equalities.DeepEqual(&newISTCopy, &oldISTCopy) {
		//glog.Infof("objects differ: ", diff.ObjectDiff(oldISTCopy, newISTCopy))
		result = append(result, field.Invalid(field.NewPath("metadata"), "", "may not update fields other than metadata.annotations"))
	}

	return result
}

func ValidateRegistryAllowedForImport(path *field.Path, name, registryHost, registryPort string, allowedRegistries *serverapi.AllowedRegistries) field.ErrorList {
	errs := field.ErrorList{}
	if allowedRegistries == nil {
		return errs
	}
	allowedRegistriesForHumans := []string{}
	for _, registry := range *allowedRegistries {
		allowedRegistryHost, allowedRegistryPort := "", ""
		parts := strings.Split(registry.DomainName, ":")
		switch len(parts) {
		case 1:
			allowedRegistryHost = parts[0]
			if registry.Insecure {
				allowedRegistryPort = "80"
			} else {
				allowedRegistryPort = "443"
			}
		case 2:
			allowedRegistryHost, allowedRegistryPort = parts[0], parts[1]
		default:
			continue
		}
		if stringsutil.IsWildcardMatch(registryHost, allowedRegistryHost) && stringsutil.IsWildcardMatch(registryPort, allowedRegistryPort) {
			return errs
		}
		allowedRegistriesForHumans = append(allowedRegistriesForHumans, registry.DomainName)
	}
	return append(errs, field.Invalid(path, name,
		fmt.Sprintf("importing images from registry %q is forbidden, only images from %q are allowed", registryHost+":"+registryPort, strings.Join(allowedRegistriesForHumans, ","))))
}

func ValidateImageStreamImport(isi *imageapi.ImageStreamImport) field.ErrorList {
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
				errs = append(errs, field.Required(imagesPath.Index(i).Child("from", "name"), ""))
			} else {
				// The ParseDockerImageReference qualifies '*' as a wrong name.
				// The legacy clients use this character to look up imagestreams.
				// TODO: This should be removed in 1.6
				// See for more info: https://github.com/openshift/origin/pull/11774#issuecomment-258905994
				if spec.From.Name == "*" {
					continue
				}
				if ref, err := imageapi.ParseDockerImageReference(spec.From.Name); err != nil {
					errs = append(errs, field.Invalid(imagesPath.Index(i).Child("from", "name"), spec.From.Name, err.Error()))
				} else {
					if len(ref.ID) > 0 && spec.ImportPolicy.Scheduled {
						errs = append(errs, field.Invalid(imagesPath.Index(i).Child("from", "name"), spec.From.Name, "only tags can be scheduled for import"))
					}
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
				errs = append(errs, field.Required(repoPath.Child("from", "name"), "Docker image references require a name"))
			} else {
				if ref, err := imageapi.ParseDockerImageReference(from.Name); err != nil {
					errs = append(errs, field.Invalid(repoPath.Child("from", "name"), from.Name, err.Error()))
				} else {
					if len(ref.ID) > 0 || len(ref.Tag) > 0 {
						errs = append(errs, field.Invalid(repoPath.Child("from", "name"), from.Name, "you must specify an image repository, not a tag or ID"))
					}
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
