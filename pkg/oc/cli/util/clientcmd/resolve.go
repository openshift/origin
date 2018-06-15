package clientcmd

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
)

// resolveImagePullSpec resolves the provided source which can be "docker", "istag" or
// "isimage" and returns the full Docker pull spec.
func resolveImagePullSpec(imageClient imageclient.ImageInterface, source, name, defaultNamespace string) (string, error) {
	// for Docker source, just passtrough the image name
	if isDockerImageSource(source) {
		return name, nil
	}
	// parse the namespace from the provided image
	namespace, image := splitNamespaceAndImage(name)
	if len(namespace) == 0 {
		namespace = defaultNamespace
	}

	dockerImageReference := ""

	if isImageStreamTag(source) {
		if resolved, err := imageClient.ImageStreamTags(namespace).Get(image, metav1.GetOptions{}); err != nil {
			return "", fmt.Errorf("failed to get image stream tag %q: %v", image, err)
		} else {
			dockerImageReference = resolved.Image.DockerImageReference
		}
	}

	if isImageStreamImage(source) {
		if resolved, err := imageClient.ImageStreamImages(namespace).Get(image, metav1.GetOptions{}); err != nil {
			return "", fmt.Errorf("failed to get image stream image %q: %v", image, err)
		} else {
			dockerImageReference = resolved.Image.DockerImageReference
		}
	}

	if len(dockerImageReference) == 0 {
		return "", fmt.Errorf("unable to resolve %s %q", source, name)
	}

	reference, err := imageapi.ParseDockerImageReference(dockerImageReference)
	if err != nil {
		return "", err
	}
	return reference.String(), nil
}

func isDockerImageSource(source string) bool {
	return source == "docker"
}

func isImageStreamTag(source string) bool {
	return source == "istag" || source == "imagestreamtag"
}

func isImageStreamImage(source string) bool {
	return source == "isimage" || source == "imagestreamimage"
}

func splitNamespaceAndImage(name string) (string, string) {
	namespace := ""
	imageName := ""
	if parts := strings.Split(name, "/"); len(parts) == 2 {
		namespace, imageName = parts[0], parts[1]
	} else if len(parts) == 1 {
		imageName = parts[0]
	}
	return namespace, imageName
}
