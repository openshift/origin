package extensions

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/sirupsen/logrus"
	k8simage "k8s.io/kubernetes/test/utils/image"
)

const MirroredImagesEnvVar = "EXTENSION_MIRRORED_IMAGES"

// ComputeMirroredImages builds a mapping from each extension image's original
// pull spec to its mirrored pull spec at fromRepository. Extensions report
// their images through the info command; each Image carries the upstream
// registry/name/version and an optional Mapped hint for the mirrored tag.
func ComputeMirroredImages(extensionsInfo []*Extension, fromRepository string) (map[string]string, error) {
	if fromRepository == "" {
		return nil, nil
	}

	ref, err := reference.Parse(fromRepository)
	if err != nil {
		return nil, fmt.Errorf("invalid from-repository %q: %v", fromRepository, err)
	}

	mapping := make(map[string]string)

	for _, ext := range extensionsInfo {
		if ext == nil || ext.Extension == nil {
			continue
		}
		for _, img := range ext.Images {
			original, mapped := mirroredPullSpecs(img, ref)
			if original == mapped {
				continue
			}
			mapping[original] = mapped
		}
	}

	if len(mapping) == 0 {
		return nil, nil
	}

	logrus.Infof("Computed mirrored image mappings for %d images", len(mapping))
	return mapping, nil
}

// mirroredPullSpecs returns the original and mirrored pull specs for an image.
// The logic mirrors imageSetsForImage in the images command.
func mirroredPullSpecs(img extension.Image, ref reference.DockerImageReference) (original, mirrored string) {
	original = fmt.Sprintf("%s/%s:%s", img.Registry, img.Name, img.Version)

	if img.Mapped == nil {
		id := k8simage.ImageID(img.Index)
		config := k8simage.Config{}
		config.SetName(img.Name)
		config.SetVersion(img.Version)
		config.SetRegistry(img.Registry)
		originalSet := map[k8simage.ImageID]k8simage.Config{id: config}
		mappedSet := k8simage.GetMappedImageConfigs(originalSet, ref.Exact())
		mappedConfig := mappedSet[id]
		mirrored = mappedConfig.GetE2EImage()
	} else {
		mirrored = fmt.Sprintf("%s/%s:%s", ref.Registry, ref.RepositoryName(), img.Mapped.Version)
	}

	return original, mirrored
}

// WriteMirroredImagesFile writes the mapping to a temporary JSON file and
// returns the path. The caller is responsible for removing the file.
func WriteMirroredImagesFile(mapping map[string]string) (string, error) {
	if len(mapping) == 0 {
		return "", nil
	}

	data, err := json.Marshal(mapping)
	if err != nil {
		return "", fmt.Errorf("failed to marshal mirrored images: %v", err)
	}

	f, err := os.CreateTemp("", "mirrored-images-*.json")
	if err != nil {
		return "", fmt.Errorf("failed to create mirrored images file: %v", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("failed to write mirrored images file: %v", err)
	}

	return f.Name(), nil
}

// ReadMirroredImagesFile reads a mirrored images mapping from a JSON file.
func ReadMirroredImagesFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read mirrored images file: %v", err)
	}

	mapping := make(map[string]string)
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, fmt.Errorf("failed to parse mirrored images file: %v", err)
	}

	return mapping, nil
}

// MirroredLocationFor returns the mirrored pull spec for an image if one
// exists in the mapping, or the original pull spec unchanged.
func MirroredLocationFor(mapping map[string]string, original string) string {
	if mapped, ok := mapping[original]; ok {
		return mapped
	}
	return original
}

// LoadMirroredImagesFromEnv reads the mirrored images mapping from the file
// referenced by the EXTENSION_MIRRORED_IMAGES environment variable. Returns
// nil without error when the variable is unset.
func LoadMirroredImagesFromEnv() (map[string]string, error) {
	path := os.Getenv(MirroredImagesEnvVar)
	if path == "" {
		return nil, nil
	}
	return ReadMirroredImagesFile(path)
}
