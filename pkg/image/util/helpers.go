package util

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/golang/glog"
	godigest "github.com/opencontainers/go-digest"

	imagev1 "github.com/openshift/api/image/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/docker10"
)

func getImageLayers(manifest docker10.DockerImageManifest) ([]imageapi.ImageLayer, error) {
	var imageLayers []imageapi.ImageLayer
	switch manifest.SchemaVersion {
	case 1:
		if len(manifest.History) != len(manifest.FSLayers) {
			return nil, fmt.Errorf("mismatched history and fslayer cardinality (%d != %d)", len(manifest.History), len(manifest.FSLayers))
		}

		imageLayers = make([]imageapi.ImageLayer, len(manifest.FSLayers))
		for i, obj := range manifest.History {
			layer := manifest.FSLayers[i]

			var size docker10.DockerV1CompatibilityImageSize
			if err := json.Unmarshal([]byte(obj.DockerV1Compatibility), &size); err != nil {
				size.Size = 0
			}

			// reverse order of the layers: in schema1 manifests the
			// first layer is the youngest (base layers are at the
			// end), but we want to store layers in the Image resource
			// in order from the oldest to the youngest.
			revidx := (len(manifest.History) - 1) - i // n-1, n-2, ..., 1, 0

			imageLayers[revidx].Name = layer.DockerBlobSum
			imageLayers[revidx].LayerSize = size.Size
			imageLayers[revidx].MediaType = schema1.MediaTypeManifestLayer
		}
	case 2:
		// The layer list is ordered starting from the base image (opposite order of schema1).
		// So, we do not need to change the order of layers.
		imageLayers = make([]imageapi.ImageLayer, len(manifest.Layers))
		for i, layer := range manifest.Layers {
			imageLayers[i].Name = layer.Digest
			imageLayers[i].LayerSize = layer.Size
			imageLayers[i].MediaType = layer.MediaType
		}
	default:
		return nil, fmt.Errorf("unrecognized Docker image manifest schema %d", manifest.SchemaVersion)
	}

	return imageLayers, nil
}

// reorderImageLayers mutates the given image. It reorders the layers in ascending order.
// Ascending order matches the order of layers in schema 2. Schema 1 has reversed (descending) order of layers.
func reorderImageLayers(imageLayers []imageapi.ImageLayer, layersOrder, imageManifestMediaType string) bool {
	if imageLayers == nil || len(imageLayers) == 0 {
		return false
	}

	if layersOrder == "" {
		switch imageManifestMediaType {
		case schema1.MediaTypeManifest, schema1.MediaTypeSignedManifest:
			layersOrder = imageapi.DockerImageLayersOrderAscending
		case schema2.MediaTypeManifest:
			layersOrder = imageapi.DockerImageLayersOrderDescending
		default:
			return false
		}
	}

	if layersOrder == imageapi.DockerImageLayersOrderDescending {
		// reverse order of the layers (lowest = 0, highest = i)
		for i, j := 0, len(imageLayers)-1; i < j; i, j = i+1, j-1 {
			imageLayers[i], imageLayers[j] = imageLayers[j], imageLayers[i]
		}
	}

	return true
}

func convertImageLayers(imageLayers []imagev1.ImageLayer) []imageapi.ImageLayer {
	if imageLayers == nil {
		return nil
	}

	result := make([]imageapi.ImageLayer, len(imageLayers))
	for i := range imageLayers {
		result[i].MediaType = imageLayers[i].MediaType
		result[i].Name = imageLayers[i].Name
		result[i].LayerSize = imageLayers[i].LayerSize
	}
	return result
}

func GetImageMetadata(image *imagev1.Image) (imageapi.DockerImage, error) {
	if len(image.DockerImageManifest) == 0 {
		return imageapi.DockerImage{}, nil
	}

	imageLayers := convertImageLayers(image.DockerImageLayers)
	reorderImageLayers(imageLayers, image.Annotations[imageapi.DockerImageLayersOrderAnnotation], image.DockerImageManifestMediaType)

	_, imageMetadata, _, _, err := getImageMetadata(image.Name, image.DockerImageReference,
		image.DockerImageManifest, image.DockerImageConfig, imageLayers)
	return imageMetadata, err

}

