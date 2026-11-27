package payload

import (
	"context"
	"fmt"
	"os"

	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/shared/registryauth"
)

// ExtractImageFromReleasePayload extracts the image pull spec for a specific tag from a release payload.
// It returns the image pull spec for the specified tag or an error if the tag is not found.
func ExtractImageFromReleasePayload(ctx context.Context, releaseImage, imageTag string, oc *util.CLI) (string, error) {
	// Create a temporary directory for extraction
	tmpDir, err := os.MkdirTemp("", "release-extract")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	registryAuthFilePath, err := registryauth.DetermineAuthFilePath(ctx, clusterClient(oc), tmpDir)
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

// clusterClient returns a client for the cluster oc targets, or nil when oc is
// nil. It is a registryauth.ClientFunc so the cluster is only contacted when no
// cheaper credential source is available.
func clusterClient(oc *util.CLI) registryauth.ClientFunc {
	return func() (corev1client.CoreV1Interface, error) {
		if oc == nil {
			return nil, nil
		}
		return oc.AdminKubeClient().CoreV1(), nil
	}
}
