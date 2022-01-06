package image

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// GetE2eImageMappedToRegistry for some CLI tests in 4.6 either no registry is provided for the image
// and/or docker.io is hard coded.
// returns the fully qualified path, the base image name/tag, or error
func GetE2eImageMappedToRegistry(image string, path string) (string, error) {
	registry, defined := os.LookupEnv("TEST_IMAGE_MIRROR_REGISTRY")
	if !defined {
		return "", errors.New("TEST_IMAGE_MIRROR_REGISTRY not defined")
	}
	baseName := ""
	parts := strings.Split(image, "/")
	if len(parts) < 2 {
		baseName = image
	} else {
		baseName = parts[len(parts)-1]
	}
	if len(parts) == 3 && path == "" {
		path = parts[1]
	}

	return fmt.Sprintf("%s/%s/%s", registry, path, baseName), nil
}
