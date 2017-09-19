package util

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/golang/glog"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// ImageWithMetadata mutates the given image. It parses raw DockerImageManifest data stored in the image and
// fills its DockerImageMetadata and other fields.
func ImageWithMetadata(image *imageapi.Image) error {
	if len(image.DockerImageManifest) == 0 {
		return nil
	}

	if len(image.DockerImageLayers) > 0 && image.DockerImageMetadata.Size > 0 && len(image.DockerImageManifestMediaType) > 0 {
		glog.V(5).Infof("Image metadata already filled for %s", image.Name)

		ReorderImageLayers(image)

		// don't update image already filled
		return nil
	}

	manifestData := image.DockerImageManifest

	manifest := imageapi.DockerImageManifest{}
	if err := json.Unmarshal([]byte(manifestData), &manifest); err != nil {
		return err
	}

	if image.Annotations == nil {
		image.Annotations = map[string]string{}
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

		image.DockerImageLayers = make([]imageapi.ImageLayer, len(manifest.FSLayers))
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
		} else {
			glog.V(4).Infof("Imported image has mismatched layer count and history count, not updating image metadata: %s", image.Name)
		}
		// reverse order of the layers for v1 (lowest = 0, highest = i)
		for i, j := 0, len(image.DockerImageLayers)-1; i < j; i, j = i+1, j-1 {
			image.DockerImageLayers[i], image.DockerImageLayers[j] = image.DockerImageLayers[j], image.DockerImageLayers[i]
		}
		image.Annotations[imageapi.DockerImageLayersOrderAnnotation] = imageapi.DockerImageLayersOrderAscending

		image.DockerImageMetadata.ID = v1Metadata.ID
		image.DockerImageMetadata.Parent = v1Metadata.Parent
		image.DockerImageMetadata.Comment = v1Metadata.Comment
		image.DockerImageMetadata.Created = v1Metadata.Created
		image.DockerImageMetadata.Container = v1Metadata.Container
		image.DockerImageMetadata.ContainerConfig = v1Metadata.ContainerConfig
		image.DockerImageMetadata.DockerVersion = v1Metadata.DockerVersion
		image.DockerImageMetadata.Author = v1Metadata.Author
		image.DockerImageMetadata.Config = v1Metadata.Config
		image.DockerImageMetadata.Architecture = v1Metadata.Architecture
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
			image.DockerImageMetadata.Size = size
		} else {
			image.DockerImageMetadata.Size = v1Metadata.Size
		}
	case 2:
		image.DockerImageManifestMediaType = schema2.MediaTypeManifest

		if len(image.DockerImageConfig) == 0 {
			return fmt.Errorf("dockerImageConfig must not be empty for manifest schema 2")
		}
		config := imageapi.DockerImageConfig{}
		if err := json.Unmarshal([]byte(image.DockerImageConfig), &config); err != nil {
			return fmt.Errorf("failed to parse dockerImageConfig: %v", err)
		}

		// The layer list is ordered starting from the base image (opposite order of schema1).
		// So, we do not need to change the order of layers.
		image.DockerImageLayers = make([]imageapi.ImageLayer, len(manifest.Layers))
		for i, layer := range manifest.Layers {
			image.DockerImageLayers[i].Name = layer.Digest
			image.DockerImageLayers[i].LayerSize = layer.Size
			image.DockerImageLayers[i].MediaType = layer.MediaType
		}
		image.Annotations[imageapi.DockerImageLayersOrderAnnotation] = imageapi.DockerImageLayersOrderAscending

		image.DockerImageMetadata.ID = manifest.Config.Digest
		image.DockerImageMetadata.Parent = config.Parent
		image.DockerImageMetadata.Comment = config.Comment
		image.DockerImageMetadata.Created = config.Created
		image.DockerImageMetadata.Container = config.Container
		image.DockerImageMetadata.ContainerConfig = config.ContainerConfig
		image.DockerImageMetadata.DockerVersion = config.DockerVersion
		image.DockerImageMetadata.Author = config.Author
		image.DockerImageMetadata.Config = config.Config
		image.DockerImageMetadata.Architecture = config.Architecture
		image.DockerImageMetadata.Size = int64(len(image.DockerImageConfig))

		layerSet := sets.NewString(image.DockerImageMetadata.ID)
		if len(image.DockerImageLayers) > 0 {
			for _, layer := range image.DockerImageLayers {
				if layerSet.Has(layer.Name) {
					continue
				}
				layerSet.Insert(layer.Name)
				image.DockerImageMetadata.Size += layer.LayerSize
			}
		}
	default:
		return fmt.Errorf("unrecognized Docker image manifest schema %d for %q (%s)", manifest.SchemaVersion, image.Name, image.DockerImageReference)
	}

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
		case schema1.MediaTypeManifest:
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
	dgst, err := digest.ParseDigest(image.Name)
	if err != nil {
		return false, err
	}
	v, err := digest.NewDigestVerifier(dgst)
	if err != nil {
		return false, err
	}
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

	v, err := digest.NewDigestVerifier(m.Config.Digest)
	if err != nil {
		return false, err
	}

	if _, err := v.Write(imageConfig); err != nil {
		return false, err
	}

	return v.Verified(), nil
}