// ImageWithMetadata mutates the given image. It parses raw DockerImageManifest data stored in the image and
// fills its DockerImageMetadata and other fields.
func ImageWithMetadata(image *imageapi.Image) error {
	if len(image.DockerImageManifest) == 0 {
		return nil
	}

	if ok := reorderImageLayers(image.DockerImageLayers,
		image.Annotations[imageapi.DockerImageLayersOrderAnnotation], image.DockerImageManifestMediaType); ok {
		if image.Annotations == nil {
			image.Annotations = map[string]string{}
		}
		image.Annotations[imageapi.DockerImageLayersOrderAnnotation] = imageapi.DockerImageLayersOrderAscending
	}

	if len(image.DockerImageLayers) > 0 && image.DockerImageMetadata.Size > 0 && len(image.DockerImageManifestMediaType) > 0 {
		glog.V(5).Infof("Image metadata already filled for %s", image.Name)
		return nil
	}
	imageManifestMediaType, imageMetadata, imageLayers, orderAscending, err := getImageMetadata(image.Name, image.DockerImageReference,
		image.DockerImageManifest, image.DockerImageConfig, image.DockerImageLayers)
	if err != nil {
		return err
	}
	image.DockerImageManifestMediaType = imageManifestMediaType
	image.DockerImageMetadata = imageMetadata
	image.DockerImageLayers = imageLayers
	if orderAscending {
		if image.Annotations == nil {
			image.Annotations = map[string]string{}
		}
		image.Annotations[imageapi.DockerImageLayersOrderAnnotation] = imageapi.DockerImageLayersOrderAscending
	}

	return nil
}

func getImageMetadata(imageName, imageReference, imageManifest, imageConfig string,
	imageLayers []imageapi.ImageLayer) (string, imageapi.DockerImage, []imageapi.ImageLayer, bool, error) {
	manifest := docker10.DockerImageManifest{}
	if err := json.Unmarshal([]byte(imageManifest), &manifest); err != nil {
		return "", imageapi.DockerImage{}, []imageapi.ImageLayer{}, false, err
	}

	var err error
	var orderAscending bool
	if len(imageLayers) == 0 {
		imageLayers, err = getImageLayers(manifest)
		if err != nil {
			return "", imageapi.DockerImage{}, []imageapi.ImageLayer{}, false, fmt.Errorf("the image %s (%s) failed reading layers: %v", imageName, imageReference, err)
		}
		orderAscending = true
	}

	var imageManifestMediaType string
	var imageMetadata imageapi.DockerImage
	switch manifest.SchemaVersion {
	case 1:
		imageManifestMediaType = schema1.MediaTypeManifest

		if len(manifest.History) == 0 {
			// It should never have an empty history, but just in case.
			return "", imageapi.DockerImage{}, []imageapi.ImageLayer{}, false, fmt.Errorf("the image %s (%s) has a schema 1 manifest, but it doesn't have history", imageName, imageReference)
		}

		v1Metadata := docker10.DockerV1CompatibilityImage{}
		if err := json.Unmarshal([]byte(manifest.History[0].DockerV1Compatibility), &v1Metadata); err != nil {
			return "", imageapi.DockerImage{}, []imageapi.ImageLayer{}, false, err
		}

		if err := imageapi.Convert_compatibility_to_api_DockerImage(&v1Metadata, &imageMetadata); err != nil {
			return "", imageapi.DockerImage{}, []imageapi.ImageLayer{}, false, err
		}
	case 2:
		imageManifestMediaType = schema2.MediaTypeManifest

		if len(imageConfig) == 0 {
			return "", imageapi.DockerImage{}, []imageapi.ImageLayer{}, false, fmt.Errorf("dockerImageConfig must not be empty for manifest schema 2")
		}

		config := docker10.DockerImageConfig{}
		if err := json.Unmarshal([]byte(imageConfig), &config); err != nil {
			return "", imageapi.DockerImage{}, []imageapi.ImageLayer{}, false, fmt.Errorf("failed to parse dockerImageConfig: %v", err)
		}

		if err := imageapi.Convert_imageconfig_to_api_DockerImage(&config, &imageMetadata); err != nil {
			return "", imageapi.DockerImage{}, []imageapi.ImageLayer{}, false, err
		}
		imageMetadata.ID = manifest.Config.Digest

	default:
		return "", imageapi.DockerImage{}, []imageapi.ImageLayer{}, false, fmt.Errorf("unrecognized Docker image manifest schema %d for %q (%s)", manifest.SchemaVersion, imageName, imageReference)
	}

	layerSet := sets.NewString()
	if manifest.SchemaVersion == 2 {
		layerSet.Insert(manifest.Config.Digest)
		imageMetadata.Size = int64(len(imageConfig))
	} else {
		imageMetadata.Size = 0
	}
	for _, layer := range imageLayers {
		if layerSet.Has(layer.Name) {
			continue
		}
		layerSet.Insert(layer.Name)
		imageMetadata.Size += layer.LayerSize
	}

	return imageManifestMediaType, imageMetadata, imageLayers, orderAscending, nil
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
