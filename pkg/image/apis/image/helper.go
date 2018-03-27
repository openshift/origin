package image

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/blang/semver"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/image/apis/image/internal/digest"
)

const (
	// DockerDefaultNamespace is the value for namespace when a single segment name is provided.
	DockerDefaultNamespace = "library"
	// DockerDefaultRegistry is the value for the registry when none was provided.
	DockerDefaultRegistry = "docker.io"
	// DockerDefaultV1Registry is the host name of the default v1 registry
	DockerDefaultV1Registry = "index." + DockerDefaultRegistry
	// DockerDefaultV2Registry is the host name of the default v2 registry
	DockerDefaultV2Registry = "registry-1." + DockerDefaultRegistry

	// TagReferenceAnnotationTagHidden indicates that a given TagReference is hidden from search results
	TagReferenceAnnotationTagHidden = "hidden"

	// ImportRegistryNotAllowed indicates that the image tag was not imported due to
	// untrusted registry.
	ImportRegistryNotAllowed = "registry is not allowed for import"
)

var errNoRegistryURLPathAllowed = errors.New("no path after <host>[:<port>] is allowed")
var errNoRegistryURLQueryAllowed = errors.New("no query arguments are allowed after <host>[:<port>]")
var errRegistryURLHostEmpty = errors.New("no host name specified")

// ErrImageStreamImportUnsupported is an error client receive when the import
// failed.
var ErrImageStreamImportUnsupported = errors.New("the server does not support directly importing images - create an image stream with tags or the dockerImageRepository field set")

// ErrCircularReference is an error when reference tag is circular.
var ErrCircularReference = errors.New("reference tag is circular")

// ErrNotFoundReference is an error when reference tag is not found.
var ErrNotFoundReference = errors.New("reference tag is not found")

// ErrCrossImageStreamReference is an error when reference tag points to another imagestream.
var ErrCrossImageStreamReference = errors.New("reference tag points to another imagestream")

// ErrInvalidReference is an error when reference tag is invalid.
var ErrInvalidReference = errors.New("reference tag is invalid")

// RegistryHostnameRetriever represents an interface for retrieving the hostname
// of internal and external registry.
type RegistryHostnameRetriever interface {
	InternalRegistryHostname() (string, bool)
	ExternalRegistryHostname() (string, bool)
}

// DefaultRegistryHostnameRetriever is a default implementation of
// RegistryHostnameRetriever.
// The first argument is a function that lazy-loads the value of
// OPENSHIFT_DEFAULT_REGISTRY environment variable which should be deprecated in
// future.
func DefaultRegistryHostnameRetriever(deprecatedDefaultRegistryEnvFn func() (string, bool), external, internal string) RegistryHostnameRetriever {
	return &defaultRegistryHostnameRetriever{
		deprecatedDefaultFn: deprecatedDefaultRegistryEnvFn,
		externalHostname:    external,
		internalHostname:    internal,
	}
}

type defaultRegistryHostnameRetriever struct {
	// deprecatedDefaultFn points to a function that will lazy-load the value of
	// OPENSHIFT_DEFAULT_REGISTRY.
	deprecatedDefaultFn func() (string, bool)
	internalHostname    string
	externalHostname    string
}

// InternalRegistryHostnameFn returns a function that can be used to lazy-load
// the internal Docker Registry hostname. If the master configuration properly
// InternalRegistryHostname is set, it will prefer that over the lazy-loaded
// environment variable 'OPENSHIFT_DEFAULT_REGISTRY'.
func (r *defaultRegistryHostnameRetriever) InternalRegistryHostname() (string, bool) {
	if len(r.internalHostname) > 0 {
		return r.internalHostname, true
	}
	if r.deprecatedDefaultFn != nil {
		return r.deprecatedDefaultFn()
	}
	return "", false
}

