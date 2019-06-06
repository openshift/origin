package util

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/blang/semver"

	imagev1 "github.com/openshift/api/image/v1"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/internal/digest"
)

const (
	// DockerDefaultRegistry is the value for the registry when none was provided.
	DockerDefaultRegistry = "docker.io"
	// DockerDefaultV1Registry is the host name of the default v1 registry
	DockerDefaultV1Registry = "index." + DockerDefaultRegistry
	// DockerDefaultV2Registry is the host name of the default v2 registry
	DockerDefaultV2Registry = "registry-1." + DockerDefaultRegistry
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
		tag = imageapi.DefaultImageTag
	}

	// find the most recent tag event with an image reference
	t, ok := StatusHasTag(stream, tag)
	if ok {
		if len(t.Items) == 0 {
			return nil
		}
		return &t.Items[0]
	}

	return nil
}

// ParseDockerImageReference parses a Docker pull spec string into a
// DockerImageReference.
func ParseDockerImageReference(spec string) (imagev1.DockerImageReference, error) {
	ref, err := imagereference.Parse(spec)
	if err != nil {
		return imagev1.DockerImageReference{}, err
	}
	return imagev1.DockerImageReference{
		Registry:  ref.Registry,
		Namespace: ref.Namespace,
		Name:      ref.Name,
		Tag:       ref.Tag,
		ID:        ref.ID,
	}, nil
}

// ResolveLatestTaggedImage returns the appropriate pull spec for a given tag in
// the image stream, handling the tag's reference policy if necessary to return
// a resolved image. Callers that transform an ImageStreamTag into a pull spec
// should use this method instead of LatestTaggedImage.
func ResolveLatestTaggedImage(stream *imagev1.ImageStream, tag string) (string, bool) {
	if len(tag) == 0 {
		tag = imageapi.DefaultImageTag
	}
	return resolveTagReference(stream, tag, LatestTaggedImage(stream, tag))
}

// ResolveTagReference applies the tag reference rules for a stream, tag, and tag event for
// that tag. It returns true if the tag is
func resolveTagReference(stream *imagev1.ImageStream, tag string, latest *imagev1.TagEvent) (string, bool) {
	if latest == nil {
		return "", false
	}
	return resolveReferenceForTagEvent(stream, tag, latest), true
}

