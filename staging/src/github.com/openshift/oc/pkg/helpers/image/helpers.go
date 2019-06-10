package image

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	"github.com/openshift/library-go/pkg/image/reference"
)

// StatusHasTag returns named tag from image stream's status and boolean whether one was found.
func StatusHasTag(stream *imagev1.ImageStream, name string) (imagev1.NamedTagEventList, bool) {
	for _, tag := range stream.Status.Tags {
		if tag.Tag == name {
			return tag, true
		}
	}
	return imagev1.NamedTagEventList{}, false
}

// SpecHasTag returns named tag from image stream's spec and boolean whether one was found.
func SpecHasTag(stream *imagev1.ImageStream, name string) (imagev1.TagReference, bool) {
	for _, tag := range stream.Spec.Tags {
		if tag.Name == name {
			return tag, true
		}
	}
	return imagev1.TagReference{}, false
}

// LatestTaggedImage returns the most recent TagEvent for the specified image
// repository and tag. Will resolve lookups for the empty tag. Returns nil
// if tag isn't present in stream.status.tags.
func LatestTaggedImage(stream *imagev1.ImageStream, tag string) *imagev1.TagEvent {
	if len(tag) == 0 {
		tag = imagev1.DefaultImageTag
	}
	history, ok := StatusHasTag(stream, tag)
	if !ok || len(history.Items) == 0 {
		return nil
	}
	return &history.Items[0]
}

// ResolveLatestTaggedImage returns the appropriate pull spec for a given tag in
// the image stream, handling the tag's reference policy if necessary to return
// a resolved image. Callers that transform an ImageStreamTag into a pull spec
// should use this method instead of LatestTaggedImage.
func ResolveLatestTaggedImage(stream *imagev1.ImageStream, tag string) (string, bool) {
	if len(tag) == 0 {
		tag = imagev1.DefaultImageTag
	}
	return resolveTagReference(stream, tag, LatestTaggedImage(stream, tag))
}

// ResolveTagReference applies the tag reference rules for a stream, tag, and tag event for
// that tag. It returns true if the tag is
func resolveTagReference(stream *imagev1.ImageStream, tag string, latest *imagev1.TagEvent) (string, bool) {
	if latest == nil {
		return "", false
	}
	return ResolveReferenceForTagEvent(stream, tag, latest), true
}

// ResolveReferenceForTagEvent applies the tag reference rules for a stream, tag, and tag event for
// that tag.
func ResolveReferenceForTagEvent(stream *imagev1.ImageStream, tag string, latest *imagev1.TagEvent) string {
	// retrieve spec policy - if not found, we use the latest spec
	ref, ok := SpecHasTag(stream, tag)
	if !ok {
		return latest.DockerImageReference
	}

	switch ref.ReferencePolicy.Type {
	// the local reference policy attempts to use image pull through on the integrated
	// registry if possible
	case imagev1.LocalTagReferencePolicy:
		local := stream.Status.DockerImageRepository
		if len(local) == 0 || len(latest.Image) == 0 {
			// fallback to the originating reference if no local docker registry defined or we
			// lack an image ID
			return latest.DockerImageReference
		}

		ref, err := reference.Parse(local)
		if err != nil {
			// fallback to the originating reference if the reported local repository spec is not valid
			return latest.DockerImageReference
		}

		// create a local pullthrough URL
		ref.Tag = ""
		ref.ID = latest.Image
		return ref.Exact()

	// the default policy is to use the originating image
	default:
		return latest.DockerImageReference
	}
}

// FollowTagReference walks through the defined tags on a stream, following any referential tags in the stream.
// Will return multiple if the tag had at least reference, and ref and finalTag will be the last tag seen.
// If an invalid reference is found, err will be returned.
func FollowTagReference(stream *imagev1.ImageStream, tag string) (string, *imagev1.TagReference, bool, error) {
	multiple := false
	seen := sets.NewString()
	for {
		if seen.Has(tag) {
			// circular reference
			return tag, nil, multiple, ErrCircularReference
		}
		seen.Insert(tag)

		tagRef, ok := SpecHasTag(stream, tag)
		if !ok {
			// no tag at the end of the rainbow
			return tag, nil, multiple, ErrNotFoundReference
		}
		if tagRef.From == nil || tagRef.From.Kind != "ImageStreamTag" {
			// terminating tag
			return tag, &tagRef, multiple, nil
		}

		if tagRef.From.Namespace != "" && tagRef.From.Namespace != stream.ObjectMeta.Namespace {
			return tag, nil, multiple, ErrCrossImageStreamReference
		}

		// The reference needs to be followed with two format patterns:
		// a) sameis:sometag and b) sometag
		if strings.Contains(tagRef.From.Name, ":") {
			name, tagref, ok := imageutil.SplitImageStreamTag(tagRef.From.Name)
			if !ok {
				return tag, nil, multiple, ErrInvalidReference
			}
			if name != stream.ObjectMeta.Name {
				// anotheris:sometag - this should not happen.
				return tag, nil, multiple, ErrCrossImageStreamReference
			}
			// sameis:sometag - follow the reference as sometag
			tag = tagref
		} else {
			// sometag - follow the reference
			tag = tagRef.From.Name
		}
		multiple = true
	}
}
