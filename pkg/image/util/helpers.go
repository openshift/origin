package util

import (
	"fmt"
	"strings"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
)

// DockerImageReferenceForImage returns the docker reference for specified image. Assuming
// the image stream contains the image and the image has corresponding tag, this function
// will try to find this tag and take the reference policy into the account.
// If the image stream does not reference the image or the image does not have
// corresponding tag event, this function will return false.
func DockerImageReferenceForImage(stream *imagev1.ImageStream, imageID string) (string, bool) {
	tag, event := latestImageTagEvent(stream, imageID)
	if len(tag) == 0 {
		return "", false
	}
	var ref *imagev1.TagReference
	for _, t := range stream.Spec.Tags {
		if t.Name == tag {
			ref = &t
			break
		}
	}
	if ref == nil {
		return event.DockerImageReference, true
	}
	switch ref.ReferencePolicy.Type {
	case imagev1.LocalTagReferencePolicy:
		ref, err := imageutil.ParseDockerImageReference(stream.Status.DockerImageRepository)
		if err != nil {
			return event.DockerImageReference, true
		}
		ref.Tag = ""
		ref.ID = event.Image
		return DockerImageReferenceExact(ref), true
	default:
		return event.DockerImageReference, true
	}
}

// DockerImageReferenceString converts a DockerImageReference to a Docker pull spec
// (which implies a default namespace according to V1 Docker registry rules).
// Use DockerImageReferenceExact() if you want no defaulting.
func DockerImageReferenceString(r imagev1.DockerImageReference) string {
	if len(r.Namespace) == 0 && imagereference.IsRegistryDockerHub(r.Registry) {
		r.Namespace = "library"
	}
	return DockerImageReferenceExact(r)
}

// DockerImageReferenceNameString returns the name of the reference with its tag or ID.
func DockerImageReferenceNameString(r imagev1.DockerImageReference) string {
	switch {
	case len(r.Name) == 0:
		return ""
	case len(r.Tag) > 0:
		return r.Name + ":" + r.Tag
	case len(r.ID) > 0:
		var ref string
		if _, err := imageutil.ParseDigest(r.ID); err == nil {
			// if it parses as a digest, its v2 pull by id
			ref = "@" + r.ID
		} else {
			// if it doesn't parse as a digest, it's presumably a v1 registry by-id tag
			ref = ":" + r.ID
		}
		return r.Name + ref
	default:
		return r.Name
	}
}

// DockerImageReferenceExact returns a string representation of the set fields on the DockerImageReference
func DockerImageReferenceExact(r imagev1.DockerImageReference) string {
	name := DockerImageReferenceNameString(r)
	if len(name) == 0 {
		return name
	}
	s := r.Registry
	if len(s) > 0 {
		s += "/"
	}
	if len(r.Namespace) != 0 {
		s += r.Namespace + "/"
	}
	return s + name
}

// LatestImageTagEvent returns the most recent TagEvent and the tag for the specified
// image.
// Copied from v3.7 github.com/openshift/origin/pkg/image/apis/image/v1/helpers.go
func latestImageTagEvent(stream *imagev1.ImageStream, imageID string) (string, *imagev1.TagEvent) {
	var (
		latestTagEvent *imagev1.TagEvent
		latestTag      string
	)
	for _, events := range stream.Status.Tags {
		if len(events.Items) == 0 {
			continue
		}
		tag := events.Tag
		for i, event := range events.Items {
			if imageutil.DigestOrImageMatch(event.Image, imageID) &&
				(latestTagEvent == nil || latestTagEvent != nil && event.Created.After(latestTagEvent.Created.Time)) {
				latestTagEvent = &events.Items[i]
				latestTag = tag
			}
		}
	}
	return latestTag, latestTagEvent
}

// SplitImageSignatureName splits given signature name into image name and signature name.
func SplitImageSignatureName(imageSignatureName string) (imageName, signatureName string, err error) {
	segments := strings.Split(imageSignatureName, "@")
	switch len(segments) {
	case 2:
		signatureName = segments[1]
		imageName = segments[0]
		if len(imageName) == 0 || len(signatureName) == 0 {
			err = fmt.Errorf("image signature name %q must have an image name and signature name", imageSignatureName)
		}
	default:
		err = fmt.Errorf("expected exactly one @ in the image signature name %q", imageSignatureName)
	}
	return
}
