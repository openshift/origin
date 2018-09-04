package util

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/golang/glog"
	godigest "github.com/opencontainers/go-digest"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	dockerapi10 "github.com/openshift/origin/pkg/image/apis/image/docker10"
	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
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

func fillImageLayers(image *imageapi.Image, manifest dockerapi10.DockerImageManifest) error {
	if len(image.DockerImageLayers) != 0 {
		// DockerImageLayers is already filled by the registry.
		return nil
	}

	switch manifest.SchemaVersion {
	case 1:
		if len(manifest.History) != len(manifest.FSLayers) {
			return fmt.Errorf("the image %s (%s) has mismatched history and fslayer cardinality (%d != %d)", image.Name, image.DockerImageReference, len(manifest.History), len(manifest.FSLayers))
		}

		image.DockerImageLayers = make([]imageapi.ImageLayer, len(manifest.FSLayers))
		for i, obj := range manifest.History {
			layer := manifest.FSLayers[i]

			var size dockerapi10.DockerV1CompatibilityImageSize
			if err := json.Unmarshal([]byte(obj.DockerV1Compatibility), &size); err != nil {
				size.Size = 0
			}

			// reverse order of the layers: in schema1 manifests the
			// first layer is the youngest (base layers are at the
			// end), but we want to store layers in the Image resource
			// in order from the oldest to the youngest.
			revidx := (len(manifest.History) - 1) - i // n-1, n-2, ..., 1, 0

			image.DockerImageLayers[revidx].Name = layer.DockerBlobSum
			image.DockerImageLayers[revidx].LayerSize = size.Size
			image.DockerImageLayers[revidx].MediaType = schema1.MediaTypeManifestLayer
		}
	case 2:
		// The layer list is ordered starting from the base image (opposite order of schema1).
		// So, we do not need to change the order of layers.
		image.DockerImageLayers = make([]imageapi.ImageLayer, len(manifest.Layers))
		for i, layer := range manifest.Layers {
			image.DockerImageLayers[i].Name = layer.Digest
			image.DockerImageLayers[i].LayerSize = layer.Size
			image.DockerImageLayers[i].MediaType = layer.MediaType
		}
	default:
		return fmt.Errorf("unrecognized Docker image manifest schema %d for %q (%s)", manifest.SchemaVersion, image.Name, image.DockerImageReference)
	}

	if image.Annotations == nil {
		image.Annotations = map[string]string{}
	}
	image.Annotations[imageapi.DockerImageLayersOrderAnnotation] = imageapi.DockerImageLayersOrderAscending

	return nil
}

// InternalImageWithMetadata mutates the given image. It parses raw DockerImageManifest data stored in the image and
// fills its DockerImageMetadata and other fields.
func InternalImageWithMetadata(image *imageapi.Image) error {
	if len(image.DockerImageManifest) == 0 {
		return nil
	}

	ReorderImageLayers(image)

	if len(image.DockerImageLayers) > 0 && image.DockerImageMetadata.Size > 0 && len(image.DockerImageManifestMediaType) > 0 {
		glog.V(5).Infof("Image metadata already filled for %s", image.Name)
		return nil
	}

	manifest := dockerapi10.DockerImageManifest{}
	if err := json.Unmarshal([]byte(image.DockerImageManifest), &manifest); err != nil {
		return err
	}

	err := fillImageLayers(image, manifest)
	if err != nil {
		return err
	}

	switch manifest.SchemaVersion {
	case 1:
		image.DockerImageManifestMediaType = schema1.MediaTypeManifest

		if len(manifest.History) == 0 {
			// It should never have an empty history, but just in case.
			return fmt.Errorf("the image %s (%s) has a schema 1 manifest, but it doesn't have history", image.Name, image.DockerImageReference)
		}

		v1Metadata := dockerapi10.DockerV1CompatibilityImage{}
		if err := json.Unmarshal([]byte(manifest.History[0].DockerV1Compatibility), &v1Metadata); err != nil {
			return err
		}

		if err := imageapi.Convert_compatibility_to_api_DockerImage(&v1Metadata, &image.DockerImageMetadata); err != nil {
			return err
		}
	case 2:
		image.DockerImageManifestMediaType = schema2.MediaTypeManifest

		if len(image.DockerImageConfig) == 0 {
			return fmt.Errorf("dockerImageConfig must not be empty for manifest schema 2")
		}

		config := dockerapi10.DockerImageConfig{}
		if err := json.Unmarshal([]byte(image.DockerImageConfig), &config); err != nil {
			return fmt.Errorf("failed to parse dockerImageConfig: %v", err)
		}

		if err := imageapi.Convert_imageconfig_to_api_DockerImage(&config, &image.DockerImageMetadata); err != nil {
			return err
		}
		image.DockerImageMetadata.ID = manifest.Config.Digest

	default:
		return fmt.Errorf("unrecognized Docker image manifest schema %d for %q (%s)", manifest.SchemaVersion, image.Name, image.DockerImageReference)
	}

	layerSet := sets.NewString()
	if manifest.SchemaVersion == 2 {
		layerSet.Insert(manifest.Config.Digest)
		image.DockerImageMetadata.Size = int64(len(image.DockerImageConfig))
	} else {
		image.DockerImageMetadata.Size = 0
	}
	for _, layer := range image.DockerImageLayers {
		if layerSet.Has(layer.Name) {
			continue
		}
		layerSet.Insert(layer.Name)
		image.DockerImageMetadata.Size += layer.LayerSize
	}

	return nil
}

