package payload

import (
	"fmt"
	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/test/extended/util"
	"os"
)

// ExtractImageFromReleasePayload extracts the image pull spec for a specific tag from a release payload.
// It returns the image pull spec for the specified tag or an error if the tag is not found.
func ExtractImageFromReleasePayload(releaseImage, imageTag string, oc *util.CLI) (string, error) {
	// Create a temporary directory for extraction
	tmpDir, err := os.MkdirTemp("", "release-extract")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	registryAuthFilePath, err := extensions.DetermineRegistryAuthFilePath(tmpDir, oc)
	if err != nil {
		return "", fmt.Errorf("failed to determine registry auth file path: %w", err)
	}

	// Extract the ImageStream from the release payload
	imageStream, _, err := extensions.ExtractReleaseImageStream(tmpDir, releaseImage, registryAuthFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to extract image references from release payload: %w", err)
	}

	// Find the specified tag in the ImageStream
	for _, tag := range imageStream.Spec.Tags {
		if tag.Name == imageTag {
			return tag.From.Name, nil
		}
	}

	return "", fmt.Errorf("image tag %q not found in release payload %q", imageTag, releaseImage)
}