// ExternalRegistryHostnameFn returns a function that can be used to retrieve an
// external/public hostname of Docker Registry. External location can be
// configured in master config using 'ExternalRegistryHostname' property.
func (r *defaultRegistryHostnameRetriever) ExternalRegistryHostname() (string, bool) {
	return r.externalHostname, len(r.externalHostname) > 0
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

// ParseDockerImageReference parses a Docker pull spec string into a
// DockerImageReference.
func ParseDockerImageReference(spec string) (DockerImageReference, error) {
	var ref DockerImageReference

	namedRef, err := parseNamedDockerImageReference(spec)
	if err != nil {
		return ref, err
	}

	ref.Registry = namedRef.Registry
	ref.Namespace = namedRef.Namespace
	ref.Name = namedRef.Name
	ref.Tag = namedRef.Tag
	ref.ID = namedRef.ID

	return ref, nil
}

// Equal returns true if the other DockerImageReference is equivalent to the
// reference r. The comparison applies defaults to the Docker image reference,
// so that e.g., "foobar" equals "docker.io/library/foobar:latest".
func (r DockerImageReference) Equal(other DockerImageReference) bool {
	defaultedRef := r.DockerClientDefaults()
	otherDefaultedRef := other.DockerClientDefaults()
	return defaultedRef == otherDefaultedRef
}

// DockerClientDefaults sets the default values used by the Docker client.
func (r DockerImageReference) DockerClientDefaults() DockerImageReference {
	if len(r.Registry) == 0 {
		r.Registry = DockerDefaultRegistry
	}
	if len(r.Namespace) == 0 && IsRegistryDockerHub(r.Registry) {
		r.Namespace = DockerDefaultNamespace
	}
	if len(r.Tag) == 0 {
		r.Tag = DefaultImageTag
	}
	return r
}

// Minimal reduces a DockerImageReference to its minimalist form.
func (r DockerImageReference) Minimal() DockerImageReference {
	if r.Tag == DefaultImageTag {
		r.Tag = ""
	}
	return r
}

// AsRepository returns the reference without tags or IDs.
func (r DockerImageReference) AsRepository() DockerImageReference {
	r.Tag = ""
	r.ID = ""
	return r
}

// RepositoryName returns the registry relative name
func (r DockerImageReference) RepositoryName() string {
	r.Tag = ""
	r.ID = ""
	r.Registry = ""
	return r.Exact()
}

// RegistryHostPort returns the registry hostname and the port.
// If the port is not specified in the registry hostname we default to 443.
// This will also default to Docker client defaults if the registry hostname is empty.
func (r DockerImageReference) RegistryHostPort(insecure bool) (string, string) {
	registryHost := r.AsV2().DockerClientDefaults().Registry
	if strings.Contains(registryHost, ":") {
		hostname, port, _ := net.SplitHostPort(registryHost)
		return hostname, port
	}
	if insecure {
		return registryHost, "80"
	}
	return registryHost, "443"
}

// RepositoryName returns the registry relative name
func (r DockerImageReference) RegistryURL() *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   r.AsV2().Registry,
	}
}

// DaemonMinimal clears defaults that Docker assumes.
func (r DockerImageReference) DaemonMinimal() DockerImageReference {
	switch r.Registry {
	case DockerDefaultV1Registry, DockerDefaultV2Registry:
		r.Registry = DockerDefaultRegistry
	}
	if IsRegistryDockerHub(r.Registry) && r.Namespace == DockerDefaultNamespace {
		r.Namespace = ""
	}
	return r.Minimal()
}

func (r DockerImageReference) AsV2() DockerImageReference {
	switch r.Registry {
	case DockerDefaultV1Registry, DockerDefaultRegistry:
		r.Registry = DockerDefaultV2Registry
	}
	return r
}

// MostSpecific returns the most specific image reference that can be constructed from the
// current ref, preferring an ID over a Tag. Allows client code dealing with both tags and IDs
// to get the most specific reference easily.
func (r DockerImageReference) MostSpecific() DockerImageReference {
	if len(r.ID) == 0 {
		return r
	}
	if _, err := digest.ParseDigest(r.ID); err == nil {
		r.Tag = ""
		return r
	}
	if len(r.Tag) == 0 {
		r.Tag, r.ID = r.ID, ""
		return r
	}
	return r
}

