// +build !kubernetes

package set

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	imageclient "github.com/openshift/client-go/image/clientset/versioned"
	imagetypedclient "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
)

// this relies on internal APIs that we don't access to in kubectl
var ParseDockerImageReferenceToStringFunc func(spec string) (string, error)

type imageResolverFunc func(in string) (string, error)

func resolveImageFactory(f cmdutil.Factory, cmd *cobra.Command) imageResolverFunc {
	source, err := cmd.Flags().GetString("source")
	if err != nil {
		return f.ResolveImage
	}

	return func(image string) (string, error) {
		if isDockerImageSource(source) {
			return f.ResolveImage(image)
		}
		config, err := f.ClientConfig()
		if err != nil {
			return "", err
		}
		imageClient, err := imageclient.NewForConfig(config)
		if err != nil {
			return "", err
		}
		namespace, _, err := f.DefaultNamespace()
		if err != nil {
			return "", err
		}

		return resolveImagePullSpec(imageClient.ImageV1(), source, image, namespace)
	}
}

// resolveImagePullSpec resolves the provided source which can be "docker", "istag" or
// "isimage" and returns the full Docker pull spec.
func resolveImagePullSpec(imageClient imagetypedclient.ImageV1Interface, source, name, defaultNamespace string) (string, error) {
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

	return ParseDockerImageReferenceToStringFunc(dockerImageReference)
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