// ImageWithMetadata mutates the given image. It parses raw DockerImageManifest data stored in the image and
// fills its DockerImageMetadata and other fields.
// Copied from github.com/openshift/image-registry/pkg/origin-common/util/util.go
func ImageWithMetadata(image *imagev1.Image) error {
	// Check if the metadata are already filled in for this image.
	meta, hasMetadata := image.DockerImageMetadata.Object.(*dockerv10.DockerImage)
	if hasMetadata && meta.Size > 0 {
		return nil
	}

	version := image.DockerImageMetadataVersion
	if len(version) == 0 {
		version = "1.0"
	}

	obj := &dockerv10.DockerImage{}
	if len(image.DockerImageMetadata.Raw) != 0 {
		if err := json.Unmarshal(image.DockerImageMetadata.Raw, obj); err != nil {
			return err
		}
		image.DockerImageMetadata.Object = obj
	}

	image.DockerImageMetadataVersion = version

	return nil
}

// ReorderImageLayers mutates the given image. It reorders the layers in ascending order.
// Ascending order matches the order of layers in schema 2. Schema 1 has reversed (descending) order of layers.
func ReorderImageLayers(image *imageapi.Image) {
	if len(image.DockerImageLayers) == 0 {
		return
	}

	layersOrder, ok := image.Annotations[imageapi.DockerImageLayersOrderAnnotation]
	if !ok {
		switch image.DockerImageManifestMediaType {
		case schema1.MediaTypeManifest, schema1.MediaTypeSignedManifest:
			layersOrder = imageapi.DockerImageLayersOrderAscending
		case schema2.MediaTypeManifest:
			layersOrder = imageapi.DockerImageLayersOrderDescending
		default:
			return
		}
	}

	if layersOrder == imageapi.DockerImageLayersOrderDescending {
		// reverse order of the layers (lowest = 0, highest = i)
		for i, j := 0, len(image.DockerImageLayers)-1; i < j; i, j = i+1, j-1 {
			image.DockerImageLayers[i], image.DockerImageLayers[j] = image.DockerImageLayers[j], image.DockerImageLayers[i]
		}
	}

	if image.Annotations == nil {
		image.Annotations = map[string]string{}
	}

	image.Annotations[imageapi.DockerImageLayersOrderAnnotation] = imageapi.DockerImageLayersOrderAscending
}

// ManifestMatchesImage returns true if the provided manifest matches the name of the image.
func ManifestMatchesImage(image *imageapi.Image, newManifest []byte) (bool, error) {
	dgst, err := godigest.Parse(image.Name)
	if err != nil {
		return false, err
	}
	v := dgst.Verifier()
	var canonical []byte
	switch image.DockerImageManifestMediaType {
	case schema2.MediaTypeManifest:
		var m schema2.DeserializedManifest
		if err := json.Unmarshal(newManifest, &m); err != nil {
			return false, err
		}
		_, canonical, err = m.Payload()
		if err != nil {
			return false, err
		}
	case schema1.MediaTypeManifest, "":
		var m schema1.SignedManifest
		if err := json.Unmarshal(newManifest, &m); err != nil {
			return false, err
		}
		canonical = m.Canonical
	default:
		return false, fmt.Errorf("unsupported manifest mediatype: %s", image.DockerImageManifestMediaType)
	}
	if _, err := v.Write(canonical); err != nil {
		return false, err
	}
	return v.Verified(), nil
}