// NameString returns the name of the reference with its tag or ID.
func (r DockerImageReference) NameString() string {
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

// Exact returns a string representation of the set fields on the DockerImageReference
func (r DockerImageReference) Exact() string {
	name := r.NameString()
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

// String converts a DockerImageReference to a Docker pull spec (which implies a default namespace
// according to V1 Docker registry rules). Use Exact() if you want no defaulting.
func (r DockerImageReference) String() string {
	if len(r.Namespace) == 0 && IsRegistryDockerHub(r.Registry) {
		r.Namespace = DockerDefaultNamespace
	}
	return r.Exact()
}

// SplitImageStreamTag turns the name of an ImageStreamTag into Name and Tag.
// It returns false if the tag was not properly specified in the name.
func SplitImageStreamTag(nameAndTag string) (name string, tag string, ok bool) {
	parts := strings.SplitN(nameAndTag, ":", 2)
	name = parts[0]
	if len(parts) > 1 {
		tag = parts[1]
	}
	if len(tag) == 0 {
		tag = DefaultImageTag
	}
	return name, tag, len(parts) == 2
}

// SplitImageStreamImage turns the name of an ImageStreamImage into Name and ID.
// It returns false if the ID was not properly specified in the name.
func SplitImageStreamImage(nameAndID string) (name string, id string, ok bool) {
	parts := strings.SplitN(nameAndID, "@", 2)
	name = parts[0]
	if len(parts) > 1 {
		id = parts[1]
	}
	return name, id, len(parts) == 2
}

// JoinImageStreamTag turns a name and tag into the name of an ImageStreamTag
func JoinImageStreamTag(name, tag string) string {
	if len(tag) == 0 {
		tag = DefaultImageTag
	}
	return fmt.Sprintf("%s:%s", name, tag)
}

// JoinImageStreamImage creates a name for image stream image object from an image stream name and an id.
func JoinImageStreamImage(name, id string) string {
	return fmt.Sprintf("%s@%s", name, id)
}

// NormalizeImageStreamTag normalizes an image stream tag by defaulting to 'latest'
// if no tag has been specified.
func NormalizeImageStreamTag(name string) string {
	stripped, tag, ok := SplitImageStreamTag(name)
	if !ok {
		// Default to latest
		return JoinImageStreamTag(stripped, tag)
	}
	return name
}

// DockerImageReferenceForStream returns a DockerImageReference that represents
// the ImageStream or false, if no valid reference exists.
func DockerImageReferenceForStream(stream *ImageStream) (DockerImageReference, error) {
	spec := stream.Status.DockerImageRepository
	if len(spec) == 0 {
		spec = stream.Spec.DockerImageRepository
	}
	if len(spec) == 0 {
		return DockerImageReference{}, fmt.Errorf("no possible pull spec for %s/%s", stream.Namespace, stream.Name)
	}
	return ParseDockerImageReference(spec)
}

// FollowTagReference walks through the defined tags on a stream, following any referential tags in the stream.
// Will return multiple if the tag had at least reference, and ref and finalTag will be the last tag seen.
// If an invalid reference is found, err will be returned.
func FollowTagReference(stream *ImageStream, tag string) (finalTag string, ref *TagReference, multiple bool, err error) {
	seen := sets.NewString()
	for {
		if seen.Has(tag) {
			// circular reference
			return tag, nil, multiple, ErrCircularReference
		}
		seen.Insert(tag)

		tagRef, ok := stream.Spec.Tags[tag]
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
			name, tagref, ok := SplitImageStreamTag(tagRef.From.Name)
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

// LatestImageTagEvent returns the most recent TagEvent and the tag for the specified
// image.
func LatestImageTagEvent(stream *ImageStream, imageID string) (string, *TagEvent) {
	var (
		latestTagEvent *TagEvent
		latestTag      string
	)
	for tag, events := range stream.Status.Tags {
		if len(events.Items) == 0 {
			continue
		}
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

// LatestTaggedImage returns the most recent TagEvent for the specified image
// repository and tag. Will resolve lookups for the empty tag. Returns nil
// if tag isn't present in stream.status.tags.
func LatestTaggedImage(stream *ImageStream, tag string) *TagEvent {
	if len(tag) == 0 {
		tag = DefaultImageTag
	}
	// find the most recent tag event with an image reference
	if stream.Status.Tags != nil {
		if history, ok := stream.Status.Tags[tag]; ok {
			if len(history.Items) == 0 {
				return nil
			}
			return &history.Items[0]
		}
	}

	return nil
}

// ResolveLatestTaggedImage returns the appropriate pull spec for a given tag in
// the image stream, handling the tag's reference policy if necessary to return
// a resolved image. Callers that transform an ImageStreamTag into a pull spec
// should use this method instead of LatestTaggedImage.
func ResolveLatestTaggedImage(stream *ImageStream, tag string) (string, bool) {
	if len(tag) == 0 {
		tag = DefaultImageTag
	}
	return ResolveTagReference(stream, tag, LatestTaggedImage(stream, tag))
}

// ResolveTagReference applies the tag reference rules for a stream, tag, and tag event for
// that tag. It returns true if the tag is
func ResolveTagReference(stream *ImageStream, tag string, latest *TagEvent) (string, bool) {
	if latest == nil {
		return "", false
	}
	return ResolveReferenceForTagEvent(stream, tag, latest), true
}

// ResolveReferenceForTagEvent applies the tag reference rules for a stream, tag, and tag event for
// that tag.
func ResolveReferenceForTagEvent(stream *ImageStream, tag string, latest *TagEvent) string {
	// retrieve spec policy - if not found, we use the latest spec
	ref, ok := stream.Spec.Tags[tag]
	if !ok {
		return latest.DockerImageReference
	}

	switch ref.ReferencePolicy.Type {
	// the local reference policy attempts to use image pull through on the integrated
	// registry if possible
	case LocalTagReferencePolicy:
		local := stream.Status.DockerImageRepository
		if len(local) == 0 || len(latest.Image) == 0 {
			// fallback to the originating reference if no local docker registry defined or we
			// lack an image ID
			return latest.DockerImageReference
		}

		ref, err := ParseDockerImageReference(local)
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

// DockerImageReferenceForImage returns the docker reference for specified image. Assuming
// the image stream contains the image and the image has corresponding tag, this function
// will try to find this tag and take the reference policy into the account.
// If the image stream does not reference the image or the image does not have
// corresponding tag event, this function will return false.
func DockerImageReferenceForImage(stream *ImageStream, imageID string) (string, bool) {
	tag, event := LatestImageTagEvent(stream, imageID)
	if len(tag) == 0 {
		return "", false
	}
	ref, ok := stream.Spec.Tags[tag]
	if !ok {
		return event.DockerImageReference, true
	}
	switch ref.ReferencePolicy.Type {
	case LocalTagReferencePolicy:
		ref, err := ParseDockerImageReference(stream.Status.DockerImageRepository)
		if err != nil {
			return event.DockerImageReference, true
		}
		ref.Tag = ""
		ref.ID = event.Image
		return ref.Exact(), true
	default:
		return event.DockerImageReference, true
	}
}

// DifferentTagEvent returns true if the supplied tag event matches the current stream tag event.
// Generation is not compared.
func DifferentTagEvent(stream *ImageStream, tag string, next TagEvent) bool {
	tags, ok := stream.Status.Tags[tag]
	if !ok || len(tags.Items) == 0 {
		return true
	}
	previous := &tags.Items[0]
	sameRef := previous.DockerImageReference == next.DockerImageReference
	sameImage := previous.Image == next.Image
	return !(sameRef && sameImage)
}

// DifferentTagEvent compares the generation on tag's spec vs its status.
// Returns if spec generation is newer than status one.
func DifferentTagGeneration(stream *ImageStream, tag string) bool {
	specTag, ok := stream.Spec.Tags[tag]
	if !ok || specTag.Generation == nil {
		return true
	}
	statusTag, ok := stream.Status.Tags[tag]
	if !ok || len(statusTag.Items) == 0 {
		return true
	}
	return *specTag.Generation > statusTag.Items[0].Generation
}

// AddTagEventToImageStream attempts to update the given image stream with a tag event. It will
// collapse duplicate entries - returning true if a change was made or false if no change
// occurred. Any successful tag resets the status field.
func AddTagEventToImageStream(stream *ImageStream, tag string, next TagEvent) bool {
	if stream.Status.Tags == nil {
		stream.Status.Tags = make(map[string]TagEventList)
	}

	tags, ok := stream.Status.Tags[tag]
	if !ok || len(tags.Items) == 0 {
		stream.Status.Tags[tag] = TagEventList{Items: []TagEvent{next}}
		return true
	}

	previous := &tags.Items[0]

	sameRef := previous.DockerImageReference == next.DockerImageReference
	sameImage := previous.Image == next.Image
	sameGen := previous.Generation == next.Generation

	switch {
	// shouldn't change the tag
	case sameRef && sameImage && sameGen:
		return false

	case sameImage && sameRef:
		// collapse the tag
	case sameRef:
		previous.Image = next.Image
	case sameImage:
		previous.DockerImageReference = next.DockerImageReference
	default:
		// shouldn't collapse the tag
		tags.Conditions = nil
		tags.Items = append([]TagEvent{next}, tags.Items...)
		stream.Status.Tags[tag] = tags
		return true
	}
	previous.Generation = next.Generation
	tags.Conditions = nil
	stream.Status.Tags[tag] = tags
	return true
}

// UpdateChangedTrackingTags identifies any tags in the status that have changed and
// ensures any referenced tracking tags are also updated. It returns the number of
// updates applied.
func UpdateChangedTrackingTags(new, old *ImageStream) int {
	changes := 0
	for newTag, newImages := range new.Status.Tags {
		if len(newImages.Items) == 0 {
			continue
		}
		if old != nil {
			oldImages := old.Status.Tags[newTag]
			changed, deleted := tagsChanged(newImages.Items, oldImages.Items)
			if !changed || deleted {
				continue
			}
		}
		changes += UpdateTrackingTags(new, newTag, newImages.Items[0])
	}
	return changes
}

// tagsChanged returns true if the two lists differ, and if the newer list is empty
// then deleted is returned true as well.
func tagsChanged(new, old []TagEvent) (changed bool, deleted bool) {
	switch {
	case len(old) == 0 && len(new) == 0:
		return false, false
	case len(new) == 0:
		return true, true
	case len(old) == 0:
		return true, false
	default:
		return new[0] != old[0], false
	}
}

// UpdateTrackingTags sets updatedImage as the most recent TagEvent for all tags
// in stream.spec.tags that have from.kind = "ImageStreamTag" and the tag in from.name
// = updatedTag. from.name may be either <tag> or <stream name>:<tag>. For now, only
// references to tags in the current stream are supported.
//
// For example, if stream.spec.tags[latest].from.name = 2.0, whenever an image is pushed
// to this stream with the tag 2.0, status.tags[latest].items[0] will also be updated
// to point at the same image that was just pushed for 2.0.
//
// Returns the number of tags changed.
func UpdateTrackingTags(stream *ImageStream, updatedTag string, updatedImage TagEvent) int {
	updated := 0
	glog.V(5).Infof("UpdateTrackingTags: stream=%s/%s, updatedTag=%s, updatedImage.dockerImageReference=%s, updatedImage.image=%s", stream.Namespace, stream.Name, updatedTag, updatedImage.DockerImageReference, updatedImage.Image)
	for specTag, tagRef := range stream.Spec.Tags {
		glog.V(5).Infof("Examining spec tag %q, tagRef=%#v", specTag, tagRef)

		// no from
		if tagRef.From == nil {
			glog.V(5).Infof("tagRef.From is nil, skipping")
			continue
		}

		// wrong kind
		if tagRef.From.Kind != "ImageStreamTag" {
			glog.V(5).Infof("tagRef.Kind %q isn't ImageStreamTag, skipping", tagRef.From.Kind)
			continue
		}

		tagRefNamespace := tagRef.From.Namespace
		if len(tagRefNamespace) == 0 {
			tagRefNamespace = stream.Namespace
		}

		// different namespace
		if tagRefNamespace != stream.Namespace {
			glog.V(5).Infof("tagRefNamespace %q doesn't match stream namespace %q - skipping", tagRefNamespace, stream.Namespace)
			continue
		}

		tag := ""
		tagRefName := ""
		if strings.Contains(tagRef.From.Name, ":") {
			// <stream>:<tag>
			ok := true
			tagRefName, tag, ok = SplitImageStreamTag(tagRef.From.Name)
			if !ok {
				glog.V(5).Infof("tagRefName %q contains invalid reference - skipping", tagRef.From.Name)
				continue
			}
		} else {
			// <tag> (this stream)
			// TODO: this is probably wrong - we should require ":<tag>", but we can't break old clients
			tagRefName = stream.Name
			tag = tagRef.From.Name
		}

		glog.V(5).Infof("tagRefName=%q, tag=%q", tagRefName, tag)

		// different stream
		if tagRefName != stream.Name {
			glog.V(5).Infof("tagRefName %q doesn't match stream name %q - skipping", tagRefName, stream.Name)
			continue
		}

		// different tag
		if tag != updatedTag {
			glog.V(5).Infof("tag %q doesn't match updated tag %q - skipping", tag, updatedTag)
			continue
		}

		if AddTagEventToImageStream(stream, specTag, updatedImage) {
			glog.V(5).Infof("stream updated")
			updated++
		}
	}
	return updated
}

// DigestOrImageMatch matches the digest in the image name.
func DigestOrImageMatch(image, imageID string) bool {
	if d, err := digest.ParseDigest(image); err == nil {
		return strings.HasPrefix(d.Hex(), imageID) || strings.HasPrefix(image, imageID)
	}
	return strings.HasPrefix(image, imageID)
}

// ResolveImageID returns latest TagEvent for specified imageID and an error if
// there's more than one image matching the ID or when one does not exist.
func ResolveImageID(stream *ImageStream, imageID string) (*TagEvent, error) {
	var event *TagEvent
	set := sets.NewString()
	for _, history := range stream.Status.Tags {
		for i := range history.Items {
			tagging := &history.Items[i]
			if DigestOrImageMatch(tagging.Image, imageID) {
				event = tagging
				set.Insert(tagging.Image)
			}
		}
	}
	switch len(set) {
	case 1:
		return &TagEvent{
			Created:              metav1.Now(),
			DockerImageReference: event.DockerImageReference,
			Image:                event.Image,
		}, nil
	case 0:
		return nil, kerrors.NewNotFound(Resource("imagestreamimage"), imageID)
	default:
		return nil, kerrors.NewConflict(Resource("imagestreamimage"), imageID, fmt.Errorf("multiple images match the prefix %q: %s", imageID, strings.Join(set.List(), ", ")))
	}
}

// MostAccuratePullSpec returns a docker image reference that uses the current ID if possible, the current tag otherwise, and
// returns false if the reference if the spec could not be parsed. The returned spec has all client defaults applied.
func MostAccuratePullSpec(pullSpec string, id, tag string) (string, bool) {
	ref, err := ParseDockerImageReference(pullSpec)
	if err != nil {
		return pullSpec, false
	}
	if len(id) > 0 {
		ref.ID = id
	}
	if len(tag) > 0 {
		ref.Tag = tag
	}
	return ref.MostSpecific().Exact(), true
}

// ShortDockerImageID returns a short form of the provided DockerImage ID for display
func ShortDockerImageID(image *DockerImage, length int) string {
	id := image.ID
	if s, err := digest.ParseDigest(id); err == nil {
		id = s.Hex()
	}
	if len(id) > length {
		id = id[:length]
	}
	return id
}

// HasTagCondition returns true if the specified image stream tag has a condition with the same type, status, and
// reason (does not check generation, date, or message).
func HasTagCondition(stream *ImageStream, tag string, condition TagEventCondition) bool {
	for _, existing := range stream.Status.Tags[tag].Conditions {
		if condition.Type == existing.Type && condition.Status == existing.Status && condition.Reason == existing.Reason {
			return true
		}
	}
	return false
}

// SetTagConditions applies the specified conditions to the status of the given tag.
func SetTagConditions(stream *ImageStream, tag string, conditions ...TagEventCondition) {
	tagEvents := stream.Status.Tags[tag]
	tagEvents.Conditions = conditions
	if stream.Status.Tags == nil {
		stream.Status.Tags = make(map[string]TagEventList)
	}
	stream.Status.Tags[tag] = tagEvents
}

// LatestObservedTagGeneration returns the generation value for the given tag that has been observed by the controller
// monitoring the image stream. If the tag has not been observed, the generation is zero.
func LatestObservedTagGeneration(stream *ImageStream, tag string) int64 {
	tagEvents, ok := stream.Status.Tags[tag]
	if !ok {
		return 0
	}

	// find the most recent generation
	lastGen := int64(0)
	if items := tagEvents.Items; len(items) > 0 {
		tagEvent := items[0]
		if tagEvent.Generation > lastGen {
			lastGen = tagEvent.Generation
		}
	}
	for _, condition := range tagEvents.Conditions {
		if condition.Type != ImportSuccess {
			continue
		}
		if condition.Generation > lastGen {
			lastGen = condition.Generation
		}
		break
	}
	return lastGen
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
	if tag == DefaultImageTag {
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

func LabelForStream(stream *ImageStream) string {
	return fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)
}

// JoinImageSignatureName joins image name and custom signature name into one string with @ separator.
func JoinImageSignatureName(imageName, signatureName string) (string, error) {
	if len(imageName) == 0 {
		return "", fmt.Errorf("imageName may not be empty")
	}
	if len(signatureName) == 0 {
		return "", fmt.Errorf("signatureName may not be empty")
	}
	if strings.Count(imageName, "@") > 0 || strings.Count(signatureName, "@") > 0 {
		return "", fmt.Errorf("neither imageName nor signatureName can contain '@'")
	}
	return fmt.Sprintf("%s@%s", imageName, signatureName), nil
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

// IndexOfImageSignatureByName returns an index of signature identified by name in the image if present. It
// returns -1 otherwise.
func IndexOfImageSignatureByName(signatures []ImageSignature, name string) int {
	for i := range signatures {
		if signatures[i].Name == name {
			return i
		}
	}
	return -1
}

// IndexOfImageSignature returns index of signature identified by type and blob in the image if present. It
// returns -1 otherwise.
func IndexOfImageSignature(signatures []ImageSignature, sType string, sContent []byte) int {
	for i := range signatures {
		if signatures[i].Type == sType && bytes.Equal(signatures[i].Content, sContent) {
			return i
		}
	}
	return -1
}

func (tagref TagReference) HasAnnotationTag(searchTag string) bool {
	for _, tag := range strings.Split(tagref.Annotations["tags"], ",") {
		if tag == searchTag {
			return true
		}
	}
	return false
}

// ValidateRegistryURL returns error if the given input is not a valid registry URL. The url may be prefixed
// with http:// or https:// schema. It may not contain any path or query after the host:[port].
func ValidateRegistryURL(registryURL string) error {
	var (
		u     *url.URL
		err   error
		parts = strings.SplitN(registryURL, "://", 2)
	)

	switch len(parts) {
	case 2:
		u, err = url.Parse(registryURL)
		if err != nil {
			return err
		}
		switch u.Scheme {
		case "http", "https":
		default:
			return fmt.Errorf("unsupported scheme: %s", u.Scheme)
		}
	case 1:
		u, err = url.Parse("https://" + registryURL)
		if err != nil {
			return err
		}
	}
	if len(u.Path) > 0 && u.Path != "/" {
		return errNoRegistryURLPathAllowed
	}
	if len(u.RawQuery) > 0 {
		return errNoRegistryURLQueryAllowed
	}
	if len(u.Host) == 0 {
		return errRegistryURLHostEmpty
	}
	return nil
}
