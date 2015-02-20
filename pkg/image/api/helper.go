package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

// DockerDefaultNamespace is the value for namespace when a single segment name is provided.
const DockerDefaultNamespace = "library"

// SplitDockerPullSpec breaks a Docker pull specification into its components, or returns
// an error if those components are not valid. Attempts to match as closely as possible the
// Docker spec up to 1.3. Future API revisions may change the pull syntax.
func SplitDockerPullSpec(spec string) (registry, namespace, name, tag string, err error) {
	registry, namespace, name, tag, err = SplitOpenShiftPullSpec(spec)
	if err != nil {
		return
	}
	return
}

// SplitOpenShiftPullSpec breaks an OpenShift pull specification into its components, or returns
// an error if those components are not valid. Attempts to match as closely as possible the
// Docker spec up to 1.3. Future API revisions may change the pull syntax.
func SplitOpenShiftPullSpec(spec string) (registry, namespace, name, tag string, err error) {
	spec, tag = docker.ParseRepositoryTag(spec)
	arr := strings.Split(spec, "/")
	switch len(arr) {
	case 2:
		return "", arr[0], arr[1], tag, nil
	case 3:
		return arr[0], arr[1], arr[2], tag, nil
	case 1:
		if len(arr[0]) == 0 {
			err = fmt.Errorf("the docker pull spec %q must be two or three segments separated by slashes", spec)
			return
		}
		return "", "", arr[0], tag, nil
	default:
		err = fmt.Errorf("the docker pull spec %q must be two or three segments separated by slashes", spec)
		return
	}
}

// IsPullSpec returns true if the provided string appears to be a valid Docker pull spec.
func IsPullSpec(spec string) bool {
	_, _, _, _, err := SplitDockerPullSpec(spec)
	return err == nil
}

// JoinDockerPullSpec turns a set of components of a Docker pull specification into a single
// string. Attempts to match as closely as possible the Docker spec up to 1.3. Future API
// revisions may change the pull syntax.
func JoinDockerPullSpec(registry, namespace, name, tag string) string {
	if len(tag) != 0 {
		tag = ":" + tag
	}
	if len(namespace) == 0 {
		if len(registry) == 0 {
			return fmt.Sprintf("%s%s", name, tag)
		}
		namespace = DockerDefaultNamespace
	}
	if len(registry) == 0 {
		return fmt.Sprintf("%s/%s%s", namespace, name, tag)
	}
	return fmt.Sprintf("%s/%s/%s%s", registry, namespace, name, tag)
}

// ImageWithMetadata returns a copy of image with the DockerImageMetadata filled in
// from the raw DockerImageManifest data stored in the image.
func ImageWithMetadata(image Image) (*Image, error) {
	if len(image.DockerImageManifest) == 0 {
		return &image, nil
	}

	manifestData := image.DockerImageManifest

	image.DockerImageManifest = ""

	manifest := DockerImageManifest{}
	if err := json.Unmarshal([]byte(manifestData), &manifest); err != nil {
		return nil, err
	}

	if len(manifest.History) == 0 {
		// should never have an empty history, but just in case...
		return &image, nil
	}

	v1Metadata := DockerV1CompatibilityImage{}
	if err := json.Unmarshal([]byte(manifest.History[0].DockerV1Compatibility), &v1Metadata); err != nil {
		return nil, err
	}

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
	image.DockerImageMetadata.Size = v1Metadata.Size

	return &image, nil
}