// ResolveReferenceForTagEvent applies the tag reference rules for a stream, tag, and tag event for
// that tag.
func resolveReferenceForTagEvent(stream *imagev1.ImageStream, tag string, latest *imagev1.TagEvent) string {
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

		// we must use imageapi's helper since we're calling Exact later on, which produces string
		ref, err := imagereference.Parse(local)
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

// ParseImageStreamImageName splits a string into its name component and ID component, and returns an error
// if the string is not in the right form.
func ParseImageStreamImageName(input string) (name string, id string, err error) {
	segments := strings.SplitN(input, "@", 3)
	switch len(segments) {
	case 2:
		name = segments[0]
		id = segments[1]
		if len(name) == 0 || len(id) == 0 {
			err = fmt.Errorf("image stream image name %q must have a name and ID", input)
		}
	default:
		err = fmt.Errorf("expected exactly one @ in the isimage name %q", input)
	}
	return
}

// ParseImageStreamTagName splits a string into its name component and tag component, and returns an error
// if the string is not in the right form.
func ParseImageStreamTagName(istag string) (name string, tag string, err error) {
	if strings.Contains(istag, "@") {
		err = fmt.Errorf("%q is an image stream image, not an image stream tag", istag)
		return
	}
	segments := strings.SplitN(istag, ":", 3)
	switch len(segments) {
	case 2:
		name = segments[0]
		tag = segments[1]
		if len(name) == 0 || len(tag) == 0 {
			err = fmt.Errorf("image stream tag name %q must have a name and a tag", istag)
		}
	default:
		err = fmt.Errorf("expected exactly one : delimiter in the istag %q", istag)
	}
	return
}

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
		ref, err := ParseDockerImageReference(stream.Status.DockerImageRepository)
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

// IsRegistryDockerHub returns true if the given registry name belongs to
// Docker hub.
func IsRegistryDockerHub(registry string) bool {
	switch registry {
	case DockerDefaultRegistry, DockerDefaultV1Registry, DockerDefaultV2Registry:
		return true
	default:
		return false
	}
}

// DockerImageReferenceString converts a DockerImageReference to a Docker pull spec
// (which implies a default namespace according to V1 Docker registry rules).
// Use DockerImageReferenceExact() if you want no defaulting.
func DockerImageReferenceString(r imagev1.DockerImageReference) string {
	if len(r.Namespace) == 0 && IsRegistryDockerHub(r.Registry) {
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
		if _, err := digest.ParseDigest(r.ID); err == nil {
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
			if DigestOrImageMatch(event.Image, imageID) &&
				(latestTagEvent == nil || latestTagEvent != nil && event.Created.After(latestTagEvent.Created.Time)) {
				latestTagEvent = &events.Items[i]
				latestTag = tag
			}
		}
	}
	return latestTag, latestTagEvent
}

// DigestOrImageMatch matches the digest in the image name.
func DigestOrImageMatch(image, imageID string) bool {
	if d, err := digest.ParseDigest(image); err == nil {
		return strings.HasPrefix(d.Hex(), imageID) || strings.HasPrefix(image, imageID)
	}
	return strings.HasPrefix(image, imageID)
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

var (
	reMinorSemantic  = regexp.MustCompile(`^[\d]+\.[\d]+$`)
	reMinorWithPatch = regexp.MustCompile(`^([\d]+\.[\d]+)-\w+$`)
)

type tagPriority int

const (
	// the "latest" tag
	tagPriorityLatest tagPriority = iota

	// a semantic minor version ("5.1", "v5.1", "v5.1-rc1")
	tagPriorityMinor

	// a full semantic version ("5.1.3-other", "v5.1.3-other")
	tagPriorityFull

	// other tags
	tagPriorityOther
)

type prioritizedTag struct {
	tag      string
	priority tagPriority
	semver   semver.Version
	prefix   string
}

func prioritizeTag(tag string) prioritizedTag {
	if tag == "latest" {
		return prioritizedTag{
			tag:      tag,
			priority: tagPriorityLatest,
		}
	}

	short := tag
	prefix := ""
	if strings.HasPrefix(tag, "v") {
		prefix = "v"
		short = tag[1:]
	}

	// 5.1.3
	if v, err := semver.Parse(short); err == nil {
		return prioritizedTag{
			tag:      tag,
			priority: tagPriorityFull,
			semver:   v,
			prefix:   prefix,
		}
	}

	// 5.1
	if reMinorSemantic.MatchString(short) {
		if v, err := semver.Parse(short + ".0"); err == nil {
			return prioritizedTag{
				tag:      tag,
				priority: tagPriorityMinor,
				semver:   v,
				prefix:   prefix,
			}
		}
	}

	// 5.1-rc1
	if match := reMinorWithPatch.FindStringSubmatch(short); match != nil {
		if v, err := semver.Parse(strings.Replace(short, match[1], match[1]+".0", 1)); err == nil {
			return prioritizedTag{
				tag:      tag,
				priority: tagPriorityMinor,
				semver:   v,
				prefix:   prefix,
			}
		}
	}

	// other
	return prioritizedTag{
		tag:      tag,
		priority: tagPriorityOther,
		prefix:   prefix,
	}
}

type prioritizedTags []prioritizedTag

func (t prioritizedTags) Len() int      { return len(t) }
func (t prioritizedTags) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t prioritizedTags) Less(i, j int) bool {
	if t[i].priority != t[j].priority {
		return t[i].priority < t[j].priority
	}

	if t[i].priority == tagPriorityOther {
		return t[i].tag < t[j].tag
	}

	cmp := t[i].semver.Compare(t[j].semver)
	if cmp > 0 { // the newer tag has a higher priority
		return true
	}
	return cmp == 0 && t[i].prefix < t[j].prefix
}

// PrioritizeTags orders a set of image tags with a few conventions:
//
// 1. the "latest" tag, if present, should be first
// 2. any tags that represent a semantic minor version ("5.1", "v5.1", "v5.1-rc1") should be next, in descending order
// 3. any tags that represent a full semantic version ("5.1.3-other", "v5.1.3-other") should be next, in descending order
// 4. any remaining tags should be sorted in lexicographic order
//
// The method updates the tags in place.
func PrioritizeTags(tags []string) {
	ptags := make(prioritizedTags, len(tags))
	for i, tag := range tags {
		ptags[i] = prioritizeTag(tag)
	}
	sort.Sort(ptags)
	for i, pt := range ptags {
		tags[i] = pt.tag
	}
}
