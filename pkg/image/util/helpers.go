package util

import (
	"encoding/json"
	"fmt"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/golang/glog"
	godigest "github.com/opencontainers/go-digest"

	"k8s.io/apimachinery/pkg/util/sets"

	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	dockerapi10 "github.com/openshift/origin/pkg/image/apis/image/docker10"
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
