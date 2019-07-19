package image

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	"github.com/openshift/library-go/pkg/image/reference"
)

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

		tagRef, ok := imageutil.SpecHasTag(stream, tag)
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

func HasAnnotationTag(tagRef *imagev1.TagReference, searchTag string) bool {
	for _, tag := range strings.Split(tagRef.Annotations["tags"], ",") {
		if tag == searchTag {
			return true
		}
	}
	return false
}

// ShortDockerImageID returns a short form of the provided DockerImage ID for display
func ShortDockerImageID(image *dockerv10.DockerImage, length int) string {
	id := image.ID
	if s, err := imageutil.ParseDigest(id); err == nil {
		id = s.Hex()
	}
	if len(id) > length {
		id = id[:length]
	}
	return id
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

// DockerImageReferenceString converts a DockerImageReference to a Docker pull spec
// (which implies a default namespace according to V1 container image registry rules).
// Use DockerImageReferenceExact() if you want no defaulting.
func DockerImageReferenceString(r imagev1.DockerImageReference) string {
	if len(r.Namespace) == 0 && reference.IsRegistryDockerHub(r.Registry) {
		r.Namespace = "library"
	}
	return DockerImageReferenceExact(r)
}
