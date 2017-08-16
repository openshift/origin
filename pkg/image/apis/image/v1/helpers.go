package v1

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// ResolveImageID returns latest TagEvent for specified imageID and an error if
// there's more than one image matching the ID or when one does not exist.
func ResolveImageID(stream *ImageStream, imageID string) (*TagEvent, error) {
	var event *TagEvent
	set := sets.NewString()
	for _, history := range stream.Status.Tags {
		for i := range history.Items {
			tagging := &history.Items[i]
			if imageapi.DigestOrImageMatch(tagging.Image, imageID) {
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
		return nil, errors.NewNotFound(Resource("imagestreamimage"), imageID)
	default:
		return nil, errors.NewConflict(Resource("imagestreamimage"), imageID, fmt.Errorf("multiple images match the prefix %q: %s", imageID, strings.Join(set.List(), ", ")))
	}
}

// LatestImageTagEvent returns the most recent TagEvent and the tag for the specified
// image.
func LatestImageTagEvent(stream *ImageStream, imageID string) (string, *TagEvent) {
	var (
		latestTagEvent *TagEvent
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

// LatestTaggedImage returns the most recent TagEvent for the specified image
// repository and tag. Will resolve lookups for the empty tag. Returns nil
// if tag isn't present in stream.status.tags.
func LatestTaggedImage(stream *ImageStream, tag string) *TagEvent {
	if len(tag) == 0 {
		tag = imageapi.DefaultImageTag
	}
	// find the most recent tag event with an image reference
	if stream.Status.Tags != nil {
		for _, t := range stream.Status.Tags {
			if t.Tag == tag {
				if len(t.Items) == 0 {
					return nil
				}
				return &t.Items[0]
			}
		}
	}

	return nil
}

// ImageWithMetadata mutates the given image. It parses raw DockerImageManifest data stored in the image and
// fills its DockerImageMetadata and other fields.
func ImageWithMetadata(image *Image) error {
	if len(image.DockerImageManifest) == 0 {
		return nil
	}

	if len(image.DockerImageLayers) > 0 && len(image.DockerImageManifestMediaType) > 0 {
		if meta, ok := image.DockerImageMetadata.Object.(*imageapi.DockerImage); ok && meta.Size > 0 {
			// don't update image already filled
			return nil
		}
	}

	manifestData := image.DockerImageManifest

	manifest := imageapi.DockerImageManifest{}
	if err := json.Unmarshal([]byte(manifestData), &manifest); err != nil {
		return err
	}

	switch manifest.SchemaVersion {
	case 0:
		// legacy config object
	case 1:
		image.DockerImageManifestMediaType = schema1.MediaTypeManifest

		if len(manifest.History) == 0 {
			// should never have an empty history, but just in case...
			return nil
		}

		v1Metadata := imageapi.DockerV1CompatibilityImage{}
		if err := json.Unmarshal([]byte(manifest.History[0].DockerV1Compatibility), &v1Metadata); err != nil {
			return err
		}

		image.DockerImageLayers = make([]ImageLayer, len(manifest.FSLayers))
		for i, layer := range manifest.FSLayers {
			image.DockerImageLayers[i].MediaType = schema1.MediaTypeManifestLayer
			image.DockerImageLayers[i].Name = layer.DockerBlobSum
		}
		if len(manifest.History) == len(image.DockerImageLayers) {
			// This code does not work for images converted from v2 to v1, since V1Compatibility does not
			// contain size information in this case.
			image.DockerImageLayers[0].LayerSize = v1Metadata.Size
			var size = imageapi.DockerV1CompatibilityImageSize{}
			for i, obj := range manifest.History[1:] {
				size.Size = 0
				if err := json.Unmarshal([]byte(obj.DockerV1Compatibility), &size); err != nil {
					continue
				}
				image.DockerImageLayers[i+1].LayerSize = size.Size
			}
		}
		// reverse order of the layers for v1 (lowest = 0, highest = i)
		for i, j := 0, len(image.DockerImageLayers)-1; i < j; i, j = i+1, j-1 {
			image.DockerImageLayers[i], image.DockerImageLayers[j] = image.DockerImageLayers[j], image.DockerImageLayers[i]
		}

		dockerImage := &imageapi.DockerImage{}

		dockerImage.ID = v1Metadata.ID
		dockerImage.Parent = v1Metadata.Parent
		dockerImage.Comment = v1Metadata.Comment
		dockerImage.Created = v1Metadata.Created
		dockerImage.Container = v1Metadata.Container
		dockerImage.ContainerConfig = v1Metadata.ContainerConfig
		dockerImage.DockerVersion = v1Metadata.DockerVersion
		dockerImage.Author = v1Metadata.Author
		dockerImage.Config = v1Metadata.Config
		dockerImage.Architecture = v1Metadata.Architecture
		if len(image.DockerImageLayers) > 0 {
			size := int64(0)
			layerSet := sets.NewString()
			for _, layer := range image.DockerImageLayers {
				if layerSet.Has(layer.Name) {
					continue
				}
				layerSet.Insert(layer.Name)
				size += layer.LayerSize
			}
			dockerImage.Size = size
		} else {
			dockerImage.Size = v1Metadata.Size
		}

		image.DockerImageMetadata.Object = dockerImage
	case 2:
		image.DockerImageManifestMediaType = schema2.MediaTypeManifest

		if len(image.DockerImageConfig) == 0 {
			return fmt.Errorf("dockerImageConfig must not be empty for manifest schema 2")
		}
		config := imageapi.DockerImageConfig{}
		if err := json.Unmarshal([]byte(image.DockerImageConfig), &config); err != nil {
			return fmt.Errorf("failed to parse dockerImageConfig: %v", err)
		}

		image.DockerImageLayers = make([]ImageLayer, len(manifest.Layers))
		for i, layer := range manifest.Layers {
			image.DockerImageLayers[i].Name = layer.Digest
			image.DockerImageLayers[i].LayerSize = layer.Size
			image.DockerImageLayers[i].MediaType = layer.MediaType
		}
		// reverse order of the layers for v1 (lowest = 0, highest = i)
		for i, j := 0, len(image.DockerImageLayers)-1; i < j; i, j = i+1, j-1 {
			image.DockerImageLayers[i], image.DockerImageLayers[j] = image.DockerImageLayers[j], image.DockerImageLayers[i]
		}
		dockerImage := &imageapi.DockerImage{}

		dockerImage.ID = manifest.Config.Digest
		dockerImage.Parent = config.Parent
		dockerImage.Comment = config.Comment
		dockerImage.Created = config.Created
		dockerImage.Container = config.Container
		dockerImage.ContainerConfig = config.ContainerConfig
		dockerImage.DockerVersion = config.DockerVersion
		dockerImage.Author = config.Author
		dockerImage.Config = config.Config
		dockerImage.Architecture = config.Architecture
		dockerImage.Size = int64(len(image.DockerImageConfig))

		layerSet := sets.NewString(dockerImage.ID)
		if len(image.DockerImageLayers) > 0 {
			for _, layer := range image.DockerImageLayers {
				if layerSet.Has(layer.Name) {
					continue
				}
				layerSet.Insert(layer.Name)
				dockerImage.Size += layer.LayerSize
			}
		}
		image.DockerImageMetadata.Object = dockerImage
	default:
		return fmt.Errorf("unrecognized Docker image manifest schema %d for %q (%s)", manifest.SchemaVersion, image.Name, image.DockerImageReference)
	}

	return nil
}
