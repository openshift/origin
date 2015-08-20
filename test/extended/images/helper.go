package images

import (
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
)

// GetImageLabels retrieves Docker labels from image from image repository name and
// image reference
func GetImageLabels(c client.ImageStreamImageInterface, imageRepoName, imageRef string) (map[string]string, error) {
	_, imageID, err := imagestreamimage.ParseNameAndID(imageRef)
	image, err := c.Get(imageRepoName, imageID)

	if err != nil {
		return map[string]string{}, err
	}
	return image.Image.DockerImageMetadata.Config.Labels, nil
}