// ImageConfigMatchesImage returns true if the provided image config matches a digest
// stored in the manifest of the image.
func ImageConfigMatchesImage(image *imageapi.Image, imageConfig []byte) (bool, error) {
	if image.DockerImageManifestMediaType != schema2.MediaTypeManifest {
		return false, nil
	}

	var m schema2.DeserializedManifest
	if err := json.Unmarshal([]byte(image.DockerImageManifest), &m); err != nil {
		return false, err
	}

	v := m.Config.Digest.Verifier()
	if _, err := v.Write(imageConfig); err != nil {
		return false, err
	}

	return v.Verified(), nil
}

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

// LatestObservedTagGeneration returns the generation value for the given tag that has been observed by the controller
// monitoring the image stream. If the tag has not been observed, the generation is zero.
func LatestObservedTagGeneration(stream *imagev1.ImageStream, tag string) int64 {
	tagEvents, ok := StatusHasTag(stream, tag)
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
		if condition.Type != imagev1.ImportSuccess {
			continue
		}
		if condition.Generation > lastGen {
			lastGen = condition.Generation
		}
		break
	}
	return lastGen
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

// SetDockerClientDefaults set the default values used by the Docker client.
func SetDockerClientDefaults(r *imagev1.DockerImageReference) {
	if len(r.Registry) == 0 {
		r.Registry = imageapi.DockerDefaultRegistry
	}
	if len(r.Namespace) == 0 && imagereference.IsRegistryDockerHub(r.Registry) {
		r.Namespace = "library"
	}
	if len(r.Tag) == 0 {
		r.Tag = "latest"
	}
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

// DigestOrImageMatch matches the digest in the image name.
func DigestOrImageMatch(image, imageID string) bool {
	if d, err := digest.ParseDigest(image); err == nil {
		return strings.HasPrefix(d.Hex(), imageID) || strings.HasPrefix(image, imageID)
	}
	return strings.HasPrefix(image, imageID)
}

// ResolveImageID returns latest TagEvent for specified imageID and an error if
// there's more than one image matching the ID or when one does not exist.
func ResolveImageID(stream *imagev1.ImageStream, imageID string) (*imagev1.TagEvent, error) {
	var event *imagev1.TagEvent
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
		return &imagev1.TagEvent{
			Created:              metav1.Now(),
			DockerImageReference: event.DockerImageReference,
			Image:                event.Image,
		}, nil
	case 0:
		return nil, kerrors.NewNotFound(imagev1.Resource("imagestreamimage"), imageID)
	default:
		return nil, kerrors.NewConflict(imagev1.Resource("imagestreamimage"), imageID, fmt.Errorf("multiple images match the prefix %q: %s", imageID, strings.Join(set.List(), ", ")))
	}
}

// ResolveLatestTaggedImage returns the appropriate pull spec for a given tag in
// the image stream, handling the tag's reference policy if necessary to return
// a resolved image. Callers that transform an ImageStreamTag into a pull spec
// should use this method instead of LatestTaggedImage.
func ResolveLatestTaggedImage(stream *imagev1.ImageStream, tag string) (string, bool) {
	if len(tag) == 0 {
		tag = imageapi.DefaultImageTag
	}
	return ResolveTagReference(stream, tag, LatestTaggedImage(stream, tag))
}

// ResolveTagReference applies the tag reference rules for a stream, tag, and tag event for
// that tag. It returns true if the tag is
func ResolveTagReference(stream *imagev1.ImageStream, tag string, latest *imagev1.TagEvent) (string, bool) {
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

		// we must use imageapi's helper since we're calling Exact later on, which produces string
		ref, err := imageapi.ParseDockerImageReference(local)
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
	tag, event := LatestImageTagEvent(stream, imageID)
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
func LatestImageTagEvent(stream *imagev1.ImageStream, imageID string) (string, *imagev1.TagEvent) {
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
			if imageapi.DigestOrImageMatch(event.Image, imageID) &&
				(latestTagEvent == nil || latestTagEvent != nil && event.Created.After(latestTagEvent.Created.Time)) {
				latestTagEvent = &events.Items[i]
				latestTag = tag
			}
		}
	}
	return latestTag, latestTagEvent
}

// DockerImageReferenceForStream returns a DockerImageReference that represents
// the ImageStream or false, if no valid reference exists.
func DockerImageReferenceForStream(stream *imagev1.ImageStream) (imagev1.DockerImageReference, error) {
	spec := stream.Status.DockerImageRepository
	if len(spec) == 0 {
		spec = stream.Spec.DockerImageRepository
	}
	if len(spec) == 0 {
		return imagev1.DockerImageReference{}, fmt.Errorf("no possible pull spec for %s/%s", stream.Namespace, stream.Name)
	}
	return ParseDockerImageReference(spec)
}
