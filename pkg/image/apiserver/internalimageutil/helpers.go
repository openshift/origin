package internalimageutil

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	godigest "github.com/opencontainers/go-digest"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"

	"github.com/openshift/api/image"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	"github.com/openshift/library-go/pkg/image/reference"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	dockerapi10 "github.com/openshift/origin/pkg/image/apis/image/docker10"
)

// InternalImageWithMetadata mutates the given image. It parses raw DockerImageManifest data stored in the image and
// fills its DockerImageMetadata and other fields.
func InternalImageWithMetadata(image *imageapi.Image) error {
	if len(image.DockerImageManifest) == 0 {
		return nil
	}

	reorderImageLayers(image)

	if len(image.DockerImageLayers) > 0 && image.DockerImageMetadata.Size > 0 && len(image.DockerImageManifestMediaType) > 0 {
		klog.V(5).Infof("Image metadata already filled for %s", image.Name)
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
	image.Annotations[imagev1.DockerImageLayersOrderAnnotation] = imagev1.DockerImageLayersOrderAscending

	return nil
}

// reorderImageLayers mutates the given image. It reorders the layers in ascending order.
// Ascending order matches the order of layers in schema 2. Schema 1 has reversed (descending) order of layers.
func reorderImageLayers(image *imageapi.Image) {
	if len(image.DockerImageLayers) == 0 {
		return
	}

	layersOrder, ok := image.Annotations[imagev1.DockerImageLayersOrderAnnotation]
	if !ok {
		switch image.DockerImageManifestMediaType {
		case schema1.MediaTypeManifest, schema1.MediaTypeSignedManifest:
			layersOrder = imagev1.DockerImageLayersOrderAscending
		case schema2.MediaTypeManifest:
			layersOrder = imagev1.DockerImageLayersOrderDescending
		default:
			return
		}
	}

	if layersOrder == imagev1.DockerImageLayersOrderDescending {
		// reverse order of the layers (lowest = 0, highest = i)
		for i, j := 0, len(image.DockerImageLayers)-1; i < j; i, j = i+1, j-1 {
			image.DockerImageLayers[i], image.DockerImageLayers[j] = image.DockerImageLayers[j], image.DockerImageLayers[i]
		}
	}

	if image.Annotations == nil {
		image.Annotations = map[string]string{}
	}

	image.Annotations[imagev1.DockerImageLayersOrderAnnotation] = imagev1.DockerImageLayersOrderAscending
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

// LatestTaggedImage returns the most recent TagEvent for the specified image
// repository and tag. Will resolve lookups for the empty tag. Returns nil
// if tag isn't present in stream.status.tags.
func LatestTaggedImage(stream *imageapi.ImageStream, tag string) *imageapi.TagEvent {
	if len(tag) == 0 {
		tag = imagev1.DefaultImageTag
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
func ResolveLatestTaggedImage(stream *imageapi.ImageStream, tag string) (string, bool) {
	if len(tag) == 0 {
		tag = imagev1.DefaultImageTag
	}
	return resolveTagReference(stream, tag, LatestTaggedImage(stream, tag))
}

// ResolveTagReference applies the tag reference rules for a stream, tag, and tag event for
// that tag. It returns true if the tag is
func resolveTagReference(stream *imageapi.ImageStream, tag string, latest *imageapi.TagEvent) (string, bool) {
	if latest == nil {
		return "", false
	}
	return ResolveReferenceForTagEvent(stream, tag, latest), true
}

// ResolveReferenceForTagEvent applies the tag reference rules for a stream, tag, and tag event for
// that tag.
func ResolveReferenceForTagEvent(stream *imageapi.ImageStream, tag string, latest *imageapi.TagEvent) string {
	// retrieve spec policy - if not found, we use the latest spec
	ref, ok := stream.Spec.Tags[tag]
	if !ok {
		return latest.DockerImageReference
	}

	switch ref.ReferencePolicy.Type {
	// the local reference policy attempts to use image pull through on the integrated
	// registry if possible
	case imageapi.LocalTagReferencePolicy:
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

// AddTagEventToImageStream attempts to update the given image stream with a tag event. It will
// collapse duplicate entries - returning true if a change was made or false if no change
// occurred. Any successful tag resets the status field.
func AddTagEventToImageStream(stream *imageapi.ImageStream, tag string, next imageapi.TagEvent) bool {
	if stream.Status.Tags == nil {
		stream.Status.Tags = make(map[string]imageapi.TagEventList)
	}

	tags, ok := stream.Status.Tags[tag]
	if !ok || len(tags.Items) == 0 {
		stream.Status.Tags[tag] = imageapi.TagEventList{Items: []imageapi.TagEvent{next}}
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
		tags.Items = append([]imageapi.TagEvent{next}, tags.Items...)
		stream.Status.Tags[tag] = tags
		return true
	}
	previous.Generation = next.Generation
	tags.Conditions = nil
	stream.Status.Tags[tag] = tags
	return true
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
func UpdateTrackingTags(stream *imageapi.ImageStream, updatedTag string, updatedImage imageapi.TagEvent) int {
	updated := 0
	klog.V(5).Infof("UpdateTrackingTags: stream=%s/%s, updatedTag=%s, updatedImage.dockerImageReference=%s, updatedImage.image=%s", stream.Namespace, stream.Name, updatedTag, updatedImage.DockerImageReference, updatedImage.Image)
	for specTag, tagRef := range stream.Spec.Tags {
		klog.V(5).Infof("Examining spec tag %q, tagRef=%#v", specTag, tagRef)

		// no from
		if tagRef.From == nil {
			klog.V(5).Infof("tagRef.From is nil, skipping")
			continue
		}

		// wrong kind
		if tagRef.From.Kind != "ImageStreamTag" {
			klog.V(5).Infof("tagRef.Kind %q isn't ImageStreamTag, skipping", tagRef.From.Kind)
			continue
		}

		tagRefNamespace := tagRef.From.Namespace
		if len(tagRefNamespace) == 0 {
			tagRefNamespace = stream.Namespace
		}

		// different namespace
		if tagRefNamespace != stream.Namespace {
			klog.V(5).Infof("tagRefNamespace %q doesn't match stream namespace %q - skipping", tagRefNamespace, stream.Namespace)
			continue
		}

		tag := ""
		tagRefName := ""
		if strings.Contains(tagRef.From.Name, ":") {
			// <stream>:<tag>
			ok := true
			tagRefName, tag, ok = imageutil.SplitImageStreamTag(tagRef.From.Name)
			if !ok {
				klog.V(5).Infof("tagRefName %q contains invalid reference - skipping", tagRef.From.Name)
				continue
			}
		} else {
			// <tag> (this stream)
			// TODO: this is probably wrong - we should require ":<tag>", but we can't break old clients
			tagRefName = stream.Name
			tag = tagRef.From.Name
		}

		klog.V(5).Infof("tagRefName=%q, tag=%q", tagRefName, tag)

		// different stream
		if tagRefName != stream.Name {
			klog.V(5).Infof("tagRefName %q doesn't match stream name %q - skipping", tagRefName, stream.Name)
			continue
		}

		// different tag
		if tag != updatedTag {
			klog.V(5).Infof("tag %q doesn't match updated tag %q - skipping", tag, updatedTag)
			continue
		}

		if AddTagEventToImageStream(stream, specTag, updatedImage) {
			klog.V(5).Infof("stream updated")
			updated++
		}
	}
	return updated
}

// ResolveImageID returns latest TagEvent for specified imageID and an error if
// there's more than one image matching the ID or when one does not exist.
func ResolveImageID(stream *imageapi.ImageStream, imageID string) (*imageapi.TagEvent, error) {
	var event *imageapi.TagEvent
	set := sets.NewString()
	for _, history := range stream.Status.Tags {
		for i := range history.Items {
			tagging := &history.Items[i]
			if imageutil.DigestOrImageMatch(tagging.Image, imageID) {
				event = tagging
				set.Insert(tagging.Image)
			}
		}
	}
	switch len(set) {
	case 1:
		return &imageapi.TagEvent{
			Created:              metav1.Now(),
			DockerImageReference: event.DockerImageReference,
			Image:                event.Image,
		}, nil
	case 0:
		return nil, errors.NewNotFound(image.Resource("imagestreamimage"), imageID)
	default:
		return nil, errors.NewConflict(image.Resource("imagestreamimage"), imageID, fmt.Errorf("multiple images match the prefix %q: %s", imageID, strings.Join(set.List(), ", ")))
	}
}

// HasTagCondition returns true if the specified image stream tag has a condition with the same type, status, and
// reason (does not check generation, date, or message).
func HasTagCondition(stream *imageapi.ImageStream, tag string, condition imageapi.TagEventCondition) bool {
	for _, existing := range stream.Status.Tags[tag].Conditions {
		if condition.Type == existing.Type && condition.Status == existing.Status && condition.Reason == existing.Reason {
			return true
		}
	}
	return false
}
