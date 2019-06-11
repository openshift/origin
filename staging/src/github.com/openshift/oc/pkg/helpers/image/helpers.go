package image

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
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
