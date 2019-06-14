package image

import (
	"fmt"
	"strings"
)

const (
	// DockerDefaultNamespace is the value for namespace when a single segment name is provided.
	DockerDefaultNamespace = "library"
	// DockerDefaultRegistry is the value for the registry when none was provided.
	DockerDefaultRegistry = "docker.io"
	// DockerDefaultV1Registry is the host name of the default v1 registry
	DockerDefaultV1Registry = "index." + DockerDefaultRegistry
	// DockerDefaultV2Registry is the host name of the default v2 registry
	DockerDefaultV2Registry = "registry-1." + DockerDefaultRegistry
)

// SplitImageSignatureName splits given signature name into image name and signature name.
func SplitImageSignatureName(imageSignatureName string) (imageName, signatureName string, err error) {
	segments := strings.Split(imageSignatureName, "@")
	switch len(segments) {
	case 2:
		signatureName = segments[1]
		imageName = segments[0]
		if len(imageName) == 0 || len(signatureName) == 0 {
			err = fmt.Errorf("image signature name %q must have an image name and signature name", imageSignatureName)
		}
	default:
		err = fmt.Errorf("expected exactly one @ in the image signature name %q", imageSignatureName)
	}
	return
}
